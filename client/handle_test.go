package client

import (
	"encoding/base64"
	"testing"
)

func TestAssembleChunksUTF8(t *testing.T) {
	by := map[string]*chunkAccum{
		"r1": {
			encoding: "utf8",
			chunks:   map[uint64]string{0: "hello ", 1: "world"},
		},
	}
	out, err := assembleChunks(by)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello world" {
		t.Fatalf("got %q", out)
	}
}

func TestAssembleChunksBase64(t *testing.T) {
	c0 := base64.StdEncoding.EncodeToString([]byte("foo"))
	c1 := base64.StdEncoding.EncodeToString([]byte("bar"))
	by := map[string]*chunkAccum{
		"r1": {encoding: "base64", chunks: map[uint64]string{1: c1, 0: c0}},
	}
	out, err := assembleChunks(by)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "foobar" {
		t.Fatalf("got %q", out)
	}
}

func TestAssembleChunksMultipleResultIDsRejected(t *testing.T) {
	by := map[string]*chunkAccum{
		"r1": {encoding: "utf8", chunks: map[uint64]string{0: "x"}},
		"r2": {encoding: "utf8", chunks: map[uint64]string{0: "y"}},
	}
	if _, err := assembleChunks(by); err == nil {
		t.Fatal("expected error for multiple result_ids")
	}
}

func TestAssembleChunksEmptyOK(t *testing.T) {
	out, err := assembleChunks(map[string]*chunkAccum{})
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("got %v, want nil", out)
	}
}
