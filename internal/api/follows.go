package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchFollowedChannels returns the lower-cased login names of every channel
// followed by the authenticated user (OAuth token required). It pages through
// the Helix REST API until all follows are collected (up to 500 as a safety
// cap to avoid very large accounts causing runaway requests).
func (c *Client) FetchFollowedChannels() ([]string, error) {
	if c.oauthToken == "" {
		return nil, fmt.Errorf("oauth token required to fetch followed channels")
	}

	userID, err := c.helixCurrentUserID()
	if err != nil {
		return nil, fmt.Errorf("get current user id: %w", err)
	}

	var all []string
	cursor := ""
	for {
		batch, next, err := c.helixFollowedChannels(userID, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch follows page: %w", err)
		}
		all = append(all, batch...)
		if next == "" || len(all) >= 500 {
			break
		}
		cursor = next
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// Internal Helix helpers
// ---------------------------------------------------------------------------

func (c *Client) helixCurrentUserID() (string, error) {
	type userEntry struct {
		ID string `json:"id"`
	}
	type resp struct {
		Data []userEntry `json:"data"`
	}

	raw, err := c.helixGet("https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return "", err
	}
	var r resp
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("parse helix users: %w", err)
	}
	if len(r.Data) == 0 {
		return "", fmt.Errorf("helix returned no user data (token may be invalid)")
	}
	return r.Data[0].ID, nil
}

func (c *Client) helixFollowedChannels(userID, cursor string) (logins []string, nextCursor string, err error) {
	type channelEntry struct {
		BroadcasterLogin string `json:"broadcaster_login"`
	}
	type pagination struct {
		Cursor string `json:"cursor"`
	}
	type resp struct {
		Data       []channelEntry `json:"data"`
		Pagination pagination     `json:"pagination"`
	}

	params := map[string]string{
		"user_id": userID,
		"first":   "100",
	}
	if cursor != "" {
		params["after"] = cursor
	}

	raw, err := c.helixGet("https://api.twitch.tv/helix/channels/followed", params)
	if err != nil {
		return nil, "", err
	}
	var r resp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, "", fmt.Errorf("parse helix follows: %w", err)
	}

	for _, ch := range r.Data {
		logins = append(logins, strings.ToLower(ch.BroadcasterLogin))
	}
	return logins, r.Pagination.Cursor, nil
}

// helixGet performs an authenticated GET request to the Twitch Helix REST API.
// Helix uses "Bearer" authorization (distinct from the GQL "OAuth" scheme).
// On a 429 response the Retry-After header is honoured and the request is
// retried once; any other non-200 status is returned as an error immediately.
func (c *Client) helixGet(url string, params map[string]string) ([]byte, error) {
	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		if len(params) > 0 {
			q := req.URL.Query()
			for k, v := range params {
				q.Set(k, v)
			}
			req.URL.RawQuery = q.Encode()
		}
		req.Header.Set("Client-Id", c.clientID)
		req.Header.Set("Authorization", "Bearer "+c.oauthToken)
		req.Header.Set("Accept", "application/json")
		return req, nil
	}

	for attempt := 0; attempt < 2; attempt++ {
		req, err := buildReq()
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			// Honour Retry-After and try once more.
			time.Sleep(parseRetryAfter(resp.Header.Get("Retry-After"), 10*time.Second))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("helix %s: HTTP %d — %s", url, resp.StatusCode, body)
		}
		return body, nil
	}
	return nil, fmt.Errorf("helix %s: rate limited after retry", url)
}
