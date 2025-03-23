package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	err := loadConfigs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

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

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Build or load cache
		log.Println("Initializing cache...")
		err = buildCache(*cfg.String(kDir))
		if err != nil {
			log.Fatal(err)
		}
		for !isCacheReady.Load() {
			log.Println("Waiting for cache to be ready...")
			time.Sleep(100 * time.Millisecond) // Poll until ready
		}
		log.Println("Cache initialized successfully")
	}()

	// Start web server with wait group for shutdown
	wg.Add(1)
	log.Printf("apario-search has started for %s with ", *cfg.String(kReaderDomain))
	go func() {
		defer wg.Done()
		webserver(ctx, *cfg.String(kPort), *cfg.String(kDir))
	}()

	<-ctx.Done()
	wg.Wait()
	log.Println("apario-search has shut down")
}
