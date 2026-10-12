package demoseed

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/pkg/argon2x"
)

var testClock = time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)

func TestBuildIsDeterministic(t *testing.T) {
	if !reflect.DeepEqual(Build(testClock), Build(testClock)) {
		t.Fatal("Build is not deterministic for a fixed clock")
	}
	a, b := Build(testClock), Build(testClock.Add(24*time.Hour))
	if a.Tasks[0].ID != b.Tasks[0].ID || a.Users[0].ID != b.Users[0].ID {
		t.Fatal("IDs must not depend on the clock")
	}
}

// The counts promised by docs/12-TESTING.md §5.
func TestBuildMatchesSpec(t *testing.T) {
	f := Build(testClock)
	orgBySlug := map[string]string{}
	for _, o := range f.Orgs {
		orgBySlug[o.Slug] = o.ID
	}
	members := map[string]int{}
	for _, m := range f.Memberships {
		members[m.OrgID]++
	}
	projects, tasks := map[string]int{}, map[string]int{}
	for _, p := range f.Projects {
		projects[p.OrgID]++
	}
	for _, tk := range f.Tasks {
		tasks[tk.OrgID]++
	}
	acme, beta := orgBySlug["acme"], orgBySlug["beta-co"]
	if members[acme] != 20 || projects[acme] != 6 || tasks[acme] != 400 {
		t.Fatalf("acme members/projects/tasks = %d/%d/%d, want 20/6/400", members[acme], projects[acme], tasks[acme])
	}
	if members[beta] != 2 {
		t.Fatalf("beta-co members = %d, want 2", members[beta])
	}
	if len(f.Usage) != 30*3 {
		t.Fatalf("usage rows = %d, want 90 (30 days × 3 metrics)", len(f.Usage))
	}

	subs := map[string]string{}
	for _, s := range f.Subscriptions {
		subs[s.OrgID] = s.PlanCode + "/" + s.Status
	}
	if subs[acme] != "business/active" || subs[orgBySlug["pastdue-inc"]] != "pro/past_due" || subs[beta] != "" {
		t.Fatalf("subscriptions = %v", subs)
	}

	var archived, private int
	for _, p := range f.Projects {
		if p.OrgID != acme {
			continue
		}
		if p.ArchivedAt != nil {
			archived++
		}
		if p.Visibility == "private" {
			private++
		}
	}
	if archived != 1 || private != 1 {
		t.Fatalf("acme archived/private projects = %d/%d, want 1/1", archived, private)
	}
}

// Rows must satisfy the schema: unique IDs, unique ranks per column, per-project
// numbering matching task_counter, valid project keys, labels from the same org.
func TestBuildSatisfiesConstraints(t *testing.T) {
	f := Build(testClock)
	seen := map[string]bool{}
	unique := func(kind, id string) {
		if seen[id] {
			t.Fatalf("duplicate %s id %s", kind, id)
		}
		seen[id] = true
	}
	keyRE := regexp.MustCompile(`^[A-Z]{2,6}$`)
	board := map[string]string{} // project -> board
	for _, p := range f.Projects {
		unique("project", p.ID)
		unique("board", p.BoardID)
		board[p.ID] = p.BoardID
		if !keyRE.MatchString(p.Key) {
			t.Fatalf("project key %q violates the schema check", p.Key)
		}
	}
	colBoard := map[string]string{}
	for _, c := range f.Columns {
		unique("column", c.ID)
		colBoard[c.ID] = c.BoardID
	}
	labelOrg := map[string]string{}
	for _, l := range f.Labels {
		unique("label", l.ID)
		labelOrg[l.ID] = l.OrgID
	}
	ranks := map[string]bool{}
	maxNumber := map[string]int{}
	for _, tk := range f.Tasks {
		unique("task", tk.ID)
		if colBoard[tk.ColumnID] != board[tk.ProjectID] {
			t.Fatalf("task %s sits in a column of another board", tk.ID)
		}
		if k := tk.ColumnID + "|" + tk.Rank; tk.Rank == "" || ranks[k] {
			t.Fatalf("task %s rank %q empty or duplicated in its column", tk.ID, tk.Rank)
		} else {
			ranks[k] = true
		}
		if tk.Number > maxNumber[tk.ProjectID] {
			maxNumber[tk.ProjectID] = tk.Number
		}
		if tk.CreatedAt.After(testClock) {
			t.Fatalf("task %s created after the clock", tk.ID)
		}
	}
	for _, p := range f.Projects {
		if maxNumber[p.ID] != p.TaskCounter {
			t.Fatalf("project %s task_counter %d, highest number %d", p.Key, p.TaskCounter, maxNumber[p.ID])
		}
	}
	for _, tl := range f.TaskLabels {
		if labelOrg[tl.LabelID] != tl.OrgID {
			t.Fatalf("task label crosses orgs: %+v", tl)
		}
	}
}

// Assignees must be able to see the task: every assignee is a LEAD or
// CONTRIBUTOR of the task's project, and the private project excludes
// member@demo.dev (the isolation tests rely on it).
func TestBuildAssigneesAndPrivateProject(t *testing.T) {
	f := Build(testClock)
	roles := map[string]string{} // project|user -> role
	for _, m := range f.ProjectMembers {
		roles[m.ProjectID+"|"+m.UserID] = m.Role
	}
	for _, tk := range f.Tasks {
		if tk.AssigneeID == nil {
			continue
		}
		if r := roles[tk.ProjectID+"|"+*tk.AssigneeID]; r != "LEAD" && r != "CONTRIBUTOR" {
			t.Fatalf("task %s assigned to a non-contributor (role %q)", tk.ID, r)
		}
	}
	member := ID("user", "member")
	for _, p := range f.Projects {
		if p.Visibility == "private" && roles[p.ID+"|"+member] != "" {
			t.Fatal("member@demo.dev must not be a member of the private project")
		}
	}
}

func TestDemoPasswordPassesPolicy(t *testing.T) {
	for _, u := range Build(testClock).Users {
		if err := argon2x.CheckPolicy(DemoPassword, u.Email); err != nil {
			t.Fatalf("demo password rejected for %s: %v", u.Email, err)
		}
	}
}
