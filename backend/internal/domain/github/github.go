// Package github is the domain model for GitHub pull-request linking
// (FR-TASK-016, ADR-038): webhook signature check, payload parsing and task-key
// extraction. Stdlib only.
package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
)

// MaxKeysPerPR bounds the tasks one pull request can link (abuse guard).
const MaxKeysPerPR = 20

// MaxRepos bounds the repository allow-list.
const MaxRepos = 50

// Event types the webhook accepts.
const (
	EventPing        = "ping"
	EventPullRequest = "pull_request"
)

// ErrBadPayload is returned for a payload that is not a usable pull_request event.
var ErrBadPayload = errors.New("github: bad pull_request payload")

// PR states stored on a link.
const (
	StateOpen   = "open"
	StateClosed = "closed"
	StateMerged = "merged"
)

// Integration is an org's GitHub webhook configuration. SecretEnc is the
// encrypted webhook secret and never leaves the server.
type Integration struct {
	OrgID        string
	SecretEnc    string
	Repos        []string // lowercase owner/name; empty = any repository
	MoveProjects []string // project ids whose linked tasks move to done on merge
	CreatedBy    string
}

// AllowsRepo reports whether a delivery for repo (owner/name) is accepted.
func (i *Integration) AllowsRepo(repo string) bool {
	if len(i.Repos) == 0 {
		return true
	}
	for _, r := range i.Repos {
		if strings.EqualFold(r, repo) {
			return true
		}
	}
	return false
}

// MovesOnMerge reports whether projectID has the move-on-merge option on.
func (i *Integration) MovesOnMerge(projectID string) bool {
	return slices.Contains(i.MoveProjects, projectID)
}

// TaskRef identifies a task a key resolved to.
type TaskRef struct{ TaskID, ProjectID string }

// LinkedPR is a pull request linked to a task.
type LinkedPR struct {
	Repo      string
	Number    int
	Title     string
	State     string
	URL       string
	UpdatedAt time.Time
}

// PullRequestEvent is the part of a pull_request webhook payload we use.
type PullRequestEvent struct {
	Action    string
	Repo      string // lowercase owner/name
	Number    int
	Title     string
	URL       string
	HeadRef   string
	State     string // StateOpen | StateClosed | StateMerged
	UpdatedAt time.Time
}

// Merged reports whether the PR was merged.
func (e *PullRequestEvent) Merged() bool { return e.State == StateMerged }

// Handled reports whether action is one we process.
func Handled(action string) bool {
	switch action {
	case "opened", "edited", "closed", "reopened":
		return true
	}
	return false
}

// VerifySignature checks GitHub's X-Hub-Signature-256 header ("sha256=<hex>")
// against the raw body in constant time.
func VerifySignature(secret string, body []byte, header string) bool {
	hexSig, ok := strings.CutPrefix(header, "sha256=")
	if !ok || secret == "" {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// Sign returns the header value GitHub would send for body (tests, and handy
// for curl-testing a local webhook).
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// ParsePullRequest decodes a pull_request payload. Call only after the
// signature has been verified.
func ParsePullRequest(body []byte) (*PullRequestEvent, error) {
	var p struct {
		Action      string `json:"action"`
		PullRequest struct {
			Number    int       `json:"number"`
			Title     string    `json:"title"`
			HTMLURL   string    `json:"html_url"`
			State     string    `json:"state"`
			Merged    bool      `json:"merged"`
			UpdatedAt time.Time `json:"updated_at"`
			Head      struct {
				Ref string `json:"ref"`
			} `json:"head"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, ErrBadPayload
	}
	pr := p.PullRequest
	repo := strings.ToLower(p.Repository.FullName)
	if pr.Number <= 0 || !ValidRepo(repo) {
		return nil, ErrBadPayload
	}
	state := StateOpen
	switch {
	case pr.Merged:
		state = StateMerged
	case pr.State == "closed":
		state = StateClosed
	}
	if pr.UpdatedAt.IsZero() {
		pr.UpdatedAt = time.Now().UTC()
	}
	return &PullRequestEvent{
		Action: p.Action, Repo: repo, Number: pr.Number, Title: pr.Title,
		URL: pr.HTMLURL, HeadRef: pr.Head.Ref, State: state, UpdatedAt: pr.UpdatedAt.UTC(),
	}, nil
}

var (
	// keyRe matches a project key (2–6 letters, projects.key) and a task number.
	keyRe  = regexp.MustCompile(`\b([A-Za-z]{2,6})-([0-9]{1,9})\b`)
	repoRe = regexp.MustCompile(`^[a-z0-9_.-]+/[a-z0-9_.-]+$`)
)

// ValidRepo reports whether s is a lowercase owner/name repository.
func ValidRepo(s string) bool { return len(s) <= 200 && repoRe.MatchString(s) }

// ExtractKeys returns the distinct upper-cased task keys ("WEB-12") found in
// the given texts, in first-seen order, capped at MaxKeysPerPR. Underscore is
// treated as a separator (it is a word character to the regexp).
func ExtractKeys(texts ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range texts {
		for _, m := range keyRe.FindAllStringSubmatch(strings.ReplaceAll(t, "_", " "), -1) {
			num := strings.TrimLeft(m[2], "0")
			if num == "" { // task numbers start at 1
				continue
			}
			k := strings.ToUpper(m[1]) + "-" + num
			if seen[k] {
				continue
			}
			seen[k] = true
			if out = append(out, k); len(out) == MaxKeysPerPR {
				return out
			}
		}
	}
	return out
}
