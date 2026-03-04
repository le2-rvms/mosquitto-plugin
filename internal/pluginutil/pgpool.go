package pluginutil

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SharedPGPool 封装共享连接池状态，避免业务层直接操作锁和裸指针。
type SharedPGPool struct {
	mu   sync.RWMutex
	pool *pgxpool.Pool
}

// PGPoolOptions 描述连接池参数；插件侧可以按业务场景选择默认值。
type PGPoolOptions struct {
	MaxConns          int32
	MinConns          int32
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// newPGPool 按调用方提供的参数创建并探活 PostgreSQL 连接池。
func newPGPool(ctx context.Context, dsn string, opts PGPoolOptions) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	// 仅在调用方显式传值时覆盖 pgx 默认参数，避免工具层持有业务默认策略。
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	if opts.MinConns > 0 {
		cfg.MinConns = opts.MinConns
	}
	if opts.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	}
	if opts.HealthCheckPeriod > 0 {
		cfg.HealthCheckPeriod = opts.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Ensure 返回可复用连接池；created=true 表示本次新建成功。
func (h *SharedPGPool) Ensure(ctx context.Context, dsn string, opts PGPoolOptions) (*pgxpool.Pool, bool, error) {
	// 快路径：已有池时只读锁返回，避免不必要的互斥。
	h.mu.RLock()
	if h.pool != nil {
		p := h.pool
		h.mu.RUnlock()
		return p, false, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	// 双检锁：防止并发协程在锁升级期间重复创建连接池。
	if h.pool != nil {
		return h.pool, false, nil
	}

	p, err := newPGPool(ctx, dsn, opts)
	if err != nil {
		return nil, false, err
	}
	h.pool = p
	return p, true, nil
}

// Close 关闭并清空池；可在插件重载/退出时重复调用。
func (h *SharedPGPool) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pool != nil {
		h.pool.Close()
		h.pool = nil
	}
}
