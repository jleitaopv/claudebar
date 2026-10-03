package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// windowResponse is one Usage Window as the endpoint reports it. Pointers
// distinguish a missing or null field from a zero value.
type windowResponse struct {
	Utilization *float64   `json:"utilization"`
	ResetsAt    *time.Time `json:"resets_at"`
}

// decodeReading parses the endpoint's response body. Only "five_hour" (the
// Session Window) and "seven_day" (the Weekly Window) are read; every other
// field is ignored. A window key that is absent means the response shape has
// changed, while an explicit null means the window hasn't started.
func decodeReading(body io.Reader) (Reading, error) {
	var fields map[string]json.RawMessage
	if err := json.NewDecoder(body).Decode(&fields); err != nil {
		return Reading{}, fmt.Errorf("%w: body is not a JSON object: %w", ErrUnexpectedResponse, err)
	}
	session, err := decodeWindow(fields, "five_hour")
	if err != nil {
		return Reading{}, err
	}
	weekly, err := decodeWindow(fields, "seven_day")
	if err != nil {
		return Reading{}, err
	}
	return Reading{SessionWindow: session, WeeklyWindow: weekly}, nil
}

func decodeWindow(fields map[string]json.RawMessage, key string) (UsageWindow, error) {
	raw, ok := fields[key]
	if !ok {
		return UsageWindow{}, fmt.Errorf("%w: missing %q, expected an object with utilization and resets_at", ErrUnexpectedResponse, key)
	}
	var w *windowResponse
	if err := json.Unmarshal(raw, &w); err != nil {
		return UsageWindow{}, fmt.Errorf("%w: %q is %s, expected an object with utilization and resets_at: %w", ErrUnexpectedResponse, key, raw, err)
	}
	if w == nil {
		return UsageWindow{}, nil
	}
	if w.Utilization == nil {
		return UsageWindow{}, fmt.Errorf("%w: %q has no utilization, got %s", ErrUnexpectedResponse, key, raw)
	}
	window := UsageWindow{Utilization: *w.Utilization}
	if w.ResetsAt != nil {
		window.ResetTime = *w.ResetsAt
	}
	return window, nil
}
