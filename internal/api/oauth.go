package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// tokenExpiryLeeway is subtracted from expires_in so a token is refreshed
// shortly before the server would actually reject it.
const tokenExpiryLeeway = 30 * time.Second

// tokenResponse is the JSON body returned by POST /oauth2/token.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// FetchToken performs the OAuth2 client_credentials grant against
// https://{server}/oauth2/token, authenticating with HTTP Basic auth as
// documented at
// https://docs.blynk.io/en/blynk.cloud/platform-https-api/authentication.
//
// It returns the access token and the moment it should be considered
// expired (expires_in minus a small leeway).
func FetchToken(server, clientID, clientSecret string) (accessToken string, expiry time.Time, err error) {
	url := fmt.Sprintf("https://%s/oauth2/token?grant_type=client_credentials", server)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("build token request: %w", err)
	}
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("fetch token from %s: %w", server, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token request failed: %s: %s", resp.Status, string(body))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", time.Time{}, fmt.Errorf("parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("token response missing access_token: %s", string(body))
	}

	expiresIn := time.Duration(tr.ExpiresIn) * time.Second
	return tr.AccessToken, time.Now().Add(expiresIn - tokenExpiryLeeway), nil
}
