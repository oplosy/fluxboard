package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

// liveClaim returns an agent's claim on t while its lease runs; a lapsed claim
// is reported as none (ADR-033: it no longer blocks anyone).
func liveClaim(t *project.Task) (*string, *time.Time) {
	if !t.ClaimActive(time.Now()) {
		return nil, nil
	}
	return t.ClaimedBy, t.LeaseExpiresAt
}

// taskError renders a task write error. A version conflict carries the
// current task so the client can show or merge it without another read
// (FR-AGT-005); anything else goes through the standard mapping.
func taskError(w http.ResponseWriter, err error) {
	var vc *project.VersionConflict
	if !errors.As(err, &vc) || vc.Current == nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusConflict, response.Envelope{Error: response.ErrorBody{
		Code:      project.ErrVersionConflict.Code,
		Message:   project.ErrVersionConflict.Message,
		Details:   map[string]any{"current": toTaskResp(vc.Current)},
		RequestID: w.Header().Get("X-Request-ID"),
	}})
}
