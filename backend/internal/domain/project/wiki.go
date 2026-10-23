package project

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// WikiPage is one page of a project's wiki (FR-PROJ-013): a title and a
// Markdown body in the format task descriptions use. Version starts at 1 and
// grows on every edit; editors send the version they read.
type WikiPage struct {
	ID        string
	OrgID     string
	ProjectID string
	Title     string
	Body      string // empty in listings
	Version   int
	CreatedBy string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WikiTaskRef is a live task a page body names by key.
type WikiTaskRef struct {
	Number int
	Title  string
}

const (
	// MaxWikiTitleLen caps a page title (runes).
	MaxWikiTitleLen = 200
	// MaxWikiBodyLen caps a page body (runes).
	MaxWikiBodyLen = 50000
)

// NormalizeWikiPage trims and validates a page's title and body.
func NormalizeWikiPage(title, body string) (string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > MaxWikiTitleLen {
		return "", "", domain.ErrValidation
	}
	if utf8.RuneCountInString(body) > MaxWikiBodyLen {
		return "", "", domain.ErrValidation
	}
	return title, body, nil
}

var taskKeyRe = regexp.MustCompile(`[A-Z]{2,6}-[0-9]{1,9}`)

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// ExtractTaskNumbers returns the sorted, distinct numbers of the tasks a body
// names as KEY-N for the given project key. Matching is case-sensitive and
// whole-word; keys of other projects are ignored (a page only links to tasks of
// its own project, so reading the page never reveals another project's tasks).
func ExtractTaskNumbers(body, projectKey string) []int {
	seen := map[int]bool{}
	for _, loc := range taskKeyRe.FindAllStringIndex(body, -1) {
		s, e := loc[0], loc[1]
		if s > 0 && isWordByte(body[s-1]) || e < len(body) && isWordByte(body[e]) {
			continue
		}
		key, num, _ := strings.Cut(body[s:e], "-")
		n, err := strconv.Atoi(num)
		if key != projectKey || err != nil || n <= 0 {
			continue
		}
		seen[n] = true
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}
