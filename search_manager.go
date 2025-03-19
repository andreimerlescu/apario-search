package main

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

func (sm *SearchManager) runSearch(session *SearchSession) {
	defer func() {
		sm.mu.Lock()
		delete(sm.activeSearches, session.Keyword)
		sm.mu.Unlock()

		for _, ch := range session.Channels {
			close(ch)
		}
		close(session.Done)
		sm.cacheResults(session)
	}()

	results, err := search(session.Keyword)
	if err != nil {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	for category, pageIDs := range results.Categories {
		if ch, ok := session.Channels[category]; ok {
			for _, pageID := range pageIDs {
				// Optionally send full match details via WebSocket
				for _, match := range results.Matches[pageID] {
					if match.Category == category {
						ch <- pageID // Simple version: just page ID
						// TODO: Add option to send MatchDetail if needed
					}
				}
			}
		}
		session.Results[category] = pageIDs
	}
}

// Get or create a search session
func (sm *SearchManager) getOrCreateSession(keyword string) *SearchSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.activeSearches[keyword]; exists {
		return session
	}

	session := &SearchSession{
		Keyword:  keyword,
		Channels: make(map[string]chan string),
		Clients:  make(map[*websocket.Conn][]string),
		Done:     make(chan struct{}),
		Results:  make(map[string][]string),
		mu:       sync.Mutex{},
	}

	// Define supported channels
	channels := []string{
		"exact/textee",
		"fuzzy/jaro",
		"fuzzy/jaro-winkler",
		"fuzzy/soundex",
		"fuzzy/hamming",
		"fuzzy/wagner-fischer",
		"gematria/simple",
		"gematria/english",
		"gematria/jewish",
		"gematria/majestic",
		"gematria/mystery",
		"gematria/eights",
	}
	for _, ch := range channels {
		session.Channels[ch] = make(chan string, 100) // Buffered to prevent blocking
	}

	sm.activeSearches[keyword] = session
	go sm.runSearch(session)
	return session
}

// Cache completed search results
func (sm *SearchManager) cacheResults(session *SearchSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.cache[session.Keyword] = &SearchResult{
		Results:   session.Results,
		Timestamp: time.Now(),
	}
}
