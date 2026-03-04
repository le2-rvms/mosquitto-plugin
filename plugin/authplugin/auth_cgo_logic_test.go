package main

import (
	"testing"

	"mosquitto-plugin/internal/pluginutil"
)

func TestRunBasicAuthDefer(t *testing.T) {
	tests := []pluginutil.ClientInfo{
		{ClientID: "c1", Username: ""},
		{ClientID: "c1", Username: "_ops"},
	}
	for _, info := range tests {
		got := runBasicAuth(info, "pwd")
		if int(got) != mosqErrDefer {
			t.Fatalf("defer mismatch for %q: got=%d", info.Username, int(got))
		}
	}
}

func TestBasicAuthCbNilEventData(t *testing.T) {
	got := basic_auth_cb_c(0, nil, nil)
	want := mosqErrAuth
	if int(got) != want {
		t.Fatalf("code mismatch for nil event_data: got=%d want=%d", int(got), want)
	}
}
