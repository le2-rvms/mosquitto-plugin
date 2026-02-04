package pluginutil

import (
	"strings"
	"testing"
	"time"
)

func TestParseBoolOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		want   bool
		wantOK bool
	}{
		{"true lowercase", "true", true, true},
		{"true numeric", "1", true, true},
		{"false uppercase", "FALSE", false, true},
		{"trim whitespace", "  yes  ", true, true},
		{"invalid", "maybe", false, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseBoolOption(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("ParseBoolOption(%q) = (%v, %v), want (%v, %v)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseTimeoutMS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		want   time.Duration
		wantOK bool
	}{
		{"1000", 1000 * time.Millisecond, true},
		{" 250 ", 250 * time.Millisecond, true},
		{"0", 0, false},
		{"-10", 0, false},
		{"abc", 0, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseTimeoutMS(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("ParseTimeoutMS(%q) = (%v, %v), want (%v, %v)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestSafeDSN(t *testing.T) {
	t.Parallel()
	input := "postgres://user:secret@example.com/db"
	got := SafeDSN(input)
	if strings.Contains(got, "secret") {
		t.Fatalf("SafeDSN leaked password: %q", got)
	}
	if !strings.Contains(got, "xxxxx") {
		t.Fatalf("SafeDSN did not mask password: %q", got)
	}

	noPass := "postgres://user@example.com/db"
	if gotNoPass := SafeDSN(noPass); gotNoPass != noPass {
		t.Fatalf("SafeDSN changed DSN without password: %q", gotNoPass)
	}

	raw := "%%%"
	if gotRaw := SafeDSN(raw); gotRaw != raw {
		t.Fatalf("SafeDSN should return original string when parsing fails")
	}
}

func TestOptionalString(t *testing.T) {
	t.Parallel()
	if got := OptionalString(""); got != nil {
		t.Fatalf("OptionalString(\"\") = %#v, want nil", got)
	}
	if got := OptionalString("x"); got != "x" {
		t.Fatalf("OptionalString(\"x\") = %#v, want \"x\"", got)
	}
}

func TestProtocolString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want string
	}{
		{3, "MQTT/3.1"},
		{4, "MQTT/3.1.1"},
		{5, "MQTT/5.0"},
		{0, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := ProtocolString(tc.in)
			if got != tc.want {
				t.Fatalf("ProtocolString(%d) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}
