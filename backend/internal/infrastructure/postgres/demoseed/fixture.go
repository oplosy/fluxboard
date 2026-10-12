// Package demoseed holds the deterministic demo fixture of docs/12-TESTING.md
// §5: Build derives every row from a clock with fixed IDs, Write loads it.
// Used by `make seed` (cmd/seed) and by the tenant-isolation integration suite.
package demoseed

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/rank"
)

// DemoPassword is the shared password of every seeded user (docs/12-TESTING.md
// §5). Local dev only: the command refuses APP_ENV=prod.
const DemoPassword = "Demo-Pass-2026!"

// PlatformAdminTOTPSecret is the fixed base32 TOTP secret of padmin@demo.dev,
// so the admin console (which requires 2FA) is reachable in dev by adding it to
// an authenticator. Dev only, like DemoPassword.
const PlatformAdminTOTPSecret = "JBSWY3DPEHPK3PXP"

// seedNamespace roots the UUIDv5 IDs: the same name always yields the same ID,
// so every run produces identical rows (fixed UUIDs, §5).
var seedNamespace = uuid.MustParse("5f1c0b7e-6a43-4f8e-9c1d-2b7a9e0d4c11")

// ID returns the fixed seed UUID for a name, e.g. ID("org", "acme") or
// ID("user", "member").
func ID(parts ...string) string {
	return uuid.NewSHA1(seedNamespace, []byte(strings.Join(parts, "/"))).String()
}

// User is a seeded account. Every user is email-verified.
type User struct {
	ID, Email, Name string
	PlatformAdmin   bool
}

// Org is a seeded organization; StripeCustomerID is empty for free orgs.
type Org struct {
	ID, Slug, Name, StripeCustomerID string
	CreatedAt                        time.Time
}

// Membership is an org role grant.
type Membership struct{ OrgID, UserID, Role string }

// Project is a seeded project with its board.
type Project struct {
	ID, OrgID, Key, Name, Visibility, CreatedBy string
	BoardID                                     string
	ArchivedAt                                  *time.Time
	TaskCounter                                 int
	CreatedAt                                   time.Time
}

// ProjectMember is a project role grant.
type ProjectMember struct{ ProjectID, OrgID, UserID, Role string }

// Column is a board column (project.DefaultColumns, in order).
type Column struct{ ID, OrgID, BoardID, Name, Rank string }

// Task is a seeded task. Rank is unique within its column.
type Task struct {
	ID, OrgID, ProjectID, ColumnID string
	Number                         int
	Title, Description, Priority   string
	AssigneeID                     *string
	StartDate, DueDate             *time.Time
	Rank, CreatedBy                string
	CreatedAt, UpdatedAt           time.Time
}

// Label is an org label; TaskLabel attaches one to a task.
type Label struct{ ID, OrgID, Name, Color string }

// TaskLabel links a task to a label.
type TaskLabel struct{ TaskID, LabelID, OrgID string }

// Subscription is the local Stripe mirror row of a paid org.
type Subscription struct {
	ID, OrgID, PlanCode, Status   string
	StripeSubID, StripeCustomerID string
	CurrentPeriodEnd              time.Time
}

// Usage is one daily usage aggregate.
type Usage struct {
	OrgID    string
	Metric   billing.UsageMetric
	Day      time.Time
	Value    int64
	PushedAt *time.Time
}

// Fixture is the complete deterministic dataset.
type Fixture struct {
	Clock          time.Time
	Users          []User
	Orgs           []Org
	Memberships    []Membership
	Projects       []Project
	ProjectMembers []ProjectMember
	Columns        []Column
	Tasks          []Task
	Labels         []Label
	TaskLabels     []TaskLabel
	Subscriptions  []Subscription
	Usage          []Usage
}

