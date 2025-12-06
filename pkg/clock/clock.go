// Package clock provides a mockable time source for testing.
package clock

import "time"

// Clock is an interface for getting the current time.
// Use RealClock in production and a mock in tests.
type Clock interface {
	Now() time.Time
}

// RealClock returns the actual system time.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// MockClock returns a fixed time for testing.
type MockClock struct {
	Time time.Time
}

// Now returns the mock time.
func (m MockClock) Now() time.Time {
	return m.Time
}
