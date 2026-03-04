package main

import "testing"

func resetConnState() {
	activeConnections.Reset()
	debugSkipCounter = 0
	debugRecordCounter = 0
}

func TestConsumeConnectedClearsState(t *testing.T) {
	resetConnState()
	key := uintptr(12345)
	activeConnections.MarkConnected(key)

	if !activeConnections.ConsumeConnected(key) {
		t.Fatal("ConsumeConnected should return true for connected key")
	}
	if activeConnections.ConsumeConnected(key) {
		t.Fatal("connection state should be cleared after first ConsumeConnected")
	}
}

func TestConsumeConnectedSkipWhenNotConnected(t *testing.T) {
	resetConnState()
	key := uintptr(999)

	if activeConnections.ConsumeConnected(key) {
		t.Fatal("ConsumeConnected should return false for non-connected key")
	}
}

func TestConsumeConnectedIdempotent(t *testing.T) {
	resetConnState()
	key := uintptr(123)
	activeConnections.MarkConnected(key)

	if !activeConnections.ConsumeConnected(key) {
		t.Fatal("first ConsumeConnected should return true")
	}
	if activeConnections.ConsumeConnected(key) {
		t.Fatal("second ConsumeConnected should return false")
	}
}
