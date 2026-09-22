package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thin HTTP client for the Glyph REST API. All calls target the
// /api/v1 group except OAuth token issuance (/oauth/token) and the unauthenticated
// health check (/health), which have their own helpers below.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func newClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// apiError carries the server's RFC-style JSON error payload so the CLI can
// surface a useful message rather than a raw status code.
type apiError struct {
	Status int
	Code   string
	Msg    string
}

func (e *apiError) Error() string {
	switch {
	case e.Code != "" && e.Msg != "":
		return fmt.Sprintf("server returned %d: %s: %s", e.Status, e.Code, e.Msg)
	case e.Msg != "":
		return fmt.Sprintf("server returned %d: %s", e.Status, e.Msg)
	case e.Code != "":
		return fmt.Sprintf("server returned %d: %s", e.Status, e.Code)
	default:
		return fmt.Sprintf("server returned %d", e.Status)
	}
}

// doJSON performs a JSON request against a /api/v1 path. body may be nil for
// requests without a payload. If out is non-nil the response body is decoded
// into it. path must start with "/".
func (c *Client) doJSON(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, c.BaseURL+"/api/v1"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Bearer tokens are exempt from the API's CSRF check, but sending
	// X-Requested-With as well keeps the CLI working against a dev-mode server
	// that authenticates via the dev cookie instead of a token.
	req.Header.Set("X-Requested-With", "glyphctl")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	return c.send(req, out)
}

// postForm posts application/x-www-form-urlencoded data to a path outside the
// /api/v1 group (used for the OAuth token endpoint).
func (c *Client) postForm(path string, form url.Values, out any) error {
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return c.send(req, out)
}

// getRaw fetches a path relative to the server root (not /api/v1) and returns
// the raw body. Used for the health check.
func (c *Client) getRaw(path string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

func (c *Client) send(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, data)
	}

	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func parseAPIError(status int, data []byte) error {
	e := &apiError{Status: status}
	var payload struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(data, &payload) == nil {
		e.Code = payload.Error
		e.Msg = payload.Description
	}
	if e.Code == "" && e.Msg == "" {
		e.Msg = strings.TrimSpace(string(data))
	}
	return e
}
