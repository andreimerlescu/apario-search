package main

import (
	"encoding/json"
	"github.com/andreimerlescu/textee"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func loadOrBuildCache(dir string) {
	if _, err := os.Stat(cacheFile); err == nil {
		log.Println("Loading cache from file...")
		if err := loadCacheFromFile(); err != nil {
			log.Printf("Failed to load cache: %v, rebuilding...", err)
			buildCache(dir)
		}
	} else {
		log.Println("No cache file found, building cache...")
		buildCache(dir)
	}
	isCacheReady.Store(true)
}

func buildCache(dir string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing %s: %v", path, err)
			return err // Propagate directory access errors
		}
		if info.IsDir() {
			return nil // Skip directories
		}
		if !strings.HasPrefix(info.Name(), "ocr.") || !strings.HasSuffix(info.Name(), ".txt") {
			return nil // Skip non-OCR text files
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			log.Printf("Error computing relative path for %s: %v", path, err)
			return nil // Skip if relative path fails
		}
		if !strings.HasPrefix(relPath, string(filepath.Separator)+"pages") && !strings.HasPrefix(relPath, "pages") {
			return nil // Skip files not in pages/ subdirectories
		}

		// Define identifiers
		pageIdentifier := relPath
		docDir := filepath.Dir(filepath.Dir(relPath))
		documentIdentifier := docDir
		coverPageIdentifier := filepath.Join(docDir, "pages", "page.000001.json")

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Error reading file %s: %v", path, err)
			return nil // Skip if file can't be read
		}

		// Create Textee instance
		text, textErr := textee.NewTextee(string(content))
		if textErr != nil {
			log.Printf("Skipping %s: failed to create Textee: %v", path, textErr)
			return nil // Skip this file but continue processing others
		}

		// Store in cache
		pageCache[pageIdentifier] = &PageData{
			Textee:              text,                // Use the successfully created Textee
			PageIdentifier:      pageIdentifier,      // Full relative path (e.g., "fc91a290.../pages/ocr.000001.txt")
			DocumentIdentifier:  documentIdentifier,  // Document directory (e.g., "fc91a290...")
			CoverPageIdentifier: coverPageIdentifier, // Assumed cover file (e.g., "fc91a290.../pages/page.000001.json")
		}

		// Update document cache if this is the first page for the document
		if _, exists := documentCache[documentIdentifier]; !exists {
			documentCache[documentIdentifier] = coverPageIdentifier
		}

		return nil // Continue walking
	})
	if err != nil {
		log.Printf("Error walking directory %s: %v", dir, err)
	}

	saveCacheToFile() // Persist the cache
}

type CacheData struct {
	Pages     map[string]*PageData
	Documents map[string]string
}

func saveCacheToFile() {
	cacheData := CacheData{
		Pages:     pageCache,
		Documents: documentCache,
	}
	data, err := json.MarshalIndent(cacheData, "", "  ")
	if err != nil {
		log.Printf("Error marshaling cache: %v", err)
		return
	}
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		log.Printf("Error writing cache file: %v", err)
	}
}

func loadCacheFromFile() error {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return err
	}
	cacheData := CacheData{}
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return err
	}
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	pageCache = cacheData.Pages
	documentCache = cacheData.Documents
	return nil
}
