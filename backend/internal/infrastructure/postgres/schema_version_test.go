package postgres

import "testing"

func TestCheckSchemaVersion(t *testing.T) {
	cases := []struct {
		name    string
		version int64
		dirty   bool
		want    uint
		ok      bool
	}{
		{"current", 32, false, 32, true},
		{"ahead of the binary (next release's expand migration)", 33, false, 32, true},
		{"behind the binary", 31, false, 32, false},
		{"dirty at the wanted version", 32, true, 32, false},
		{"nil version row", -1, false, 32, false},
	}
	for _, c := range cases {
		if err := checkSchemaVersion(c.version, c.dirty, c.want); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}
