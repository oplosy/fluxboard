package taskuc

import (
	"context"
	"errors"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type memWatchers struct{ ids map[string][]string }

func (m *memWatchers) Add(_ context.Context, _, taskID, userID string) error {
	for _, id := range m.ids[taskID] {
		if id == userID {
			return nil
		}
	}
	m.ids[taskID] = append(m.ids[taskID], userID)
	return nil
}

func (m *memWatchers) Remove(_ context.Context, _, taskID, userID string) error {
	kept := m.ids[taskID][:0]
	for _, id := range m.ids[taskID] {
		if id != userID {
			kept = append(kept, id)
		}
	}
	m.ids[taskID] = kept
	return nil
}

func (m *memWatchers) ListByTask(_ context.Context, _, taskID string) ([]project.Watcher, error) {
	out := []project.Watcher{}
	for _, id := range m.ids[taskID] {
		out = append(out, project.Watcher{UserID: id})
	}
	return out, nil
}

type memComments struct {
	project.CommentRepository
	byID map[string]*project.Comment
}

func (m *memComments) Create(_ context.Context, _ string, c *project.Comment) error {
	m.byID[c.ID] = c
	return nil
}

func (m *memComments) Get(_ context.Context, _, id string) (*project.Comment, error) {
	return m.byID[id], nil
}

type commentSpy struct {
	Notifier
	targets []string
}

func (n *commentSpy) FanOutComment(_ context.Context, _, _, _, _, _, _ string, targets []string) {
	n.targets = targets
}

func newWatcherSvc(memberRole *project.ProjectRole, vis project.Visibility, w project.WatcherRepository, n Notifier) *Service {
	return New(Deps{
		Tasks:    &claimStore{tasks: map[string]*project.Task{"t1": {ID: "t1", ProjectID: "p1", CreatedBy: "creator"}}},
		Comments: &memComments{byID: map[string]*project.Comment{}},
		Watchers: w, Notifier: n,
		Projects: fakeProjects{p: &project.Project{ID: "p1", Visibility: vis}},
		Members:  fakeMembers{role: memberRole},
	})
}

func TestWatchers(t *testing.T) {
	ctx := context.Background()

	t.Run("watch is idempotent and unwatch removes", func(t *testing.T) {
		repo := &memWatchers{ids: map[string][]string{}}
		svc := newWatcherSvc(role(project.RoleViewer), project.VisibilityOrg, repo, nil)
		for range 2 {
			if err := svc.Watch(ctx, "o", "u1", "t1", tenant.RoleMember); err != nil {
				t.Fatal(err)
			}
		}
		ws, err := svc.ListWatchers(ctx, "o", "u1", "t1", tenant.RoleMember)
		if err != nil || len(ws) != 1 || ws[0].UserID != "u1" {
			t.Fatalf("watchers = %+v %v", ws, err)
		}
		if err := svc.Unwatch(ctx, "o", "u1", "t1", tenant.RoleMember); err != nil {
			t.Fatal(err)
		}
		if ws, _ := svc.ListWatchers(ctx, "o", "u1", "t1", tenant.RoleMember); len(ws) != 0 {
			t.Fatalf("watchers after unwatch = %+v", ws)
		}
	})
	t.Run("non-member of a private project cannot watch", func(t *testing.T) {
		svc := newWatcherSvc(nil, project.VisibilityPrivate, &memWatchers{ids: map[string][]string{}}, nil)
		if err := svc.Watch(ctx, "o", "u1", "t1", tenant.RoleMember); err == nil {
			t.Fatal("want an access error")
		}
	})
	t.Run("disabled without a repository", func(t *testing.T) {
		svc := newWatcherSvc(role(project.RoleViewer), project.VisibilityOrg, nil, nil)
		if err := svc.Watch(ctx, "o", "u1", "t1", tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("err = %v, want ErrForbidden", err)
		}
	})
	t.Run("comments fan out to watchers", func(t *testing.T) {
		repo := &memWatchers{ids: map[string][]string{"t1": {"w1", "w2"}}}
		spy := &commentSpy{}
		svc := newWatcherSvc(role(project.RoleContributor), project.VisibilityOrg, repo, spy)
		if _, err := svc.AddComment(ctx, "o", "author", "t1", "hello", tenant.RoleMember); err != nil {
			t.Fatal(err)
		}
		want := []string{"creator", "w1", "w2"}
		if len(spy.targets) != len(want) {
			t.Fatalf("targets = %v, want %v", spy.targets, want)
		}
		for i := range want {
			if spy.targets[i] != want[i] {
				t.Fatalf("targets = %v, want %v", spy.targets, want)
			}
		}
	})
}
