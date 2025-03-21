package main

import (
	"os"
	"path/filepath"
	"strings"
)

func buildCache(dir string) (err error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Open files for writing (create mode)
	cacheWriter, cacheFile, err := FileAppender(cacheFile, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	idxWriter, idxFile, err := FileAppender(cacheIndexFile, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer idxFile.Close()

	wordWriter, wordFile, err := FileAppender("word_postings.txt", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer wordFile.Close()

	gemWriter, gemFile, err := FileAppender("gematria_postings.txt", os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer gemFile.Close()

	pageID := 0
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasPrefix(info.Name(), "ocr.") || !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}

		pageData, wordPostings, gemPostings, err := ProcessOCRFile(path, dir, pageID)
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

	// Flush writers
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

	// Build indexes
	if err = buildIndex("word_postings.txt", wordIndexFile); err != nil {
		return err
	}
	if err = buildIndex("gematria_postings.txt", gemIndexFile); err != nil {
		return err
	}
	return nil
}
