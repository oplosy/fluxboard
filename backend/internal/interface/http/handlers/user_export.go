package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/auth"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

// ExportSources are the other services the personal data export reads from
// (report B09). Wired in cmd/api; a nil source is left out of the export.
type ExportSources struct {
	Sessions func(ctx context.Context, userID string) ([]*auth.Session, error)
	Orgs     func(ctx context.Context, userID string) ([]tenant.OrgMembership, error)
}

type exportAccount struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	EmailVerified    bool      `json:"email_verified"`
	TwoFactorEnabled bool      `json:"two_factor_enabled"`
	PlatformRole     string    `json:"platform_role"`
	HasAvatar        bool      `json:"has_avatar"`
	CreatedAt        time.Time `json:"created_at"`
}

type exportIdentity struct {
	Provider string    `json:"provider"`
	Subject  string    `json:"subject"`
	LinkedAt time.Time `json:"linked_at"`
}

type exportOrg struct {
	OrgID    string    `json:"org_id"`
	Name     string    `json:"name"`
	Slug     string    `json:"slug"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type exportSession struct {
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	IP         string    `json:"ip,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
}

type exportResp struct {
	FormatVersion int              `json:"format_version"`
	GeneratedAt   time.Time        `json:"generated_at"`
	Account       exportAccount    `json:"account"`
	Identities    []exportIdentity `json:"sign_in_identities"`
	Organizations []exportOrg      `json:"organizations"`
	Sessions      []exportSession  `json:"sessions"`
	ContentNote   string           `json:"content_note"`
}

const exportContentNote = "Projects, tasks, comments and files you created belong to the organizations " +
	"listed above, which control that data; ask an owner or admin of the organization to export or delete it. " +
	"Secrets (password verifier, two-factor secret, recovery codes, tokens) are never exported."

// ExportMe returns the caller's personal data as a JSON download (GET
// /me/export, KVKK art. 11 / GDPR art. 15 and 20, report B09). An admin
// impersonation token is refused: it may read an org, never a person's export.
func (h *UserHandlers) ExportMe(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrUnauthorized)
		return
	}
	if p.ImpersonatedOrg != "" {
		response.Error(w, domain.ErrForbidden)
		return
	}
	ctx := r.Context()
	acct, err := h.svc.Export(ctx, p.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	now := time.Now().UTC()
	out := exportResp{
		FormatVersion: 1,
		GeneratedAt:   now,
		Account: exportAccount{
			ID: acct.ID, Email: acct.Email, Name: acct.Name, EmailVerified: acct.EmailVerified,
			TwoFactorEnabled: acct.TOTPEnabled, PlatformRole: acct.PlatformRole,
			HasAvatar: acct.HasAvatar, CreatedAt: acct.CreatedAt,
		},
		Identities:    make([]exportIdentity, 0, len(acct.Identities)),
		Organizations: []exportOrg{},
		Sessions:      []exportSession{},
		ContentNote:   exportContentNote,
	}
	for _, id := range acct.Identities {
		out.Identities = append(out.Identities, exportIdentity{Provider: id.Provider, Subject: id.ProviderSub, LinkedAt: id.CreatedAt})
	}
	if h.Export.Orgs != nil {
		orgs, err := h.Export.Orgs(ctx, p.UserID)
		if err != nil {
			response.Error(w, err)
			return
		}
		for _, o := range orgs {
			out.Organizations = append(out.Organizations, exportOrg{
				OrgID: o.OrgID, Name: o.Name, Slug: o.Slug, Role: string(o.Role), JoinedAt: o.CreatedAt,
			})
		}
	}
	if h.Export.Sessions != nil {
		sessions, err := h.Export.Sessions(ctx, p.UserID)
		if err != nil {
			response.Error(w, err)
			return
		}
		for _, s := range sessions {
			out.Sessions = append(out.Sessions, exportSession{
				CreatedAt: s.CreatedAt, LastUsedAt: s.LastUsedAt, ExpiresAt: s.ExpiresAt,
				IP: s.IP, UserAgent: s.UserAgent,
			})
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="fluxboard-personal-data-%s.json"`, now.Format("2006-01-02")))
	response.JSON(w, http.StatusOK, out)
}
