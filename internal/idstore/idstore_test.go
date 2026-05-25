package idstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryPutIfAbsentNew(t *testing.T) {
	s := NewMemory(time.Hour)
	got, fresh, err := s.PutIfAbsent(context.Background(), Entry{Principal: "p", Key: "k", JobID: "j1"})
	if err != nil {
		t.Fatal(err)
	}
	if !fresh {
		t.Fatal("first insert must be fresh")
	}
	if got.JobID != "j1" {
		t.Fatalf("JobID = %s, want j1", got.JobID)
	}
}

func TestMemoryPutIfAbsentDuplicate(t *testing.T) {
	s := NewMemory(time.Hour)
	_, _, _ = s.PutIfAbsent(context.Background(), Entry{Principal: "p", Key: "k", JobID: "j1"})
	got, fresh, err := s.PutIfAbsent(context.Background(), Entry{Principal: "p", Key: "k", JobID: "j2"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh {
		t.Fatal("duplicate insert must report not-fresh")
	}
	if got.JobID != "j1" {
		t.Fatalf("returned entry JobID = %s, want j1", got.JobID)
	}
}

func TestMemoryGet(t *testing.T) {
	s := NewMemory(time.Hour)
	if _, ok, err := s.Get(context.Background(), "p", "missing"); err != nil || ok {
		t.Fatalf("missing key: ok=%v err=%v", ok, err)
	}
	_, _, _ = s.PutIfAbsent(context.Background(), Entry{Principal: "p", Key: "k", JobID: "j1"})
	e, ok, err := s.Get(context.Background(), "p", "k")
	if err != nil || !ok {
		t.Fatalf("present key: ok=%v err=%v", ok, err)
	}
	if e.JobID != "j1" {
		t.Fatal("returned wrong entry")
	}
}

func TestMemorySweep(t *testing.T) {
	s := NewMemory(time.Hour)
	old := time.Now().Add(-2 * time.Hour)
	_, _, _ = s.PutIfAbsent(context.Background(), Entry{Principal: "p", Key: "old", JobID: "j1", CreatedAt: old})
	_, _, _ = s.PutIfAbsent(context.Background(), Entry{Principal: "p", Key: "new", JobID: "j2"})
	n, err := s.Sweep(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("swept %d, want 1", n)
	}
	if _, ok, _ := s.Get(context.Background(), "p", "old"); ok {
		t.Fatal("old key still present after sweep")
	}
	if _, ok, _ := s.Get(context.Background(), "p", "new"); !ok {
		t.Fatal("new key gone after sweep")
	}
}

func TestMemoryRespectsContext(t *testing.T) {
	s := NewMemory(time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.PutIfAbsent(ctx, Entry{Principal: "p", Key: "k", JobID: "j"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if _, _, err := s.Get(ctx, "p", "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get want context.Canceled, got %v", err)
	}
	if _, err := s.Sweep(ctx, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sweep want context.Canceled, got %v", err)
	}
}

func TestNewMemoryDefaultTTL(t *testing.T) {
	s := NewMemory(0)
	if s.ttl != 24*time.Hour {
		t.Fatalf("default ttl = %v, want 24h", s.ttl)
	}
}
