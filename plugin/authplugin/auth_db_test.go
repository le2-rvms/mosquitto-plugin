package main

import (
	"testing"
)

func TestDBAuthMissingCreds(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "missing username", password: "pwd"},
		{name: "missing password", username: "alice"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			allow, reason, err := dbAuth(tc.username, tc.password, "c1")
			if err != nil {
				t.Fatalf("dbAuth returned unexpected error: %v", err)
			}
			if allow {
				t.Fatal("dbAuth should deny when credentials are missing")
			}
			if reason != authReasonMissingCreds {
				t.Fatalf("reason mismatch: got=%q want=%q", reason, authReasonMissingCreds)
			}
		})
	}
}
