package main

import (
	"context"
	"log"
	"time"
)

func checkDataChanges(ctx context.Context, dir string) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping data change checker")
			return
		case <-ticker.C:
			// Replace with your actual recursive scan logic
			// For now, simulate a check
			log.Println("Checking for data changes in", dir)
			newDataChanged := false // Set to true if data changed (e.g., via filepath.Walk)

			if newDataChanged != dataChanged {
				dataChanged = newDataChanged
				if dataChanged {
					sm := searchManager
					sm.mu.Lock()
					sm.cache = make(map[string]*SearchResult) // Invalidate cache
					sm.mu.Unlock()
					log.Println("Data changed, cache invalidated")
				}
			}
		}
	}
}
