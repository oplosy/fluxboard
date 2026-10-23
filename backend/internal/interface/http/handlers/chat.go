// This file is the org settings chat-channel surface (FR-NTF-005, ADR-038):
// Slack / Teams incoming-webhook channels. All routes are ADMIN+ (write:chat in
// the Casbin policy). The webhook URL is accepted on create only; responses
// carry a display hint, never the URL or its ciphertext.
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/chatuc"
)

// ChatHandlers serves the org-scoped chat channel endpoints.
type ChatHandlers struct {
	svc *chatuc.Service
	log *slog.Logger
}

// NewChatHandlers builds ChatHandlers.
func NewChatHandlers(svc *chatuc.Service, log *slog.Logger) *ChatHandlers {
	if log == nil {
		log = slog.Default()
	}
	return &ChatHandlers{svc: svc, log: log}
}

type chatChannelResp struct {
	ID         string    `json:"id"`
	Provider   string    `json:"provider"`
	Name       string    `json:"name"`
	URLHint    string    `json:"url_hint"`
	Categories []string  `json:"categories"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

func toChatChannelResp(c *chat.Channel) chatChannelResp {
	return chatChannelResp{
		ID: c.ID, Provider: string(c.Provider), Name: c.Name, URLHint: c.URLHint,
		Categories: c.Categories, Enabled: c.Enabled, CreatedAt: c.CreatedAt,
	}
}

type chatDeliveryResp struct {
	ID          string     `json:"id"`
	ChannelID   string     `json:"channel_id"`
	ChannelName string     `json:"channel_name,omitempty"`
	Category    string     `json:"category"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at"`
	SentAt      *time.Time `json:"sent_at"`
}

func toChatDeliveryResp(d *chat.Delivery) chatDeliveryResp {
	return chatDeliveryResp{
		ID: d.ID, ChannelID: d.ChannelID, ChannelName: d.ChannelName, Category: d.Category,
		Title: d.Title, Status: d.Status, Attempts: d.Attempts, LastError: d.LastError,
		CreatedAt: d.CreatedAt, SentAt: d.SentAt,
	}
}

// List: GET /orgs/{orgId}/chat-channels
func (h *ChatHandlers) List(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	cs, err := h.svc.List(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]chatChannelResp, 0, len(cs))
	for i := range cs {
		out = append(out, toChatChannelResp(&cs[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"channels": out})
}

type createChatChannelReq struct {
	Provider   string   `json:"provider"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Categories []string `json:"categories"`
}

// Create: POST /orgs/{orgId}/chat-channels
func (h *ChatHandlers) Create(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req createChatChannelReq
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := h.svc.Create(r.Context(), tc.OrgID, tc.UserID, chatuc.CreateInput{
		Provider: chat.Provider(req.Provider), Name: req.Name, URL: req.URL, Categories: req.Categories,
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"channel": toChatChannelResp(c)})
}

type updateChatChannelReq struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Enabled    bool     `json:"enabled"`
}

// Update: PATCH /orgs/{orgId}/chat-channels/{channelId}
func (h *ChatHandlers) Update(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req updateChatChannelReq
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := h.svc.Update(r.Context(), tc.OrgID, chi.URLParam(r, "channelId"), chatuc.UpdateInput{
		Name: req.Name, Categories: req.Categories, Enabled: req.Enabled,
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"channel": toChatChannelResp(c)})
}

// Delete: DELETE /orgs/{orgId}/chat-channels/{channelId}
func (h *ChatHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Delete(r.Context(), tc.OrgID, chi.URLParam(r, "channelId")); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test: POST /orgs/{orgId}/chat-channels/{channelId}/test — queues a test message.
func (h *ChatHandlers) Test(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	d, err := h.svc.Test(r.Context(), tc.OrgID, chi.URLParam(r, "channelId"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"delivery": toChatDeliveryResp(d)})
}

// Deliveries: GET /orgs/{orgId}/chat-deliveries — newest first.
func (h *ChatHandlers) Deliveries(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	ds, err := h.svc.Deliveries(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]chatDeliveryResp, 0, len(ds))
	for i := range ds {
		out = append(out, toChatDeliveryResp(&ds[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"deliveries": out})
}
