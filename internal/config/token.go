package config

import (
	"fmt"
	"time"

	"github.com/anthony-blynk/blynk-cli/internal/api"
)

// EnsureToken returns a valid bearer token for the resolved profile,
// fetching (and caching) a fresh one via OAuth2 client-credentials if
// needed. Static-token profiles just return their token as-is.
//
// When profileName is non-empty, a refreshed token is written back into
// cfg and persisted to disk so subsequent commands reuse it.
func EnsureToken(cfg *Config, profileName string, p *Profile) (string, error) {
	if p.IsStaticToken() {
		return p.Token, nil
	}

	if p.CachedToken != "" && time.Now().Before(p.TokenExpiry) {
		return p.CachedToken, nil
	}

	token, expiry, err := api.FetchToken(p.Server, p.ClientID, p.ClientSecret)
	if err != nil {
		return "", fmt.Errorf("fetch access token: %w", err)
	}

	p.CachedToken = token
	p.TokenExpiry = expiry

	if profileName != "" && cfg != nil {
		cfg.Set(profileName, *p)
		if err := cfg.Save(); err != nil {
			return "", fmt.Errorf("cache token: %w", err)
		}
	}

	return token, nil
}
