package clock

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRealClockNow(t *testing.T) {
	r := Real()
	first := r.Now()
	time.Sleep(2 * time.Millisecond)
	if !r.Now().After(first) {
		t.Fatal("real clock did not advance")
	}
}

func TestMockAfterFuncFires(t *testing.T) {
	m := NewMock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	var fired atomic.Int32
	m.AfterFunc(10*time.Second, func() { fired.Add(1) })
	m.Advance(5 * time.Second)
	if fired.Load() != 0 {
		t.Fatal("timer fired before deadline")
	}
	m.Advance(10 * time.Second)
	if fired.Load() != 1 {
		t.Fatalf("timer fired %d times, want 1", fired.Load())
	}
}

func TestMockStop(t *testing.T) {
	m := NewMock(time.Now())
	var fired atomic.Int32
	tm := m.AfterFunc(5*time.Second, func() { fired.Add(1) })
	if !tm.Stop() {
		t.Fatal("Stop() returned false for active timer")
	}
	m.Advance(10 * time.Second)
	if fired.Load() != 0 {
		t.Fatal("stopped timer fired")
	}
	if tm.Stop() {
		t.Fatal("Stop on already-stopped timer should return false")
	}
}

func TestMockReset(t *testing.T) {
	m := NewMock(time.Now())
	var fired atomic.Int32
	tm := m.AfterFunc(5*time.Second, func() { fired.Add(1) })
	tm.Stop()
	tm.Reset(2 * time.Second)
	m.Advance(3 * time.Second)
	if fired.Load() != 1 {
		t.Fatalf("reset timer did not fire after Advance; fired=%d", fired.Load())
	}
}
