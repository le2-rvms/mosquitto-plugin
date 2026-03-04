package main

import (
	"context"
	"time"

	"mosquitto-plugin/internal/pluginutil"
)

func recordEvent(info pluginutil.ClientInfo, eventType string, reasonCode *int) error {
	ctx := context.Background()
	cancel := func() {}
	// timeout<=0 时保持无超时行为，便于排障时临时放开超时限制。
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()

	p, created, err := poolHolder.Ensure(ctx, pgDSN, connPGPoolOptions)
	if err != nil {
		return err
	}
	if created {
		log(mosqLogInfo, "conn-plugin: postgres pool connected", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN)})
	}

	ts := time.Now().UTC()
	connectTS, disconnectTS := any(nil), any(nil)
	if eventType == connEventTypeConnect {
		connectTS = ts
	} else {
		disconnectTS = ts
	}
	// connect 事件没有 reason code，disconnect 则写入具体错误码。
	var reasonArg any
	if reasonCode != nil {
		reasonArg = *reasonCode
	}

	_, err = p.Exec(ctx, recordEventSQL,
		ts,
		eventType,
		info.ClientID,
		pluginutil.OptionalString(info.Username),
		pluginutil.OptionalString(info.Peer),
		pluginutil.OptionalString(info.Protocol),
		reasonArg,
		nil,
		connectTS,
		disconnectTS,
	)
	if err != nil {
		return err
	}
	if pluginutil.ShouldSample(&debugRecordCounter, debugSampleEvery) {
		log(mosqLogDebug, "conn-plugin: recorded event", map[string]any{"event": eventType, "client_id": info.ClientID, "username": info.Username, "peer": info.Peer, "protocol": info.Protocol, "reason_code": reasonCode})
	}
	return nil
}
