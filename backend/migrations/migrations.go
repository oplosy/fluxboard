// Package migrations embeds the up migrations so a binary knows which schema
// version it was built against (/readyz, docs/08-API-SPEC.md). golang-migrate
// and sqlc only read the .sql files in this directory.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed *.up.sql
var files embed.FS

// Latest returns the highest migration version shipped with this build: the
// numeric prefix of the newest NNNN_name.up.sql file.
func Latest() (uint, error) { return latest(files) }

func latest(fsys fs.FS) (uint, error) {
	names, err := fs.Glob(fsys, "*.up.sql")
	if err != nil {
		return 0, err
	}
	var max uint
	for _, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return 0, fmt.Errorf("migrations: %s has no NNNN_ version prefix", name)
		}
		v, err := strconv.ParseUint(prefix, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("migrations: %s: version prefix: %w", name, err)
		}
		if uint(v) > max {
			max = uint(v)
		}
	}
	if max == 0 {
		return 0, fmt.Errorf("migrations: no up migrations embedded")
	}
	return max, nil
}