// Build returns the fixture of docs/12-TESTING.md §5, anchored at clock (UTC
// midnight). Same clock in, same rows out.
func Build(clock time.Time) Fixture {
	f := Fixture{Clock: clock}
	day := 24 * time.Hour

	user := func(key, email, name string, admin bool) string {
		u := User{ID: ID("user", key), Email: email, Name: name, PlatformAdmin: admin}
		f.Users = append(f.Users, u)
		return u.ID
	}
	owner := user("owner", "owner@demo.dev", "Olivia Owner", false)
	admin := user("admin", "admin@demo.dev", "Adam Admin", false)
	member := user("member", "member@demo.dev", "Mia Member", false)
	guest := user("guest", "guest@demo.dev", "Gus Guest", false)
	outsider := user("outsider", "outsider@demo.dev", "Otto Outsider", false)
	user("padmin", "padmin@demo.dev", "Paula Platform", true)
	var team []string
	for i := 1; i <= 16; i++ {
		team = append(team, user(fmt.Sprintf("user%02d", i), fmt.Sprintf("user%02d@demo.dev", i), fmt.Sprintf("Teammate %02d", i), false))
	}
	betaMember := user("beta.member", "beta.member@demo.dev", "Bea Beta", false)

	org := func(slug, name, customer string, age time.Duration) string {
		o := Org{ID: ID("org", slug), Slug: slug, Name: name, StripeCustomerID: customer, CreatedAt: clock.Add(-age)}
		f.Orgs = append(f.Orgs, o)
		return o.ID
	}
	acme := org("acme", "Acme Inc.", "cus_seed_acme", 120*day)
	beta := org("beta-co", "Beta Co", "", 40*day)
	pastdue := org("pastdue-inc", "Pastdue Inc.", "cus_seed_pastdue", 90*day)

	grant := func(orgID, userID, role string) {
		f.Memberships = append(f.Memberships, Membership{orgID, userID, role})
	}
	// acme: 20 members (owner, admin, member, guest + 16 teammates).
	grant(acme, owner, "OWNER")
	grant(acme, admin, "ADMIN")
	grant(acme, member, "MEMBER")
	grant(acme, guest, "GUEST")
	for _, u := range team {
		grant(acme, u, "MEMBER")
	}
	// beta-co: 2 members, none of them in acme (the isolation suite's org B).
	grant(beta, outsider, "OWNER")
	grant(beta, betaMember, "MEMBER")
	grant(pastdue, owner, "OWNER")

	labels := map[string][]string{}
	label := func(orgID, name, color string) {
		l := Label{ID: ID("label", orgID, name), OrgID: orgID, Name: name, Color: color}
		f.Labels = append(f.Labels, l)
		labels[orgID] = append(labels[orgID], l.ID)
	}
	for _, l := range [][2]string{{"bug", "#ef4444"}, {"feature", "#3b82f6"}, {"chore", "#6b7280"}, {"customer", "#f59e0b"}} {
		label(acme, l[0], l[1])
	}
	label(beta, "bug", "#ef4444")

	archivedAt := clock.Add(-10 * day)
	type spec struct {
		org, key, name, visibility string
		archived                   *time.Time
		tasks                      int
		lead                       string
		contributors, viewers      []string
		marker                     string // searchable string unique to this project (§6)
	}
	teamFor := func(i int) []string { // teammates spread over the four active org projects
		var out []string
		for k, u := range team {
			if k%4 == i {
				out = append(out, u)
			}
		}
		return out
	}
	specs := []spec{
		{acme, "WEB", "Website", "org", nil, 100, owner, append([]string{member}, teamFor(0)...), []string{guest}, ""},
		{acme, "API", "Public API", "org", nil, 90, owner, append([]string{member}, teamFor(1)...), nil, ""},
		{acme, "MOB", "Mobile App", "org", nil, 80, owner, append([]string{member}, teamFor(2)...), nil, ""},
		{acme, "OPS", "Operations", "org", nil, 60, owner, teamFor(3), nil, ""},
		{acme, "OLD", "Legacy Portal", "org", &archivedAt, 40, owner, team[:4], nil, ""},
		{acme, "SEC", "Security Audit", "private", nil, 30, team[0], []string{team[1]}, nil, "secaudit-private-marker"},
		{beta, "BETA", "Beta Launch", "org", nil, 6, outsider, []string{betaMember}, nil, "betaco-marker"},
		{pastdue, "PAY", "Payments", "org", nil, 4, owner, nil, nil, "pastdue-marker"},
	}

	for _, s := range specs {
		p := Project{
			ID: ID("project", s.org, s.key), OrgID: s.org, Key: s.key, Name: s.name,
			Visibility: s.visibility, CreatedBy: s.lead, ArchivedAt: s.archived, TaskCounter: s.tasks,
			CreatedAt: clock.Add(-75 * day),
		}
		p.BoardID = ID("board", p.ID)
		f.Projects = append(f.Projects, p)

		// Org OWNER/ADMIN are effective LEAD on every project without a row, so
		// the private project lists only its own members.
		addMember := func(userID, role string) {
			f.ProjectMembers = append(f.ProjectMembers, ProjectMember{p.ID, p.OrgID, userID, role})
		}
		addMember(s.lead, "LEAD")
		for _, u := range s.contributors {
			addMember(u, "CONTRIBUTOR")
		}
		for _, u := range s.viewers {
			addMember(u, "VIEWER")
		}
		assignees := append([]string{s.lead}, s.contributors...)

		cols := make([]Column, len(project.DefaultColumns))
		colRanks := rank.Initial(len(cols))
		for i, name := range project.DefaultColumns {
			cols[i] = Column{ID: ID("column", p.ID, name), OrgID: p.OrgID, BoardID: p.BoardID, Name: name, Rank: colRanks[i]}
		}
		f.Columns = append(f.Columns, cols...)

		byColumn := make([][]int, len(cols)) // task indexes per column, in number order
		for n := 1; n <= s.tasks; n++ {
			t := buildTask(clock, p, n, s.tasks, assignees, s.marker)
			col := columnFor(clock, t, p.Key, n)
			t.ColumnID = cols[col].ID
			byColumn[col] = append(byColumn[col], len(f.Tasks))
			f.Tasks = append(f.Tasks, t)
			for _, lid := range labelsFor(labels[p.OrgID], p.Key, n) {
				f.TaskLabels = append(f.TaskLabels, TaskLabel{t.ID, lid, p.OrgID})
			}
		}
		for _, idxs := range byColumn {
			ranks := rank.Initial(len(idxs))
			for k, ti := range idxs {
				f.Tasks[ti].Rank = ranks[k]
			}
		}
	}

	f.Subscriptions = []Subscription{
		{ID("subscription", acme), acme, "business", "active", "sub_seed_acme", "cus_seed_acme", clock.Add(18 * day)},
		{ID("subscription", pastdue), pastdue, "pro", "past_due", "sub_seed_pastdue", "cus_seed_pastdue", clock.Add(-3 * day)},
	}

	// 30 days of acme usage, ending today; days before today count as pushed
	// to Stripe (business is metered).
	for d := 29; d >= 0; d-- {
		date := clock.Add(-time.Duration(d) * day)
		var pushed *time.Time
		if d > 0 {
			at := date.Add(day + time.Hour)
			pushed = &at
		}
		h := hash("usage", fmt.Sprint(d))
		f.Usage = append(f.Usage,
			Usage{acme, billing.MetricActiveMembers, date, int64(12 + h%9), pushed},
			Usage{acme, billing.MetricStorageBytes, date, int64(1_200_000_000 + (29-d)*80_000_000), pushed},
			Usage{acme, billing.MetricAPICalls, date, int64(2_000 + h%1_500), pushed},
		)
	}
	return f
}

