package main

import (
	"bufio"
	"context"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func checkDataChanges(ctx context.Context, dir string) {
	// Create a new filesystem watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to create watcher:", err)
	}
	defer watcher.Close()

	// Recursively watch the directory and its subdirectories
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatal("Failed to watch directory:", err)
	}

	log.Println("Started watching directory:", dir)

	// Event handling loop
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("Watcher event channel closed")
				return
			}
			// Handle new subdirectory creation
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					log.Println("New subdirectory detected:", event.Name)
					// Process the new subdirectory and update the cache
					processNewSubdirectory(event.Name)
					// Add the new subdirectory to the watcher
					watcher.Add(event.Name)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				log.Println("Watcher error channel closed")
				return
			}
			log.Println("Watcher error:", err)

		case <-ctx.Done():
			log.Println("Stopping data change checker")
			return
		}
	}
}

func processNewSubdirectory(subdir string) error {
	// Determine the next available page ID
	nextPageID, err := getNextPageID()
	if err != nil {
		return err
	}

	// Open files for appending
	cacheWriter, cacheFile, err := FileAppender(cacheFile, os.O_APPEND|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	idxWriter, idxFile, err := FileAppender(cacheIndexFile, os.O_APPEND|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer idxFile.Close()

	wordWriter, wordFile, err := FileAppender("word_postings.txt", os.O_APPEND|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer wordFile.Close()

	gemWriter, gemFile, err := FileAppender("gematria_postings.txt", os.O_APPEND|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer gemFile.Close()

	// Process the subdirectory
	pageID := nextPageID
	err = filepath.Walk(subdir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasPrefix(info.Name(), "ocr.") || !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}

		pageData, wordPostings, gemPostings, err := ProcessOCRFile(path, *cfg.String(kDir), pageID)
		if err != nil {
			return err
		}
		if pageData == nil {
			return nil // Skip if not in 'pages'
		}

		// Append to cache and index
		if err := AppendToCache(cacheWriter, idxWriter, pageData, pageID, cacheFile); err != nil {
			return err
		}

		// Write postings
		for _, posting := range wordPostings {
			_, err = wordWriter.WriteString(posting + "\n")
			if err != nil {
				return err
			}
		}
		for _, posting := range gemPostings {
			_, err = gemWriter.WriteString(posting + "\n")
			if err != nil {
				return err
			}
		}

		pageID++
		return nil
	})
	if err != nil {
		return err
	}

	// Flush all writers
	if err = cacheWriter.Flush(); err != nil {
		return err
	}
	if err = idxWriter.Flush(); err != nil {
		return err
	}
	if err = wordWriter.Flush(); err != nil {
		return err
	}
	if err = gemWriter.Flush(); err != nil {
		return err
	}

	// Rebuild the indexes
	if err = buildIndex("word_postings.txt", wordIndexFile); err != nil {
		return err
	}
	if err = buildIndex("gematria_postings.txt", gemIndexFile); err != nil {
		return err
	}

	return nil
}

// getNextPageID retrieves the next available page ID by finding the maximum ID in cache_index.txt
func getNextPageID() (int, error) {
	file, err := os.Open(cacheIndexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // If file doesn't exist, start at 0
		}
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	maxID := -1
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) < 1 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if id > maxID {
			maxID = id
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return maxID + 1, nil
}
