package usage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// DefaultBaseURL is where the OAuth usage endpoint lives.
const DefaultBaseURL = "https://api.anthropic.com"

const usagePath = "/api/oauth/usage"

// ErrUnreachable means the usage endpoint couldn't be contacted at all,
// e.g. because the machine is offline.
var ErrUnreachable = errors.New("can't reach the usage endpoint")

// ErrTokenExpired means the stored access token is no longer valid. Claude
// Code refreshes it the next time it runs; claudebar never does (ADR 0001).
var ErrTokenExpired = errors.New("access token expired; run `claude` to refresh it")

// ErrUnexpectedResponse means the endpoint answered with something other than
// a usage report in the expected shape, e.g. because the API changed.
var ErrUnexpectedResponse = errors.New("unexpected response from the usage endpoint")

// ClientConfig holds a Client's dependencies.
type ClientConfig struct {
	BaseURL         string
	CredentialsPath string
	HTTPClient      *http.Client
	Now             func() time.Time
}

// Client fetches Readings from the OAuth usage endpoint.
type Client struct {
	config ClientConfig
}

// NewClient returns a Client. It does no I/O; credentials are read, and the
// endpoint contacted, on each Fetch.
func NewClient(config ClientConfig) *Client {
	return &Client{config: config}
}

// Fetch reads the current Reading. Credentials are re-read on every call so a
// token Claude Code has refreshed in the meantime is picked up.
func (c *Client) Fetch(ctx context.Context) (Reading, error) {
	creds, err := ReadCredentials(c.config.CredentialsPath)
	if err != nil {
		return Reading{}, err
	}
	if !c.config.Now().Before(creds.ExpiresAt) {
		return Reading{}, fmt.Errorf("%w (expired at %s)", ErrTokenExpired, creds.ExpiresAt.Format(time.RFC3339))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+usagePath, nil)
	if err != nil {
		return Reading{}, fmt.Errorf("building usage request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return Reading{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return Reading{}, fmt.Errorf("%w (endpoint returned HTTP 401)", ErrTokenExpired)
	}
	if resp.StatusCode != http.StatusOK {
		return Reading{}, fmt.Errorf("%w: HTTP %d, expected 200", ErrUnexpectedResponse, resp.StatusCode)
	}
	reading, err := decodeReading(resp.Body)
	if err != nil {
		return Reading{}, err
	}
	reading.FetchedAt = c.config.Now()
	return reading, nil
}
