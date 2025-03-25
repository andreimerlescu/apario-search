package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// loadSearchData loads the word index header and cache index mappings into memory,
// and keeps the word_index.bin and apario-search-cache.jsonl files open for search operations.
func loadSearchData() error {
	// Load word index
	wordIndexHandle, err := os.Open(filepath.Join(*cfigs.String(kCacheDir), wordIndexFile))
	if err != nil {
		return fmt.Errorf("failed to open word index file: %w", err)
	}

	var headerOffset uint64
	err = binary.Read(wordIndexHandle, binary.LittleEndian, &headerOffset)
	if err != nil {
		return fmt.Errorf("failed to read word header offset: %w", err)
	}

	_, err = wordIndexHandle.Seek(int64(headerOffset), io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to word header offset: %w", err)
	}

	wordIndexHeader = make(map[string][2]int64)
	if err := json.NewDecoder(wordIndexHandle).Decode(&wordIndexHeader); err != nil {
		return fmt.Errorf("failed to decode word header: %w", err)
	}
	if len(wordIndexHeader) == 0 {
		return fmt.Errorf("word index header is empty")
	}
	log.Printf("Loaded word index header with %d entries", len(wordIndexHeader))

	// Load gematria index
	gemIndexHandle, err = os.Open(filepath.Join(*cfigs.String(kCacheDir), gemIndexFile))
	if err != nil {
		return fmt.Errorf("failed to open gematria index file: %w", err)
	}

	err = binary.Read(gemIndexHandle, binary.LittleEndian, &headerOffset)
	if err != nil {
		return fmt.Errorf("failed to read gematria header offset: %w", err)
	}

	_, err = gemIndexHandle.Seek(int64(headerOffset), io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to gematria header offset: %w", err)
	}

	wordIndexGematrias = make(map[string][2]int64)
	if err := json.NewDecoder(gemIndexHandle).Decode(&wordIndexGematrias); err != nil {
		return fmt.Errorf("failed to decode gematria header: %w", err)
	}
	if len(wordIndexGematrias) == 0 {
		return fmt.Errorf("gematria index header is empty")
	}
	log.Printf("Loaded gematria index header with %d entries", len(wordIndexGematrias))

	// Load cache index
	cacheIdx, err := os.Open(filepath.Join(*cfigs.String(kCacheDir), cacheIndexFile))
	if err != nil {
		return fmt.Errorf("failed to open cache index file: %w", err)
	}
	defer cacheIdx.Close()

	cacheIdToOffset = make(map[int][2]int64)
	scanner := bufio.NewScanner(cacheIdx)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) != 3 {
			continue
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("failed to parse page ID: %w", err)
		}
		offset, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse offset: %w", err)
		}
		length, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse length: %w", err)
		}
		cacheIdToOffset[id] = [2]int64{offset, length}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading cache index: %w", err)
	}
	if len(cacheIdToOffset) == 0 {
		return fmt.Errorf("cache index is empty")
	}
	log.Printf("Loaded cache index with %d entries", len(cacheIdToOffset))

	// Open cache file
	cacheFileHandle, err = os.Open(filepath.Join(*cfigs.String(kCacheDir), cacheFile))
	if err != nil {
		return fmt.Errorf("failed to open cache file: %w", err)
	}

	return nil
}
