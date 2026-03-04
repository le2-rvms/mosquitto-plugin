package pluginutil

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSharedPGPoolEnsureCreateError(t *testing.T) {
	var holder SharedPGPool
	p, created, err := holder.Ensure(context.Background(), "://bad")
	if err == nil {
		t.Fatal("SharedPGPool.Ensure should return error on invalid dsn")
	}
	if p != nil {
		t.Fatalf("pool should remain nil on error: p=%p", p)
	}
	if created {
		t.Fatal("created mismatch: got true want false")
	}
}

func TestSharedPGPoolEnsureCreatesWhenDSNReachable(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		// 本地/CI 未配置数据库时跳过，避免把单测强绑定外部依赖。
		t.Skip("set TEST_PG_DSN to run integration create-path test")
	}

	var holder SharedPGPool
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	p, created, err := holder.Ensure(ctx, dsn)
	if err != nil {
		t.Fatalf("SharedPGPool.Ensure returned error: %v", err)
	}
	if p == nil {
		t.Fatalf("pool should be created: p=%p", p)
	}
	if !created {
		t.Fatal("created mismatch: got false want true")
	}
	t.Cleanup(holder.Close)

	p2, created2, err := holder.Ensure(ctx, dsn)
	if err != nil {
		t.Fatalf("SharedPGPool.Ensure(second) returned error: %v", err)
	}
	if p2 != p {
		t.Fatalf("second call should reuse pool: got=%p want=%p", p2, p)
	}
	if created2 {
		t.Fatal("created(second) mismatch: got true want false")
	}
}
