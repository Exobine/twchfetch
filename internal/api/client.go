package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const gqlEndpoint = "https://gql.twitch.tv/gql"

// rateLimitError is returned by doRequest when the server responds 429.
// RetryAfter is the duration to wait before the next attempt, parsed from the
// Retry-After response header when present, otherwise a conservative default.
type rateLimitError struct {
	RetryAfter time.Duration
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("rate limited: retry after %s", e.RetryAfter)
}

// parseRetryAfter converts a Retry-After header value to a duration.
// Accepts both integer seconds ("30") and HTTP-date formats.
// Falls back to fallback when the header is absent or unparseable.
func parseRetryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}
	if n, err := strconv.Atoi(header); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return fallback
}

// Client wraps the HTTP client and Twitch credentials.
type Client struct {
	http       *http.Client
	clientID   string
	oauthToken string // optional; adds Authorization header when set
}

// NewClient creates a Client with the given credentials and request timeout.
func NewClient(clientID, oauthToken string, timeoutSec int) *Client {
	return &Client{
		http:       &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		clientID:   clientID,
		oauthToken: oauthToken,
	}
}

// HasAuth reports whether an OAuth token is configured.
func (c *Client) HasAuth() bool {
	return c.oauthToken != ""
}

// doGQL sends one or many GQL operations and returns the raw JSON data field
// for each operation in order.  Retries up to 3 times with exponential backoff
// and jitter.  On a 429 response the Retry-After header is honoured exactly;
// on other errors the sleep is 2^attempt seconds ± up to 50 % jitter.
func (c *Client) doGQL(payload interface{}) ([]json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal gql payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			var rl *rateLimitError
			if errors.As(lastErr, &rl) {
				// Server told us exactly how long to wait — respect it.
				time.Sleep(rl.RetryAfter)
			} else {
				// Exponential backoff: 1 s, 2 s … with up to 50 % random jitter
				// to desynchronise concurrent callers hitting the same error.
				base := time.Duration(1<<uint(attempt-1)) * time.Second
				jitter := time.Duration(rand.Int63n(int64(base / 2)))
				time.Sleep(base + jitter)
			}
		}
		results, err := c.doRequest(body)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Client) doRequest(body []byte) ([]json.RawMessage, error) {
	req, err := http.NewRequest("POST", gqlEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Client-ID", c.clientID)
	req.Header.Set("Content-Type", "application/json")
	if c.oauthToken != "" {
		req.Header.Set("Authorization", "OAuth "+c.oauthToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &rateLimitError{
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), 10*time.Second),
		}
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gql http %d: %s", resp.StatusCode, raw)
	}

	// Twitch returns an array for batch requests, object for single.
	if len(raw) > 0 && raw[0] == '[' {
		var envelopes []gqlEnvelope
		if err := json.Unmarshal(raw, &envelopes); err != nil {
			return nil, err
		}
		out := make([]json.RawMessage, len(envelopes))
		for i, e := range envelopes {
			if len(e.Errors) > 0 {
				return nil, fmt.Errorf("gql error[%d]: %s", i, e.Errors[0].Message)
			}
			out[i] = e.Data
		}
		return out, nil
	}

	var envelope gqlEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Errors) > 0 {
		return nil, fmt.Errorf("gql error: %s", envelope.Errors[0].Message)
	}
	return []json.RawMessage{envelope.Data}, nil
}
