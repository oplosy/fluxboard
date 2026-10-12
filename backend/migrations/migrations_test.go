package migrations

import (
	"testing"
	"testing/fstest"
)

func TestLatest(t *testing.T) {
	got, err := Latest()
	if err != nil {
		t.Fatal(err)
	}
	// Versions are numbered without gaps, so the newest equals the count.
	names, _ := files.ReadDir(".")
	if want := uint(len(names)); got != want {
		t.Fatalf("Latest() = %d, want %d (one per embedded up migration)", got, want)
	}
}

func TestLatestRejectsBadNames(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"no prefix":   {"init.up.sql": {}},
		"bad version": {"v1_init.up.sql": {}},
		"empty":       {},
	} {
		if _, err := latest(fsys); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	got, err := latest(fstest.MapFS{"0002_b.up.sql": {}, "0010_c.up.sql": {}, "0001_a.up.sql": {}})
	if err != nil || got != 10 {
		t.Fatalf("latest = (%d, %v), want (10, nil)", got, err)
	}
}
