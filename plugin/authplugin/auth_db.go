package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"mosquitto-plugin/internal/pluginutil"
)

// dbAuth 执行认证逻辑并返回结果/原因。
func dbAuth(username, password, clientID string) (bool, string, error) {
	if username == "" || password == "" {
		return false, authReasonMissingCreds, nil
	}
	ctx := context.Background()
	cancel := func() {}
	// 保留 timeout<=0 时“无超时”的旧行为。
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()

	p, created, err := poolHolder.Ensure(ctx, pgDSN, authPGPoolOptions)
	if err != nil {
		return false, "", err
	}
	if created {
		log(mosqLogInfo, "auth-plugin: postgres pool connected", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN)})
	}
	var passwordHash string
	var salt string
	var enabled int16
	err = p.QueryRow(ctx, selectAuthAccountSQL, username, clientID).Scan(&passwordHash, &salt, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, authReasonUserNotFound, nil
	}
	if err != nil {
		// DB 运行时错误交给上层 runBasicAuth 统一处理 fail_open。
		return false, "", err
	}
	if enabled == 0 {
		return false, authReasonUserDisabled, nil
	}
	if passwordHash != sha256PwdSalt(password, salt) {
		return false, authReasonInvalidPassword, nil
	}

	return true, authReasonOK, nil
}

// recordAuthEvent 写入认证事件。
func recordAuthEvent(info pluginutil.ClientInfo, result, reason string) error {
	ctx := context.Background()
	cancel := func() {}
	// 保留 timeout<=0 时“无超时”的旧行为。
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()

	p, created, err := poolHolder.Ensure(ctx, pgDSN, authPGPoolOptions)
	if err != nil {
		return err
	}
	if created {
		log(mosqLogInfo, "auth-plugin: postgres pool connected", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN)})
	}
	_, err = p.Exec(ctx, insertAuthEventSQL,
		time.Now().UTC(),
		result,
		reason,
		pluginutil.OptionalString(info.ClientID),
		pluginutil.OptionalString(info.Username),
		pluginutil.OptionalString(info.Peer),
		pluginutil.OptionalString(info.Protocol),
	)
	return err
}
