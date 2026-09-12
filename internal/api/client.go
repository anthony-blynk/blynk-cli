// Package api is the shared HTTP client for the Blynk Platform API:
// base URL resolution, bearer token injection, and error unwrapping.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is a configured connection to one Blynk server.
type Client struct {
	Server string
	Token  string
	HTTP   *http.Client
}

// NewClient builds a Client for the given server and bearer token.
func NewClient(server, token string) *Client {
	return &Client{
		Server: server,
		Token:  token,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
	}
}

// apiError carries the parsed error body from a non-2xx API response.
type apiError struct {
	StatusCode int
	Message    string
	Raw        string
}

func (e *apiError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Raw)
}

// errorBody covers the shapes the Platform API is known to return errors in.
type errorBody struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// Get issues a GET request with the given query params, decoding the JSON
// response body into out (which may be nil to discard it).
func (c *Client) Get(path string, query url.Values, out any) error {
	return c.do(http.MethodGet, path, query, nil, out)
}

// Post issues a POST request with a JSON-encoded body, decoding the JSON
// response into out (which may be nil to discard it).
func (c *Client) Post(path string, query url.Values, body, out any) error {
	return c.do(http.MethodPost, path, query, body, out)
}

// Put issues a PUT request with a JSON-encoded body.
func (c *Client) Put(path string, query url.Values, body, out any) error {
	return c.do(http.MethodPut, path, query, body, out)
}

// Delete issues a DELETE request.
func (c *Client) Delete(path string, query url.Values, out any) error {
	return c.do(http.MethodDelete, path, query, nil, out)
}

func (c *Client) do(method, path string, query url.Values, body, out any) error {
	u := url.URL{
		Scheme: "https",
		Host:   c.Server,
		Path:   path,
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &apiError{StatusCode: resp.StatusCode, Raw: string(respBody)}
		var eb errorBody
		if json.Unmarshal(respBody, &eb) == nil {
			if eb.Message != "" {
				apiErr.Message = eb.Message
			} else if eb.Error != "" {
				apiErr.Message = eb.Error
			}
		}
		return apiErr
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
	}
	return nil
}
