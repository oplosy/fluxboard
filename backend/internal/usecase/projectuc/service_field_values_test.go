package projectuc

import (
	"context"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// projectValueFields answers ValuesByProject for one project only.
type projectValueFields struct {
	*fakeFields
	projectID string
	values    []project.CustomValue
}

func (f *projectValueFields) ValuesByProject(_ context.Context, _, projectID string) ([]project.CustomValue, error) {
	if projectID != f.projectID {
		return nil, nil
	}
	return f.values, nil
}

func TestProjectFieldValues(t *testing.T) {
	ctx := context.Background()
	text := "blue"
	fields := &projectValueFields{
		fakeFields: newFakeFields(), projectID: "p1",
		values: []project.CustomValue{{TaskID: "t1", FieldID: "f1", Type: "text", Text: &text}},
	}
	svc := func(vis project.Visibility, role *project.ProjectRole) *Service {
		return New(Deps{
			Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: vis}},
			Members:  &fakeMembers{role: role},
			Fields:   fields,
		})
	}

	// A non-member reads an org-visible project's values as a VIEWER.
	got, err := svc(project.VisibilityOrg, nil).ProjectFieldValues(ctx, "o", "u", "p1", tenant.RoleMember)
	if err != nil || len(got) != 1 || got[0].TaskID != "t1" {
		t.Fatalf("org-visible viewer: %+v, %v", got, err)
	}
	// A private project hides its values from non-members.
	if _, err := svc(project.VisibilityPrivate, nil).ProjectFieldValues(ctx, "o", "u", "p1", tenant.RoleMember); err != domain.ErrNotFound {
		t.Fatalf("private non-member: err %v, want ErrNotFound", err)
	}
}
