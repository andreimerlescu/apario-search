package main

import (
	"github.com/andreimerlescu/textee"
	"golang.org/x/net/websocket"
	"sync"
	"time"
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
