package main

import (
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	authResultSuccess = "success"
	authResultFail    = "fail"

	authReasonOK              = "ok"
	authReasonMissingCreds    = "missing_credentials"
	authReasonUserNotFound    = "user_not_found"
	authReasonUserDisabled    = "user_disabled"
	authReasonInvalidPassword = "invalid_password"
	authReasonClientNotBound  = "client_not_bound"
	authReasonDBError         = "db_error"
	authReasonDBErrorFailOpen = "db_error_fail_open"
)

const (
	connEventTypeConnect = "connect"
)

// insertAuthEventSQL 写入认证结果事件。
const insertAuthEventSQL = `
INSERT INTO client_auth_events
  (ts, result, reason, client_id, username, peer, protocol)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

// recordConnEventSQL 写入连接事件并更新会话表。
const recordConnEventSQL = `
WITH ins AS (
  INSERT INTO client_conn_events
    (ts, event_type, client_id, username, peer, protocol, reason_code, extra)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
  RETURNING 1
)
INSERT INTO client_sessions
  (client_id, username, last_event_ts, last_event_type, last_connect_ts, last_disconnect_ts,
   last_peer, last_protocol, last_reason_code, extra)
SELECT $3, $4, $1, $2, $9, $10, $5, $6, $7, $8
FROM ins
ON CONFLICT (client_id) DO UPDATE SET
  username = EXCLUDED.username,
  last_event_ts = EXCLUDED.last_event_ts,
  last_event_type = EXCLUDED.last_event_type,
  last_connect_ts = EXCLUDED.last_connect_ts,
  last_disconnect_ts = EXCLUDED.last_disconnect_ts,
  last_peer = EXCLUDED.last_peer,
  last_protocol = EXCLUDED.last_protocol,
  last_reason_code = EXCLUDED.last_reason_code,
  extra = EXCLUDED.extra
`

// clientInfo 保存认证/连接事件中的关键信息。
type clientInfo struct {
	clientID string
	username string
	peer     string
	protocol string
}

var (
	// 连接池与配置
	pool        *pgxpool.Pool
	poolMu      sync.RWMutex
	pgDSN       string // postgres://user:pass@host:5432/db?sslmode=verify-full
	timeout     = time.Duration(1500) * time.Millisecond
	failOpen    bool
	enforceBind bool
)
