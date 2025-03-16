package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Parse flags
	dir := flag.String("dir", ".", "Directory to scan for ocr.*.txt files")
	port := flag.String("port", "17004", "HTTP port to use 1000-65534")
	flag.Parse()

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Build or load cache
	log.Println("Initializing cache...")
	loadOrBuildCache(*dir)
	for !isCacheReady.Load() {
		log.Println("Waiting for cache to be ready...")
		time.Sleep(100 * time.Millisecond) // Poll until ready
	}
	log.Println("Cache initialized successfully")

	// Start web server with wait group for shutdown
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		webserver(ctx, *port, *dir)
	}()

	<-ctx.Done()
	wg.Wait()
	log.Println("live-writer-db has shut down")
}
