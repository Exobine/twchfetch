package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const liveStatusQuery = `query VideoPlayerStatusOverlayChannel($channel: String!) {
  user(login: $channel) {
    stream { type createdAt game { displayName } }
  }
}`

const liveDetailsQuery = `query VideoPlayerStatusOverlayChannel($channel: String!) {
  user(login: $channel) {
    followers {
      totalCount
    }
    stream {
      title
      game { displayName }
      createdAt
      viewersCount
    }
  }
}`

// FetchLiveStatusBatch fetches the live status for a slice of usernames in one
// batched GQL request. Returns one StreamerStatus per username.
func (c *Client) FetchLiveStatusBatch(usernames []string) ([]StreamerStatus, error) {
	ops := make([]Operation, len(usernames))
	for i, u := range usernames {
		ops[i] = Operation{
			OperationName: "VideoPlayerStatusOverlayChannel",
			Variables:     map[string]string{"channel": strings.ToLower(u)},
			Query:         liveStatusQuery,
		}
	}

	results, err := c.doGQL(ops)
	if err != nil {
		// Return all-offline on error rather than propagating
		out := make([]StreamerStatus, len(usernames))
		for i, u := range usernames {
			out[i] = StreamerStatus{Username: u}
		}
		return out, nil
	}

	out := make([]StreamerStatus, len(usernames))
	for i, u := range usernames {
		out[i] = StreamerStatus{Username: u}
		if i >= len(results) {
			continue
		}
		var data struct {
			User struct {
				Stream *struct {
					Type      string `json:"type"`
					CreatedAt string `json:"createdAt"`
					Game      struct {
						DisplayName string `json:"displayName"`
					} `json:"game"`
				} `json:"stream"`
			} `json:"user"`
		}
		if err := json.Unmarshal(results[i], &data); err != nil {
			continue
		}
		if data.User.Stream != nil && data.User.Stream.Type == "live" {
			out[i].IsLive = true
			out[i].Game = data.User.Stream.Game.DisplayName
			if sec, str, ok := parseUptime(data.User.Stream.CreatedAt); ok {
				out[i].UptimeSec = sec
				out[i].UptimeStr = str
			}
		}
	}
	return out, nil
}

// FetchLiveDetails fetches the detailed stream info for a single channel.
// Returns (details, followers, err): details is nil when the channel is offline;
// followers is the channel's follower count (0 when unavailable).
func (c *Client) FetchLiveDetails(username string) (*StreamDetails, int, error) {
	ops := []Operation{{
		OperationName: "VideoPlayerStatusOverlayChannel",
		Variables:     map[string]string{"channel": strings.ToLower(username)},
		Query:         liveDetailsQuery,
	}}
	results, err := c.doGQL(ops)
	if err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return nil, 0, fmt.Errorf("no results for %s", username)
	}

	var data struct {
		User struct {
			Followers struct {
				TotalCount int `json:"totalCount"`
			} `json:"followers"`
			Stream *struct {
				Title        string `json:"title"`
				Game         struct{ DisplayName string `json:"displayName"` } `json:"game"`
				CreatedAt    string `json:"createdAt"`
				ViewersCount int    `json:"viewersCount"`
			} `json:"stream"`
		} `json:"user"`
	}
	if err := json.Unmarshal(results[0], &data); err != nil {
		return nil, 0, err
	}
	followers := data.User.Followers.TotalCount
	if data.User.Stream == nil {
		return nil, followers, nil // offline — not an error
	}
	s := data.User.Stream
	return &StreamDetails{
		Title:        s.Title,
		Game:         s.Game.DisplayName,
		ViewersCount: s.ViewersCount,
		CreatedAt:    s.CreatedAt,
	}, followers, nil
}

// parseUptime parses an ISO-8601 createdAt string and returns uptime seconds,
// a human-readable uptime string, and whether parsing succeeded.
func parseUptime(createdAt string) (int64, string, bool) {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return 0, "", false
	}
	dur := time.Since(t)
	sec := int64(dur.Seconds())
	h := int(dur.Hours())
	m := int(dur.Minutes()) % 60
	s := int(dur.Seconds()) % 60
	str := fmt.Sprintf("%dh %02dm %02ds", h, m, s)
	return sec, str, true
}
