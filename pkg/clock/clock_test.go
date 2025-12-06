package clock

import (
	"testing"
	"time"
)

func TestRealClock_Now(t *testing.T) {
	c := RealClock{}
	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("RealClock.Now() returned time outside expected range")
	}
}

func TestMockClock_Now(t *testing.T) {
	fixedTime := time.Date(2025, 12, 6, 9, 0, 0, 0, time.UTC)
	c := MockClock{Time: fixedTime}

	got := c.Now()
	if !got.Equal(fixedTime) {
		t.Errorf("MockClock.Now() = %v, want %v", got, fixedTime)
	}
}
