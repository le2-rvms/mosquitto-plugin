package main

import "testing"

func TestParseBoolOption(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   bool
		wantOK bool
	}{
		{name: "true lowercase", input: "true", want: true, wantOK: true},
		{name: "true numeric", input: "1", want: true, wantOK: true},
		{name: "false uppercase", input: "FALSE", want: false, wantOK: true},
		{name: "trim whitespace", input: "  yes  ", want: true, wantOK: true},
		{name: "invalid", input: "maybe", want: false, wantOK: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseBoolOption(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("parseBoolOption(%q) = (%v, %v), want (%v, %v)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
