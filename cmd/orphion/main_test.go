package main

import (
	"errors"
	"testing"
)

func TestClassifyErrorUsage(t *testing.T) {
	tests := []struct {
		errMsg string
		want   int
	}{
		// Exit code 2: usage/config errors
		{"usage: invalid argument", 2},
		{"required flag --title", 2},
		{"not configured", 2},
		{"config: invalid yaml", 2},
		{"ffmpeg not found at \"/usr/bin/ffmpeg\"", 2},
		// Exit code 3: not found / no results
		{"no results for anime \"Frieren\"", 3},
		{"provider \"unknown\" not found", 3},
		{"ambiguous search for anime \"Naruto\"", 3},
		{"no streams for episode 1", 3},
		{"no episodes matching \"99\"", 3},
		// Exit code 1: general errors
		{"ffmpeg: exit status 1", 1},
		{"network timeout", 1},
		{"connection refused", 1},
	}
	for _, tt := range tests {
		got := classifyError(errors.New(tt.errMsg))
		if got != tt.want {
			t.Errorf("classifyError(%q) = %d, want %d", tt.errMsg, got, tt.want)
		}
	}
}

func TestClassifyErrorCanceledIsGeneral(t *testing.T) {
	// Exit code 130 is the standard code for SIGINT (128 + 2).
	// classifyError alone returns 1 for generic errors;
	// the special 130 code is only set in handleError() when ctx.Err() is set.
	err := errors.New("operation canceled")
	code := classifyError(err)
	if code != 1 {
		t.Errorf("classifyError(canceled) = %d, want 1 (general)", code)
	}
}
