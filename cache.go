package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	sema "github.com/andreimerlescu/go-sema"
)

func buildCache(dir string) (err error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Define file paths for cache and indexes.
	theCacheFilePath := filepath.Join(*cfg.String(kCacheDir), cacheFile)
	theCacheIndexFilePath := filepath.Join(*cfg.String(kCacheDir), cacheIndexFile)
	theWordPostingsFilePath := filepath.Join(*cfg.String(kCacheDir), "word_postings.txt")
	theGematriaPostingsFilePath := filepath.Join(*cfg.String(kCacheDir), "gematria_postings.txt")

	// Open files for writing (create mode).
	cacheWriter, cachedFile, err := FileAppender(theCacheFilePath, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("the FileAppender(%s) failed with: %v", theCacheFilePath, err)
	}
	defer cachedFile.Close()

	idxWriter, idxFile, err := FileAppender(theCacheIndexFilePath, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("the FileAppender(%s) failed with: %v", theCacheIndexFilePath, err)
	}
	defer idxFile.Close()

	wordWriter, wordFile, err := FileAppender(theWordPostingsFilePath, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("the FileAppender(%s) failed with: %v", theWordPostingsFilePath, err)
	}
	defer wordFile.Close()

	gemWriter, gemFile, err := FileAppender(theGematriaPostingsFilePath, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("the FileAppender(%s) failed with: %v", theGematriaPostingsFilePath, err)
	}
	defer gemFile.Close()

	// Step 1: Collect all OCR file paths to process.
	var ocrFiles []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "ocr.") && strings.HasSuffix(info.Name(), ".txt") {
			ocrFiles = append(ocrFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("filepath.Walk failed: %v", err)
	}

	// Step 2: Set up concurrency with a semaphore to limit the number of goroutines.
	workerLimit := *cfg.Int(kWorkers)
	if workerLimit == 0 || workerLimit == -1 {
		workerLimit = runtime.GOMAXPROCS(0)
	} else if workerLimit < 1 {
		workerLimit = 1
	} else if n := runtime.GOMAXPROCS(0); !*cfg.Bool(kBoost) && workerLimit > n {
		workerLimit = n
	} else if *cfg.Bool(kBoost) {
		if workerLimit < runtime.GOMAXPROCS(0) {
			workerLimit = runtime.GOMAXPROCS(0) * 2
		}
	}
	semaphore := sema.New(workerLimit)

	// Channel to collect results from goroutines.
	resultsChan := make(chan processResult, len(ocrFiles))
	var wg sync.WaitGroup

	// Step 3: Process each OCR file in a goroutine.
	pageID := 0
	for _, path := range ocrFiles {
		wg.Add(1)
		go func(path string, pageID int) {
			defer wg.Done()

			// Acquire a semaphore slot to limit concurrency.
			semaphore.Acquire()
			defer semaphore.Release()

			// Process the OCR file.
			pageData, wordPostings, gemPostings, err := ProcessOCRFile(path, pageID)
			if err != nil {
				resultsChan <- processResult{pageID: pageID, err: err}
				return
			}
			if pageData == nil {
				// Skip if not in 'pages' directory.
				resultsChan <- processResult{pageID: pageID}
				return
			}

			// Send the result to the channel.
			resultsChan <- processResult{
				pageID:       pageID,
				pageData:     pageData,
				wordPostings: wordPostings,
				gemPostings:  gemPostings,
			}
		}(path, pageID)
		pageID++
	}

	// Step 4: Close the results channel once all goroutines are done.
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Step 5: Collect results and write to cache files in order.
	// We write sequentially to avoid race conditions on file writes.
	for result := range resultsChan {
		if result.err != nil {
			return fmt.Errorf("processing page %d failed: %v", result.pageID, result.err)
		}
		if result.pageData == nil {
			continue // Skip if the file wasn’t in 'pages'.
		}

		// Append to cache and index.
		if err := AppendToCache(cacheWriter, idxWriter, result.pageData, result.pageID, cachedFile); err != nil {
			return fmt.Errorf("AppendToCache for page %d failed: %v", result.pageID, err)
		}

		// Write word postings.
		for _, posting := range result.wordPostings {
			_, err = wordWriter.WriteString(posting + "\n")
			if err != nil {
				return fmt.Errorf("writing word posting for page %d failed: %v", result.pageID, err)
			}
		}

		// Write gematria postings.
		for _, posting := range result.gemPostings {
			_, err = gemWriter.WriteString(posting + "\n")
			if err != nil {
				return fmt.Errorf("writing gematria posting for page %d failed: %v", result.pageID, err)
			}
		}
	}

	// Step 6: Flush all writers to ensure data is written to disk.
	if err = cacheWriter.Flush(); err != nil {
		return fmt.Errorf("flushing cache writer failed: %v", err)
	}
	if err = idxWriter.Flush(); err != nil {
		return fmt.Errorf("flushing index writer failed: %v", err)
	}
	if err = wordWriter.Flush(); err != nil {
		return fmt.Errorf("flushing word writer failed: %v", err)
	}
	if err = gemWriter.Flush(); err != nil {
		return fmt.Errorf("flushing gematria writer failed: %v", err)
	}

	// Step 7: Build the indexes (this part remains sequential for now).
	postingsFilePath := filepath.Join(*cfg.String(kCacheDir), "word_postings.txt")
	wordIndexFilePath := filepath.Join(*cfg.String(kCacheDir), wordIndexFile)
	if err = buildIndex(postingsFilePath, wordIndexFilePath); err != nil {
		return fmt.Errorf("building word index failed: %v", err)
	}
	gematriasFilePath := filepath.Join(*cfg.String(kCacheDir), "gematria_postings.txt")
	gemIndexFilePath := filepath.Join(*cfg.String(kCacheDir), gemIndexFile)
	if err = buildIndex(gematriasFilePath, gemIndexFilePath); err != nil {
		return fmt.Errorf("building gematria index failed: %v", err)
	}

	return nil
}
