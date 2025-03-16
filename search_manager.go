package main

import (
	"golang.org/x/net/websocket"
	"sync"
	"time"
)

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

// Run the search and stream results
func (sm *SearchManager) runSearch(session *SearchSession) {
	defer func() {
		// Clean up
		sm.mu.Lock()
		delete(sm.activeSearches, session.Keyword)
		sm.mu.Unlock()

		for _, ch := range session.Channels {
			close(ch)
		}
		close(session.Done)
		sm.cacheResults(session)
	}()

	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	// Simulate search (replace with your actual search logic)
	for pageID, data := range pageCache {
		// Example exact match
		if data.Textee.Input == session.Keyword {
			session.Channels["exact/textee"] <- pageID
			session.Results["exact/textee"] = append(session.Results["exact/textee"], pageID)
		}
		// Example Gematria match (simplified)
		if data.Textee.Input == session.Keyword+" the wise" { // Placeholder logic
			session.Channels["gematria/simple"] <- pageID
			session.Results["gematria/simple"] = append(session.Results["gematria/simple"], pageID)
		}
		// Add your fuzzy and other Gematria logic here
	}
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
