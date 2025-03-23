package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/RoaringBitmap/roaring"
)

func buildIndex(postingsFile, indexFile string) error {
	inFile, err := os.Open(postingsFile)
	if err != nil {
		return fmt.Errorf("open postings: %w", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer outFile.Close()
	writer := bufio.NewWriter(outFile)

	tempDir := filepath.Join(*cfg.String(kCacheDir), "temp_postings")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	keyWriters := make(map[string]*bufio.Writer)
	keyFiles := make(map[string]*os.File)
	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			continue
		}
		key, id := parts[0], parts[1]

		if _, exists := keyWriters[key]; !exists {
			f, err := os.Create(filepath.Join(tempDir, key))
			if err != nil {
				return fmt.Errorf("create temp %s: %w", key, err)
			}
			keyFiles[key] = f
			keyWriters[key] = bufio.NewWriter(f)
		}
		_, err := keyWriters[key].WriteString(id + "\n")
		if err != nil {
			return fmt.Errorf("write temp %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan postings: %w", err)
	}

	for key, w := range keyWriters {
		if err := w.Flush(); err != nil {
			return fmt.Errorf("flush temp %s: %w", key, err)
		}
		if err := keyFiles[key].Close(); err != nil {
			return fmt.Errorf("close temp %s: %w", key, err)
		}
	}

	header := make(map[string][2]int64)
	for key := range keyFiles {
		f, err := os.Open(filepath.Join(tempDir, key))
		if err != nil {
			return fmt.Errorf("reopen temp %s: %w", key, err)
		}
		defer f.Close()

		bitmap := roaring.New()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			id, _ := strconv.Atoi(scanner.Text())
			bitmap.Add(uint32(id))
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scan temp %s: %w", key, err)
		}

		offset, err := outFile.Seek(0, io.SeekCurrent)
		if err != nil {
			return fmt.Errorf("seek for %s: %w", key, err)
		}
		data, err := bitmap.MarshalBinary()
		if err != nil {
			return fmt.Errorf("marshal %s: %w", key, err)
		}
		_, err = writer.Write(data)
		if err != nil {
			return fmt.Errorf("write bitmap %s: %w", key, err)
		}
		header[key] = [2]int64{offset, int64(len(data))}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush output: %w", err)
	}
	if _, err := outFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek start: %w", err)
	}
	if err := json.NewEncoder(outFile).Encode(header); err != nil {
		return fmt.Errorf("encode header: %w", err)
	}

	return nil
}
