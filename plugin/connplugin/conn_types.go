package main

import (
	"time"

	"mosquitto-plugin/internal/pluginutil"
)

const recordEventSQL = `
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
  last_connect_ts = COALESCE(EXCLUDED.last_connect_ts, client_sessions.last_connect_ts),
  last_disconnect_ts = EXCLUDED.last_disconnect_ts,
  last_peer = EXCLUDED.last_peer,
  last_protocol = EXCLUDED.last_protocol,
  last_reason_code = EXCLUDED.last_reason_code,
  extra = EXCLUDED.extra
`

const (
	connEventTypeConnect    = "connect"
	connEventTypeDisconnect = "disconnect"

	defaultTimeout   = 1000 * time.Millisecond
	debugSampleEvery = uint64(128)
)

var (
	// poolHolder 统一管理连接事件插件的 PostgreSQL 连接池。
	poolHolder pluginutil.SharedPGPool
	// connPGPoolOptions 定义连接事件插件默认连接池参数。
	connPGPoolOptions = pluginutil.PGPoolOptions{
		MaxConns:          16,
		MinConns:          2,
		MaxConnIdleTime:   60 * time.Second,
		HealthCheckPeriod: 30 * time.Second,
	}
	pgDSN      string
	timeout    = defaultTimeout

	activeConnections = newConnTracker()

	debugSkipCounter   uint64
	debugRecordCounter uint64
)
