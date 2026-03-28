package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// emoteHTTPClient is used for third-party emote API calls (BTTV, 7TV).
// Separate from the GQL client so timeouts are independent.
var emoteHTTPClient = &http.Client{Timeout: 5 * time.Second}

// FetchTwitchUserID returns the numeric Twitch user ID for the given login.
// Required by BTTV and 7TV to look up channel-specific emotes.
func (c *Client) FetchTwitchUserID(login string) (string, error) {
	const q = `query UserID($login: String!) { user(login: $login) { id } }`
	results, err := c.doGQL(Operation{
		OperationName: "UserID",
		Variables:     map[string]interface{}{"login": strings.ToLower(login)},
		Query:         q,
	})
	if err != nil || len(results) == 0 {
		return "", err
	}
	var data struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(results[0], &data); err != nil {
		return "", err
	}
	return data.User.ID, nil
}

// FetchEmoteSet fetches BTTV and 7TV emote codes for the given channel login.
// Both global emote sets and channel-specific emotes are included.  The Twitch
// numeric user ID is resolved via GQL; if that fails only global emotes are
// returned.  All per-source errors are silently swallowed so partial results
// degrade gracefully.
func (c *Client) FetchEmoteSet(channel string) map[string]struct{} {
	out := make(map[string]struct{})

	var twitchID string
	if channel != "" {
		twitchID, _ = c.FetchTwitchUserID(channel)
	}

	// BTTV global emotes
	fetchBTTVGlobal(out)
	// BTTV channel emotes
	if twitchID != "" {
		fetchBTTVChannel(twitchID, out)
	}
	// 7TV global emotes
	fetch7TVGlobal(out)
	// 7TV channel emotes
	if twitchID != "" {
		fetch7TVChannel(twitchID, out)
	}

	return out
}

// fetchEmoteJSON performs a plain GET and JSON-decodes the response body into out.
func fetchEmoteJSON(url string, out interface{}) error {
	resp, err := emoteHTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func fetchBTTVGlobal(out map[string]struct{}) {
	var emotes []struct {
		Code string `json:"code"`
	}
	if err := fetchEmoteJSON("https://api.betterttv.net/3/cached/emotes/global", &emotes); err != nil {
		return
	}
	for _, e := range emotes {
		if e.Code != "" {
			out[e.Code] = struct{}{}
		}
	}
}

func fetchBTTVChannel(twitchID string, out map[string]struct{}) {
	var data struct {
		ChannelEmotes []struct {
			Code string `json:"code"`
		} `json:"channelEmotes"`
		SharedEmotes []struct {
			Code string `json:"code"`
		} `json:"sharedEmotes"`
	}
	if err := fetchEmoteJSON("https://api.betterttv.net/3/cached/users/twitch/"+twitchID, &data); err != nil {
		return
	}
	for _, e := range data.ChannelEmotes {
		if e.Code != "" {
			out[e.Code] = struct{}{}
		}
	}
	for _, e := range data.SharedEmotes {
		if e.Code != "" {
			out[e.Code] = struct{}{}
		}
	}
}

func fetch7TVGlobal(out map[string]struct{}) {
	var data struct {
		Emotes []struct {
			Name string `json:"name"`
		} `json:"emotes"`
	}
	if err := fetchEmoteJSON("https://7tv.io/v3/emote-sets/global", &data); err != nil {
		return
	}
	for _, e := range data.Emotes {
		if e.Name != "" {
			out[e.Name] = struct{}{}
		}
	}
}

func fetch7TVChannel(twitchID string, out map[string]struct{}) {
	var data struct {
		EmoteSet struct {
			Emotes []struct {
				Name string `json:"name"`
			} `json:"emotes"`
		} `json:"emote_set"`
	}
	if err := fetchEmoteJSON("https://7tv.io/v3/users/twitch/"+twitchID, &data); err != nil {
		return
	}
	for _, e := range data.EmoteSet.Emotes {
		if e.Name != "" {
			out[e.Name] = struct{}{}
		}
	}
}
