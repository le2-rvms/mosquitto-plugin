package main

import "testing"

func TestSHA256PwdSalt(t *testing.T) {
	const want = "7a37b85c8918eac19a9089c0fa5a2ab4dce3f90528dcdeec108b23ddf3607b99"
	if got := sha256PwdSalt("password", "salt"); got != want {
		t.Fatalf("sha256PwdSalt mismatch: got %q want %q", got, want)
	}
}
