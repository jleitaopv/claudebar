package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// ErrNoLogin means Claude Code isn't logged in with a subscription on this
// machine: no credentials file, or one holding only an API key.
var ErrNoLogin = errors.New("no Claude subscription login found; run `claude` and log in with your Claude subscription")

// Credentials is the subset of Claude Code's stored OAuth login that
// claudebar needs. claudebar only ever reads the file (ADR 0001).
type Credentials struct {
	AccessToken string
	ExpiresAt   time.Time
	// Plan is the subscription tier, e.g. "pro" or "max".
	Plan string
}

// credentialsShape is the part of the file claudebar relies on, quoted in
// errors so a malformed file can be diagnosed.
const credentialsShape = `{"claudeAiOauth": {"accessToken": "...", "expiresAt": <epoch ms>, "subscriptionType": "..."}}`

type credentialsFile struct {
	ClaudeAiOauth *struct {
		AccessToken      string `json:"accessToken"`
		ExpiresAt        int64  `json:"expiresAt"`
		SubscriptionType string `json:"subscriptionType"`
	} `json:"claudeAiOauth"`
}

// ReadCredentials loads the Claude Code credentials file at path.
func ReadCredentials(path string) (Credentials, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Credentials{}, ErrNoLogin
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("reading credentials %s: %w", path, err)
	}
	var file credentialsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return Credentials{}, fmt.Errorf("credentials %s is not valid JSON, expected %s: %w", path, credentialsShape, err)
	}
	oauth := file.ClaudeAiOauth
	if oauth == nil {
		return Credentials{}, ErrNoLogin
	}
	if oauth.AccessToken == "" || oauth.ExpiresAt == 0 {
		return Credentials{}, fmt.Errorf("credentials %s lack accessToken or expiresAt, expected %s", path, credentialsShape)
	}
	return Credentials{
		AccessToken: oauth.AccessToken,
		ExpiresAt:   time.UnixMilli(oauth.ExpiresAt),
		Plan:        oauth.SubscriptionType,
	}, nil
}