var (
	verbs = []string{"Fix", "Add", "Refactor", "Document", "Investigate", "Ship", "Review", "Migrate", "Test", "Design"}
	nouns = []string{"login flow", "billing page", "search index", "export job", "rate limiter", "onboarding emails",
		"settings screen", "audit log", "webhook retries", "dashboard charts", "invite flow", "error states"}
	priorities = []string{"urgent", "high", "high", "medium", "medium", "medium", "low", "low", "none", "none"}
)

// buildTask derives one task deterministically from (project, number): ages
// spread over the last 60 days, ~80% assigned, ~60% with a due date (so some
// open tasks are overdue), ~30% with a start date.
func buildTask(clock time.Time, p Project, n, total int, assignees []string, marker string) Task {
	day := 24 * time.Hour
	h := hash(p.Key, fmt.Sprint(n))
	created := clock.Add(-time.Duration(60*(total-n)/total)*day - time.Duration(h%20)*time.Hour - time.Hour)
	t := Task{
		ID: ID("task", p.ID, fmt.Sprint(n)), OrgID: p.OrgID, ProjectID: p.ID, Number: n,
		Title:     fmt.Sprintf("%s %s", verbs[h%uint32(len(verbs))], nouns[(h/7)%uint32(len(nouns))]),
		Priority:  priorities[(h/13)%uint32(len(priorities))],
		CreatedBy: p.CreatedBy, CreatedAt: created, UpdatedAt: created.Add(2 * time.Hour),
	}
	if marker != "" {
		t.Description = marker
	}
	if h%10 < 8 && len(assignees) > 0 {
		a := assignees[(h/3)%uint32(len(assignees))]
		t.AssigneeID = &a
	}
	if h%10 < 6 {
		due := created.Add(time.Duration(7+h%22) * day)
		t.DueDate = &due
	}
	if (h/5)%10 < 3 {
		start := created.Add(day)
		t.StartDate = &start
	}
	return t
}

// columnFor buckets a task by age: older tasks are mostly done, recent ones
// mostly open. Index into project.DefaultColumns (Backlog, Todo, In Progress, Done).
func columnFor(clock time.Time, t Task, key string, n int) int {
	r := hash(key, fmt.Sprint(n), "column") % 100
	old := clock.Sub(t.CreatedAt) > 21*24*time.Hour
	switch {
	case old && r < 60, !old && r < 15:
		return 3
	case old && r < 75, !old && r < 40:
		return 2
	case old && r < 90, !old && r < 70:
		return 1
	default:
		return 0
	}
}

// labelsFor attaches zero to two of the org's labels.
func labelsFor(orgLabels []string, key string, n int) []string {
	if len(orgLabels) == 0 {
		return nil
	}
	h := hash(key, fmt.Sprint(n), "labels")
	switch h % 4 {
	case 0, 1:
		return nil
	case 2:
		return []string{orgLabels[h%uint32(len(orgLabels))]}
	default:
		a := h % uint32(len(orgLabels))
		b := (a + 1) % uint32(len(orgLabels))
		if a == b {
			return []string{orgLabels[a]}
		}
		return []string{orgLabels[a], orgLabels[b]}
	}
}

func hash(parts ...string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.Join(parts, "|")))
	return h.Sum32()
}
