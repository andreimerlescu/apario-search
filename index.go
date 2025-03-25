package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/RoaringBitmap/roaring"
)

// buildIndex constructs an inverted index from a postings file (e.g., word_postings.txt or gematria_postings.txt)
// and writes it to an index file (e.g., word_index.bin or gematria_index.bin).
// The postings file contains lines in the format "key pageID" (e.g., "secret 123").
// The index file has:
//   - A JSON header mapping each key to a [offset, length] pair, where offset is the byte position of the key’s bitmap in the file.
//   - A binary body containing Roaring Bitmaps, where each bitmap lists the page IDs associated with a key.
//
// This function uses temporary files to group page IDs by key before building the bitmaps, with a semaphore to limit open files.
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

	// Reserve 8 bytes for header offset
	_, err = outFile.Write(make([]byte, 8))
	if err != nil {
		return fmt.Errorf("reserve header offset: %w", err)
	}

	// Build bitmaps
	keyToBitmap := make(map[string]*roaring.Bitmap)
	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Fields(line) // Split by any whitespace
		if len(parts) < 2 {
			continue // Skip invalid lines
		}
		pageIDStr := parts[len(parts)-1] // Last part is the page ID
		keyParts := parts[:len(parts)-1] // Everything before is the key
		key := strings.Join(keyParts, " ")
		pageID, err := strconv.Atoi(pageIDStr)
		if err != nil {
			continue // Skip if page ID isn’t an integer
		}
		if _, exists := keyToBitmap[key]; !exists {
			keyToBitmap[key] = roaring.New()
		}
		keyToBitmap[key].Add(uint32(pageID))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan postings: %w", err)
	}

	// Write bitmaps and track offsets
	header := make(map[string][2]int64)
	bitmapStart, _ := outFile.Seek(0, io.SeekCurrent)
	currentOffset := bitmapStart
	for key, bitmap := range keyToBitmap {
		data, err := bitmap.MarshalBinary()
		if err != nil {
			return fmt.Errorf("marshal bitmap %s: %w", key, err)
		}
		n, err := outFile.Write(data)
		if err != nil {
			return fmt.Errorf("write bitmap %s: %w", key, err)
		}
		header[key] = [2]int64{currentOffset, int64(n)}
		currentOffset += int64(n)
	}

	// Write header at the end
	headerOffset, _ := outFile.Seek(0, io.SeekCurrent)
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}
	_, err = outFile.Write(headerJSON)
	if err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write header offset at the start
	_, err = outFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek start: %w", err)
	}
	err = binary.Write(outFile, binary.LittleEndian, uint64(headerOffset))
	if err != nil {
		return fmt.Errorf("write header offset: %w", err)
	}

	return nil
}

func buildIndexUnlimited(postingsFile, indexFile string) error {
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
