package main

import (
	"sync"
	"time"

	"github.com/andreimerlescu/gematria"
	"github.com/andreimerlescu/textee"
	"github.com/gorilla/websocket"
)

type PageData struct {
	Textee              *textee.Textee
	PageIdentifier      string
	DocumentIdentifier  string
	CoverPageIdentifier string
}

type SearchAnalysis struct {
	Ors  map[uint]string
	Ands []string
	Nots []string
}

type SearchSession struct {
	mu       sync.Mutex
	Keyword  string
	Channels map[string]chan string       // e.g., "exact/textee" -> channel
	Clients  map[*websocket.Conn][]string // WebSocket conn -> subscribed channels
	Done     chan struct{}                // Signals search completion
	Results  map[string][]string          // Accumulates results for caching
}

type SearchResult struct {
	Results   map[string][]string // Channel -> pageIDs
	Timestamp time.Time
}

// SearchManager manages ongoing searches and cached results
type SearchManager struct {
	activeSearches map[string]*SearchSession
	cache          map[string]*SearchResult
	mu             sync.Mutex
}

// MatchDetail captures the specifics of a match
type MatchDetail struct {
	Text       string            `json:"text"`     // The matched word from Textee.Gematrias
	Gematria   gematria.Gematria `json:"gematria"` // The Gematria struct that matched
	TexTeeTexT string            `json:"original"` // The full Textee.Input for context
	Category   string            `json:"category"` // e.g., "exact/textee", "gematria/simple"
}

// SearchResults holds categorized results, hit counts, and match details
type SearchResults struct {
	Categories map[string][]string      // e.g., "exact/textee" -> page IDs
	HitCounts  map[string]int           // page ID -> total hits across categories
	Matches    map[string][]MatchDetail // page ID -> list of match details
}
