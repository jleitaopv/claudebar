// Package usage reads a Claude subscription's Usage Windows.
//
// Everything that knows about the undocumented OAuth usage endpoint (its URL,
// headers and JSON shape) lives in this package; see ADR 0001.
package usage

import "time"

// UsageWindow is one rolling period over which the Plan caps usage.
type UsageWindow struct {
	// Utilization is the percentage of the window's limit consumed so far.
	// It can exceed 100 when extra usage is enabled.
	Utilization float64
	// ResetTime is when the window ends. The zero value means the window
	// hasn't started yet.
	ResetTime time.Time
}

// Started reports whether the window is running, i.e. has a Reset Time.
func (w UsageWindow) Started() bool { return !w.ResetTime.IsZero() }

// Reading is every Usage Window as fetched at one moment.
type Reading struct {
	SessionWindow UsageWindow
	WeeklyWindow  UsageWindow
	FetchedAt     time.Time
}
