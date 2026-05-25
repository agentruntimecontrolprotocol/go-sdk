package client

import (
	"testing"
	"time"

	arcp "github.com/agentruntimecontrolprotocol/go-sdk"
)

func TestOptionsWithDefaults(t *testing.T) {
	o := Options{}.withDefaults()
	if o.ClientName != "arcp-go-client" {
		t.Fatalf("ClientName = %s, want arcp-go-client", o.ClientName)
	}
	if o.ClientVersion != arcp.SDKVersion {
		t.Fatalf("ClientVersion = %s, want %s", o.ClientVersion, arcp.SDKVersion)
	}
	if o.Logger == nil {
		t.Fatal("Logger must default to non-nil")
	}
	if len(o.Features) == 0 {
		t.Fatal("Features must default to the SDK list")
	}
	if o.AutoAckInterval == 0 {
		t.Fatal("AutoAckInterval must default")
	}
}

func TestOptionsKeepsUserOverrides(t *testing.T) {
	o := Options{
		ClientName:      "custom",
		ClientVersion:   "9.9.9",
		Features:        []string{"x"},
		AutoAckInterval: time.Minute,
	}.withDefaults()
	if o.ClientName != "custom" {
		t.Fatal("override lost")
	}
	if o.ClientVersion != "9.9.9" {
		t.Fatal("version override lost")
	}
	if len(o.Features) != 1 || o.Features[0] != "x" {
		t.Fatalf("Features = %v, want [x]", o.Features)
	}
	if o.AutoAckInterval != time.Minute {
		t.Fatal("AutoAckInterval override lost")
	}
}
