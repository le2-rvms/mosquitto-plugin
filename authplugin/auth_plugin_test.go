package main

import (
	"context"
	"testing"
	"time"
)

func TestSha256PwdSalt(t *testing.T) {
	t.Parallel()
	const want = "7a37b85c8918eac19a9089c0fa5a2ab4dce3f90528dcdeec108b23ddf3607b99"
	if got := sha256PwdSalt("password", "salt"); got != want {
		t.Fatalf("sha256PwdSalt mismatch: got %q want %q", got, want)
	}
}

func TestCtxTimeout(t *testing.T) {
	ctx, cancel := timeoutContext(100 * time.Millisecond)
	defer cancel()
	if deadline, ok := ctx.Deadline(); !ok {
		t.Fatal("ctxTimeout expected deadline to be set")
	} else if remaining := time.Until(deadline); remaining < 40*time.Millisecond || remaining > 120*time.Millisecond {
		t.Fatalf("ctxTimeout deadline remaining %v outside expected range", remaining)
	}

	ctx, cancel = timeoutContext(0)
	cancel()
	if ctx != context.Background() {
		t.Fatalf("ctxTimeout with timeout<=0 should return Background context")
	}
}
