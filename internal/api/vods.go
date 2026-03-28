package api

import (
	"encoding/json"
	"strings"
)

// vodQuery fetches archived VODs. No cursor pagination — Twitch's anonymous
// GQL endpoint does not reliably support after-cursor for the videos field.
// Pagination is handled client-side by escalating the `first` count.
const vodQuery = `query UserVODs($login: String!, $first: Int!) {
  user(login: $login) {
    videos(first: $first, sort: TIME, type: ARCHIVE) {
      edges {
        node {
          id title createdAt lengthSeconds
          game { displayName }
        }
      }
    }
  }
}`

// FetchVODs retrieves the most recent `count` archived VODs for a channel.
func (c *Client) FetchVODs(username string, count int) ([]VOD, error) {
	results, err := c.doGQL(Operation{
		OperationName: "UserVODs",
		Variables: map[string]interface{}{
			"login": strings.ToLower(username),
			"first": count,
		},
		Query: vodQuery,
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	var data struct {
		User struct {
			Videos struct {
				Edges []struct {
					Node struct {
						ID            string `json:"id"`
						Title         string `json:"title"`
						CreatedAt     string `json:"createdAt"`
						LengthSeconds int    `json:"lengthSeconds"`
						Game          struct {
							DisplayName string `json:"displayName"`
						} `json:"game"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"videos"`
		} `json:"user"`
	}
	if err := json.Unmarshal(results[0], &data); err != nil {
		return nil, err
	}

	vods := make([]VOD, 0, len(data.User.Videos.Edges))
	for _, e := range data.User.Videos.Edges {
		n := e.Node
		vods = append(vods, VOD{
			ID:            n.ID,
			Title:         n.Title,
			CreatedAt:     n.CreatedAt,
			LengthSeconds: n.LengthSeconds,
			Game:          n.Game.DisplayName,
		})
	}
	return vods, nil
}

// FetchVODChapters retrieves game chapter moments for a slice of VOD IDs.
// Returns a map from VOD ID to its chapter list.
func (c *Client) FetchVODChapters(vodIDs []string, chapterHash string) (map[string][]Chapter, error) {
	if len(vodIDs) == 0 {
		return map[string][]Chapter{}, nil
	}

	ops := make([]Operation, len(vodIDs))
	for i, id := range vodIDs {
		ops[i] = Operation{
			OperationName: "VideoPlayer_ChapterSelectButtonVideo",
			Variables:     map[string]string{"videoID": id},
			Extensions: &Extensions{
				PersistedQuery: &PersistedQuery{
					Version:    1,
					SHA256Hash: chapterHash,
				},
			},
		}
	}

	results, err := c.doGQL(ops)
	if err != nil {
		// Return empty map on error — chapters are optional
		return map[string][]Chapter{}, nil
	}

	out := make(map[string][]Chapter, len(vodIDs))
	for i, id := range vodIDs {
		if i >= len(results) {
			break
		}
		var data struct {
			Video struct {
				Moments struct {
					Edges []struct {
						Node struct {
							Description string `json:"description"`
							Game        struct {
								DisplayName string `json:"displayName"`
							} `json:"game"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"moments"`
			} `json:"video"`
		}
		if err := json.Unmarshal(results[i], &data); err != nil {
			continue
		}
		chapters := make([]Chapter, 0, len(data.Video.Moments.Edges))
		for _, e := range data.Video.Moments.Edges {
			chapters = append(chapters, Chapter{
				Description: e.Node.Description,
				Game:        e.Node.Game.DisplayName,
			})
		}
		out[id] = chapters
	}
	return out, nil
}
