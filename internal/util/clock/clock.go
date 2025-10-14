package clock

import "time"

// Clock abstracts time source for testability.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Default is the global clock. Overwrite in tests if needed.
var Default Clock = systemClock{}

// Now returns current time from the default clock.
func Now() time.Time { return Default.Now() }

// Set replaces the default clock and returns a restore function.
func Set(c Clock) (restore func()) {
	prev := Default
	Default = c
	return func() { Default = prev }
}

// UTCNow returns the current time in UTC via the default clock.
func UTCNow() time.Time { return Now().UTC() }

// NowFormatted formats current time with the given layout using the default clock.
func NowFormatted(layout string) string { return Now().Format(layout) }

// NowUTCFormatted formats current time in UTC with the given layout.
func NowUTCFormatted(layout string) string { return UTCNow().Format(layout) }
