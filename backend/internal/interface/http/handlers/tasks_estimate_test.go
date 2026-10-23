package handlers

import (
	"encoding/json"
	"testing"
)

func TestUpdateTaskReq_Estimate(t *testing.T) {
	for body, want := range map[string]struct {
		set   bool
		value *int
	}{
		`{}`:                {false, nil},
		`{"estimate":null}`: {true, nil},
		`{"estimate":8}`:    {true, ptrInt(8)},
	} {
		var req updateTaskReq
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		got := req.Estimate
		if got.Set != want.set || (got.Value == nil) != (want.value == nil) || (got.Value != nil && *got.Value != *want.value) {
			t.Errorf("%s: got %+v, want set=%v value=%v", body, got, want.set, want.value)
		}
	}
	var req updateTaskReq
	if err := json.Unmarshal([]byte(`{"estimate":2.5}`), &req); err == nil {
		t.Error("fractional estimate: want a decode error")
	}
}

func ptrInt(n int) *int { return &n }
