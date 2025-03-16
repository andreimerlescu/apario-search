package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"log"
)

// Start the server
func webserver(ctx context.Context) {
	go checkDataChanges(ctx, *dir) // Start data change checker
	r := gin.Default()
	r.GET("/search", handleSearch)       // Existing endpoint
	r.GET("/ws/search", handleWebSocket) // New WebSocket endpoint
	if err := r.Run(":17004"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
