package server

import (
	"testing"
	"time"
)

func TestOptionsWithDefaults(t *testing.T) {
	opts := Options{}.withDefaults()

	if got, want := opts.HeartbeatInterval, 30*time.Second; got != want {
		t.Fatalf("HeartbeatInterval = %v, want %v", got, want)
	}
	if got, want := opts.ResumeWindow, 10*time.Minute; got != want {
		t.Fatalf("ResumeWindow = %v, want %v", got, want)
	}
	if opts.Verifier != nil {
		t.Fatalf("Verifier = %#v, want nil", opts.Verifier)
	}
}
