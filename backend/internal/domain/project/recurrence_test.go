package project

import (
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func at(s, zone string) time.Time {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		panic(err)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		panic(err)
	}
	return t
}

func TestRecurrenceNext(t *testing.T) {
	daily := Recurrence{Freq: FreqDaily}
	monWedFri := Recurrence{Freq: FreqWeekly, Weekdays: []time.Weekday{time.Monday, time.Wednesday, time.Friday}}
	on31 := Recurrence{Freq: FreqMonthly, MonthDay: 31}
	on29 := Recurrence{Freq: FreqMonthly, MonthDay: 29}
	on15 := Recurrence{Freq: FreqMonthly, MonthDay: 15}
	for _, c := range []struct {
		name  string
		r     Recurrence
		after string
		want  string
	}{
		{"daily", daily, "2026-10-14", "2026-10-15"},
		{"daily over year end", daily, "2026-12-31", "2027-01-01"},
		{"daily into leap day", daily, "2028-02-28", "2028-02-29"},
		{"weekly next listed day", monWedFri, "2026-10-14", "2026-10-16"}, // Wed → Fri
		{"weekly wraps to next week", monWedFri, "2026-10-16", "2026-10-19"},
		{"weekly single day is a week later", Recurrence{Freq: FreqWeekly, Weekdays: []time.Weekday{time.Wednesday}}, "2026-10-14", "2026-10-21"},
		{"31st in a 31-day month", on31, "2026-01-01", "2026-01-31"},
		{"31st clamps to Feb 28", on31, "2026-01-31", "2026-02-28"},
		{"31st clamps to Feb 29 in a leap year", on31, "2028-01-31", "2028-02-29"},
		{"31st clamps to Apr 30", on31, "2026-03-31", "2026-04-30"},
		{"31st after a clamped date returns to the 31st", on31, "2026-02-28", "2026-03-31"},
		{"29th in non-leap Feb clamps to 28", on29, "2027-01-29", "2027-02-28"},
		{"29th in leap Feb", on29, "2028-01-29", "2028-02-29"},
		{"monthly later this month", on15, "2026-10-03", "2026-10-15"},
		{"monthly on the day moves a month", on15, "2026-10-15", "2026-11-15"},
		{"monthly over year end", on15, "2026-12-20", "2027-01-15"},
	} {
		if got := c.r.Next(day(c.after)); !got.Equal(day(c.want)) {
			t.Errorf("%s: Next(%s) = %s, want %s", c.name, c.after, got.Format(time.DateOnly), c.want)
		}
	}
}

func TestRecurrenceDueSchedule(t *testing.T) {
	r := Recurrence{Freq: FreqDaily, Timezone: "Europe/Istanbul", Mode: ModeSchedule, LastOn: day("2026-10-14")}

	if _, ok := r.Due(at("2026-10-14 23:59", "Europe/Istanbul"), nil); ok {
		t.Fatal("nothing is due on LastOn's own day")
	}
	// 21:30 UTC on the 14th is already the 15th in Istanbul (UTC+3).
	if d, ok := r.Due(time.Date(2026, 10, 14, 21, 30, 0, 0, time.UTC), nil); !ok || !d.Equal(day("2026-10-15")) {
		t.Fatalf("Due = %s %v, want 2026-10-15 (the rule's zone decides the day)", d, ok)
	}
	// After an outage only the newest missed date is due.
	if d, ok := r.Due(at("2026-10-18 08:00", "Europe/Istanbul"), nil); !ok || !d.Equal(day("2026-10-18")) {
		t.Fatalf("Due after outage = %s %v, want 2026-10-18", d, ok)
	}

	weekly := Recurrence{Freq: FreqWeekly, Weekdays: []time.Weekday{time.Monday}, Timezone: "UTC", Mode: ModeSchedule, LastOn: day("2026-10-12")}
	if _, ok := weekly.Due(at("2026-10-18 12:00", "UTC"), nil); ok {
		t.Fatal("weekly Monday rule is not due on Sunday")
	}
}

