package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mosquitto-plugin/internal/pluginutil"
)

func timeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), timeout)
}

// ensurePool 延迟初始化连接池，必要时创建并复用。
func ensurePool(ctx context.Context) (*pgxpool.Pool, error) {
	poolMu.RLock()
	current := pool
	poolMu.RUnlock()
	if current != nil {
		return current, nil
	}

	poolMu.Lock()
	defer poolMu.Unlock()

	if pool != nil {
		return pool, nil
	}

	cfg, err := pgxpool.ParseConfig(pgDSN)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 16
	cfg.MinConns = 2
	cfg.MaxConnIdleTime = 60 * time.Second
	cfg.HealthCheckPeriod = 30 * time.Second

	newPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := newPool.Ping(ctx); err != nil {
		newPool.Close()
		return nil, err
	}
	pool = newPool
	logKV(mosqLogInfo, "auth-plugin: connected to PostgreSQL successfully")
	return pool, nil
}

// dbAuth 执行认证逻辑并返回结果/原因。
func dbAuth(username, password, clientID string) (bool, string, error) {
	if username == "" || password == "" {
		return false, authReasonMissingCreds, nil
	}
	ctx, cancel := timeoutContext(timeout)
	defer cancel()

	p, err := ensurePool(ctx)
	if err != nil {
		return false, authReasonDBError, err
	}

	var hash string
	var salt string
	var enabledInt int16
	err = p.QueryRow(ctx, selectAuthAccountSQL, username, clientID).
		Scan(&hash, &salt, &enabledInt)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, authReasonUserNotFound, nil
	}
	if err != nil {
		return false, authReasonDBError, err
	}
	if enabledInt == 0 {
		return false, authReasonUserDisabled, nil
	}
	if hash != sha256PwdSalt(password, salt) {
		return false, authReasonInvalidPassword, nil
	}

	return true, authReasonOK, nil
}

// recordAuthEvent 写入认证事件。
func recordAuthEvent(info clientInfo, result, reason string) error {
	ctx, cancel := timeoutContext(timeout)
	defer cancel()

	p, err := ensurePool(ctx)
	if err != nil {
		return err
	}

	_, err = p.Exec(ctx, insertAuthEventSQL,
		time.Now().UTC(),
		result,
		reason,
		pluginutil.OptionalString(info.clientID),
		pluginutil.OptionalString(info.username),
		pluginutil.OptionalString(info.peer),
		pluginutil.OptionalString(info.protocol),
	)
	return err
}
