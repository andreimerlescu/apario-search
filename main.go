package main

import (
	"context"
	"fmt"
	"github.com/andreimerlescu/sema"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

const Currency = "APARIO"
const TrustLine = "rU16Gt85z6ZM84vTgb7D82QueJ26HvhTz2"

func main() {
	err := loadConfigs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cacheMutex = sync.RWMutex{}

	// Set up logging to error.log
	logFile, err := os.OpenFile(*cfg.String(kErrorLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open error.log: %v", err)
	}
	defer logFile.Close()
	errorLogger = log.New(logFile, "", log.LstdFlags)

	// Handle signals for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	systemSearchSemaphore = sema.New(*cfg.Int(kMaxSearches))

	// Check cache integrity
	cacheFiles := []string{cacheFile, cacheIndexFile, wordIndexFile, gemIndexFile}
	cacheValid := true
	for _, file := range cacheFiles {
		filePath := filepath.Join(*cfg.String(kCacheDir), file)
		checksumPath := filePath + ".sha256"
		if !verifyChecksum(filePath, checksumPath) {
			cacheValid = false
			break
		}
	}

	// Initialize cache
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Checking cache...")
		if cacheValid {
			log.Println("Cache files validated, loading search data...")
			if err := loadSearchData(); err != nil {
				errorLogger.Printf("Failed to load search data: %v", err)
				cancel()
				return
			}
			log.Println("Search data loaded successfully")
		} else {
			log.Println("Cache invalid or missing, rebuilding...")
			if err := buildCache(*cfg.String(kDir)); err != nil {
				errorLogger.Printf("Cache initialization failed: %v", err)
				cancel()
				return
			}
			log.Println("Cache initialized successfully")
			// Generate checksums after build
			for _, file := range cacheFiles {
				filePath := filepath.Join(*cfg.String(kCacheDir), file)
				if err := generateChecksum(filePath); err != nil {
					errorLogger.Printf("Failed to generate checksum for %s: %v", file, err)
				}
			}
			if err := loadSearchData(); err != nil {
				errorLogger.Printf("Failed to load search data: %v", err)
				cancel()
				return
			}
			log.Println("Search data loaded successfully")
		}
	}()

	// Start web server
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("apario-search has started for %s", *cfg.String(kReaderDomain))
		webserver(ctx, *cfg.String(kPort), *cfg.String(kDir))
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Printf("Received %v signal, initiating shutdown...", sig)
		cancel() // Cancel the context to signal goroutines to stop

		if wordIndexHandle != nil {
			_ = wordIndexHandle.Close()
		}
		if gemIndexHandle != nil {
			_ = gemIndexHandle.Close()
		}
		if cacheFileHandle != nil {
			_ = cacheFileHandle.Close()
		}
	}

	// Wait for all goroutines to complete with a timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("apario-search has shut down gracefully")
	case <-sigChan: // Second signal received
		log.Println("Forcing immediate shutdown")
		os.Exit(1)
	case <-ctx.Done():
		// Context was cancelled from within (e.g., cache build failed)
		wg.Wait()
		log.Println("apario-search has shut down due to internal cancellation")
	}

	// Final cleanup message
	log.Println("Shutdown complete")

}