// Across DST changes every civil date comes exactly once: consecutive runs
// every 30 minutes through the US spring-forward and fall-back nights yield
// each date once, in order.
func TestRecurrenceDueAcrossDST(t *testing.T) {
	for _, c := range []struct{ from, to string }{
		{"2026-03-07 00:00", "2026-03-10 00:00"}, // spring forward Mar 8 (02:00 → 03:00)
		{"2026-10-31 00:00", "2026-11-03 00:00"}, // fall back Nov 1 (02:00 → 01:00)
	} {
		r := Recurrence{Freq: FreqDaily, Timezone: "America/New_York", Mode: ModeSchedule}
		start, end := at(c.from, "America/New_York"), at(c.to, "America/New_York")
		r.LastOn = r.Today(start)
		var got []string
		for now := start; !now.After(end); now = now.Add(30 * time.Minute) {
			if d, ok := r.Due(now, nil); ok {
				got = append(got, d.Format(time.DateOnly))
				r.LastOn = d
			}
		}
		if len(got) != 3 {
			t.Fatalf("%s → %s: generated %v, want 3 consecutive days", c.from, c.to, got)
		}
		for i := 1; i < len(got); i++ {
			if !day(got[i]).Equal(day(got[i-1]).AddDate(0, 0, 1)) {
				t.Fatalf("%s → %s: generated %v, want consecutive days", c.from, c.to, got)
			}
		}
	}
}

func TestRecurrenceDueAfterCompletion(t *testing.T) {
	r := Recurrence{Freq: FreqWeekly, Weekdays: []time.Weekday{time.Monday, time.Thursday}, Timezone: "Asia/Tokyo",
		Mode: ModeAfterCompletion, LastOn: day("2026-10-12")} // Monday

	if _, ok := r.Due(at("2026-10-30 09:00", "Asia/Tokyo"), nil); ok {
		t.Fatal("nothing is due while the latest instance is open")
	}
	// Completed Fri Oct 16 at 23:30 UTC = Sat Oct 17 in Tokyo → next is Mon Oct 19.
	done := time.Date(2026, 10, 16, 23, 30, 0, 0, time.UTC)
	if _, ok := r.Due(at("2026-10-18 12:00", "Asia/Tokyo"), &done); ok {
		t.Fatal("not due before the next matching day")
	}
	if d, ok := r.Due(at("2026-10-25 12:00", "Asia/Tokyo"), &done); !ok || !d.Equal(day("2026-10-19")) {
		t.Fatalf("Due = %s %v, want 2026-10-19 (first match after completion, no skipping ahead)", d, ok)
	}
	// Completed before its own date (LastOn): count from LastOn.
	early := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	if d, ok := r.Due(at("2026-10-15 12:00", "Asia/Tokyo"), &early); !ok || !d.Equal(day("2026-10-15")) {
		t.Fatalf("Due = %s %v, want 2026-10-15", d, ok)
	}
}

func TestRecurrenceRestart(t *testing.T) {
	r := Recurrence{Freq: FreqWeekly, Weekdays: []time.Weekday{time.Monday}, Timezone: "UTC", Mode: ModeSchedule, LastOn: day("2026-09-07")}
	now := at("2026-10-14 10:00", "UTC") // Wednesday; Monday Oct 12 passed while paused
	r.Restart(now)
	if !r.LastOn.Equal(day("2026-10-13")) {
		t.Fatalf("LastOn = %s, want yesterday", r.LastOn)
	}
	if _, ok := r.Due(now, nil); ok {
		t.Fatal("a resume must not back-fill the Monday that passed while paused")
	}
	r.LastOn = day("2026-10-14") // today already generated
	r.Restart(now)
	if !r.LastOn.Equal(day("2026-10-14")) {
		t.Fatal("Restart must never move LastOn back")
	}
}

func TestRecurrenceValidate(t *testing.T) {
	ok := Recurrence{Freq: FreqMonthly, MonthDay: 31, Timezone: "Europe/Istanbul", Mode: ModeSchedule}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid rule: %v", err)
	}
	for name, r := range map[string]Recurrence{
		"bad zone":       {Freq: FreqDaily, Timezone: "Mars/Olympus", Mode: ModeSchedule},
		"empty zone":     {Freq: FreqDaily, Mode: ModeSchedule},
		"bad mode":       {Freq: FreqDaily, Timezone: "UTC", Mode: "sometimes"},
		"bad freq":       {Freq: "yearly", Timezone: "UTC", Mode: ModeSchedule},
		"weekly no days": {Freq: FreqWeekly, Timezone: "UTC", Mode: ModeSchedule},
		"weekday 7":      {Freq: FreqWeekly, Weekdays: []time.Weekday{7}, Timezone: "UTC", Mode: ModeSchedule},
		"month day 0":    {Freq: FreqMonthly, Timezone: "UTC", Mode: ModeSchedule},
		"month day 32":   {Freq: FreqMonthly, MonthDay: 32, Timezone: "UTC", Mode: ModeSchedule},
	} {
		if err := r.Validate(); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
}
