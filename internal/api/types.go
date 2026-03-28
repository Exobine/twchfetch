package api

import "encoding/json"

// ---------------------------------------------------------------------------
// GQL wire types
// ---------------------------------------------------------------------------

type gqlError struct {
	Message string `json:"message"`
}

type gqlEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

// Operation is a single GQL operation for batch or single requests.
type Operation struct {
	OperationName string      `json:"operationName,omitempty"`
	Variables     interface{} `json:"variables,omitempty"`
	Query         string      `json:"query,omitempty"`
	Extensions    *Extensions `json:"extensions,omitempty"`
}

type Extensions struct {
	PersistedQuery *PersistedQuery `json:"persistedQuery,omitempty"`
}

type PersistedQuery struct {
	Version    int    `json:"version"`
	SHA256Hash string `json:"sha256Hash"`
}

// ---------------------------------------------------------------------------
// Domain types returned from API helpers
// ---------------------------------------------------------------------------

// StreamerStatus is the lightweight live-check result for the main list.
type StreamerStatus struct {
	Username  string
	IsLive    bool
	UptimeSec int64  // 0 when offline
	UptimeStr string // formatted "2h 34m 12s"
	Game      string // populated by batch refresh for live streamers; backfilled on details fetch
}

// StreamDetails holds the richer data shown on the details screen.
type StreamDetails struct {
	Title        string
	Game         string
	ViewersCount int
	CreatedAt    string // raw ISO-8601
}

// VOD represents a single archived broadcast.
type VOD struct {
	ID            string
	Title         string
	CreatedAt     string // ISO-8601
	LengthSeconds int
	Game          string
}

// Chapter represents a game/category moment inside a VOD.
type Chapter struct {
	Description string
	Game        string
}
