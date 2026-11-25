package policy

import (
	"context"
	"testing"
)

// TestDeployPolicy runs the kind policy in gate/deploy/policy for each rule.
func TestDeployPolicy(t *testing.T) {
	o, err := Load(context.Background(), "../../deploy/policy")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		in   Input
		want Outcome
	}{
		{"echo", Input{Tool: "echo", Arguments: map[string]any{"text": "hi"}}, Allow},
		{"delete prod", Input{Tool: "delete_thing", Arguments: map[string]any{"id": "prod"}}, Deny},
		{"delete other", Input{Tool: "delete_thing", Arguments: map[string]any{"id": "dev"}}, Approve},
		{"delete no arguments", Input{Tool: "delete_thing"}, Approve},
		{"other tool", Input{Tool: "drop_table"}, Deny},
	} {
		if got := o.Decide(context.Background(), tc.in); got.Outcome != tc.want {
			t.Errorf("%s: got %+v, want %s", tc.name, got, tc.want)
		}
	}
}
