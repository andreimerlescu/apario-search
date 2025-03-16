package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust for security in production
	},
}

// WebSocket handler for /ws/search
func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		var msg struct {
			Keyword  string   `json:"keyword"`
			Channels []string `json:"channels"`
		}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		if msg.Keyword == "" || len(msg.Channels) == 0 {
			conn.WriteJSON(map[string]string{"error": "Missing keyword or channels"})
			continue
		}

		subscribeToSearch(conn, msg.Keyword, msg.Channels)
	}
}

// Subscribe client to a search
func subscribeToSearch(conn *websocket.Conn, keyword string, subChannels []string) {
	sm := searchManager

	sm.mu.Lock()
	// Check cached results first
	if cached, exists := sm.cache[keyword]; exists && time.Since(cached.Timestamp) < time.Hour && !dataChanged {
		sm.mu.Unlock()
		for _, ch := range subChannels {
			if results, ok := cached.Results[ch]; ok {
				for _, pageID := range results {
					conn.WriteJSON(map[string]interface{}{
						"channel": fmt.Sprintf("/results/%s/%s", keyword, ch),
						"pageID":  pageID,
					})
				}
			}
		}
		conn.WriteJSON(map[string]string{"status": "completed"})
		return
	}
	sm.mu.Unlock()

	// Get or create search session
	session := sm.getOrCreateSession(keyword)

	// Register client
	session.mu.Lock()
	session.Clients[conn] = subChannels
	session.mu.Unlock()

	// Stream results to this client
	for _, chName := range subChannels {
		fullChName := fmt.Sprintf("/results/%s/%s", keyword, chName)
		if ch, ok := session.Channels[chName]; ok {
			go func(ch chan string, chName string) {
				for pageID := range ch {
					conn.WriteJSON(map[string]interface{}{
						"channel": chName,
						"pageID":  pageID,
					})
				}
			}(ch, fullChName)
		}
	}

	// Notify when search completes
	<-session.Done
	conn.WriteJSON(map[string]string{"status": "completed"})
}
