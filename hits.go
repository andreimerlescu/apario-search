package main

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"os"
	"sync/atomic"
	"time"
)

type Hit struct {
	Total int64
}

// Load returns the Total of Hits
func (h *Hit) Load() int64 {
	return h.Total
}

// Add increases the Total of Hits by 1
func (h *Hit) Add(i int64) int64 {
	return atomic.AddInt64(&h.Total, i)
}

// Sub decreases the Total of Hits by 1
func (h *Hit) Sub(i int64) int64 {
	return atomic.AddInt64(&h.Total, -i)
}

// Store replaces the value of Total in Hits
func (h *Hit) Store(i int64) int64 {
	atomic.StoreInt64(&h.Total, i)
	return h.Total
}

// hitJar is where the instance will keep track of hits
var hitJar *Hit

// Hits is a universally safe way of accessing the hitJar that returns *Hit
func Hits() *Hit {
	if hitJar == nil {
		hitJar = &Hit{
			Total: 0,
		}
	}
	return hitJar
}

// CurrentHits is an alias of Hit.Load
func CurrentHits() int64 {
	return Hits().Load()
}

// persistHitsOffline writes the hitJar to the file defined in kHitsStorePath
func persistHitsOffline(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 30): // Save the hits to disk every 30 seconds
			hitFile, openErr := os.OpenFile(*cfg.String(kHitsStorePath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if openErr != nil {
				log.Printf("Error opening IP ban file for writing: %v", openErr)
				return
			}
			defer hitFile.Close()

			encoder := json.NewEncoder(hitFile)
			if err := encoder.Encode(Hits()); err != nil {
				log.Printf("Error encoding Hits: %v", err)
			}

		case <-time.After(time.Minute * 3): // reload hits from disk every 3 minutes
			hitFile, openErr := os.OpenFile(*cfg.String(kHitsStorePath), os.O_RDONLY, 0600)
			if openErr != nil {
				log.Printf("Error opening Hits: %v", openErr)
				return
			}
			defer hitFile.Close()

			var results Hit
			decoder := json.NewDecoder(hitFile)
			if err := decoder.Decode(&results); err != nil {
				log.Printf("Error decoding Hits: %v", err)
				return
			}
			Hits().Store(results.Total)
		}
	}
}

// handlerAddHit uses Hits and Hit.Add to increase the hitJar
func handlerAddHit(c *gin.Context) {
	Hits().Add(1)
}
