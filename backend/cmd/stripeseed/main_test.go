package main

import "testing"

func TestRequireTestKey(t *testing.T) {
	for key, ok := range map[string]bool{
		"sk_test_abc": true,
		"rk_test_abc": true,
		"sk_live_abc": false, // catalog not final (B02)
		"rk_live_abc": false,
		"pk_test_abc": false,
		"":            false,
	} {
		if err := requireTestKey(key); (err == nil) != ok {
			t.Errorf("requireTestKey(%q) = %v, want ok=%v", key, err, ok)
		}
	}
}
