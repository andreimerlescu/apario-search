package main

import (
	"context"
	"flag"
	"github.com/andreimerlescu/go-smartchan"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		cancel()
	}()

	// Start web server immediately
	go webserver(ctx)

	// Build or load cache
	loadOrBuildCache(*dir)

	log.Println("live-writer-db is fully operational")
}

func search(query string) []string {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	analysis := AnalyzeQuery(query)
	results := make(map[string]struct{})

	ctx := context.Background()
	sch := go_smartchan.NewSmartChan(1000)
	var wg sync.WaitGroup

	// Process the entire analysis in one go
	wg.Add(1)
	go func() {
		defer wg.Done()
		findPagesForWord(ctx, sch, analysis)
	}()

	go func() {
		wg.Wait()
		sch.Close()
	}()

	// Collect results
	for data := range sch.Chan() {
		if pageID, ok := data.(string); ok {
			results[pageID] = struct{}{}
		}
	}

	// Convert to slice
	var finalResults []string
	for pageID := range results {
		finalResults = append(finalResults, pageID)
	}

	return finalResults
}
