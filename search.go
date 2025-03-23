package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/andreimerlescu/gematria"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing query"})
		return
	}

	sortParam := c.Query("sort")
	rank := sortParam == "ranked"

	results, err := search(query)
	if err != nil {
		errorLogger.Printf("Search error for query %q: %v", query, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal server error",
			"message": "Check the server logs to see what happened.",
		})
		return
	}

	if rank {
		// Return ranked results with scores and match details
		type rankedPage struct {
			ID      string        `json:"id"`
			Score   int           `json:"score"`
			Matches []MatchDetail `json:"matches"`
		}
		var ranked []rankedPage
		for pageID, count := range results.HitCounts {
			ranked = append(ranked, rankedPage{
				ID:      pageID,
				Score:   count,
				Matches: results.Matches[pageID],
			})
		}
		// Sort by score descending, then ID ascending for stability
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].Score == ranked[j].Score {
				return ranked[i].ID < ranked[j].ID
			}
			return ranked[i].Score > ranked[j].Score
		})
		c.JSON(http.StatusOK, ranked)
	} else {
		// Default: flat list for backward compatibility
		seen := make(map[string]struct{})
		var flatResults []string
		for _, pages := range results.Categories {
			for _, pageID := range pages {
				if _, exists := seen[pageID]; !exists {
					seen[pageID] = struct{}{}
					flatResults = append(flatResults, pageID)
				}
			}
		}
		c.JSON(http.StatusOK, flatResults)
	}
}

func search(query string) (SearchResults, error) {
	// Analyze the query
	analysis := AnalyzeQuery(query)
	resultBitmap := roaring.New()

	// Open the word index file
	wordIndex, err := os.Open(filepath.Join(*cfg.String(kCacheDir), wordIndexFile))
	if err != nil {
		return SearchResults{}, fmt.Errorf("failed to open word index file: %w", err)
	}
	defer wordIndex.Close()

	// Decode the word header
	var wordHeader map[string][2]int64
	if err := json.NewDecoder(wordIndex).Decode(&wordHeader); err != nil {
		return SearchResults{}, fmt.Errorf("failed to decode word header: %w", err)
	}

	// Process AND conditions
	for _, andCond := range analysis.Ands {
		words := strings.Fields(andCond)
		temp := roaring.New()
		for _, word := range words {
			if offsetLen, ok := wordHeader[word]; ok {
				b := roaring.New()
				data := make([]byte, offsetLen[1])
				if _, err := wordIndex.Seek(offsetLen[0], io.SeekStart); err != nil {
					return SearchResults{}, fmt.Errorf("failed to seek in word index: %w", err)
				}
				if _, err := wordIndex.Read(data); err != nil {
					return SearchResults{}, fmt.Errorf("failed to read from word index: %w", err)
				}
				if err := b.UnmarshalBinary(data); err != nil {
					return SearchResults{}, fmt.Errorf("failed to unmarshal bitmap: %w", err)
				}
				temp.Or(b)
			}
		}
		if resultBitmap.IsEmpty() {
			resultBitmap = temp
		} else {
			resultBitmap.And(temp)
		}
	}

	// Process NOT conditions
	for _, notCond := range analysis.Nots {
		words := strings.Fields(notCond)
		temp := roaring.New()
		for _, word := range words {
			if offsetLen, ok := wordHeader[word]; ok {
				b := roaring.New()
				data := make([]byte, offsetLen[1])
				if _, err := wordIndex.Seek(offsetLen[0], io.SeekStart); err != nil {
					return SearchResults{}, fmt.Errorf("failed to seek in word index for NOT condition: %w", err)
				}
				if _, err := wordIndex.Read(data); err != nil {
					return SearchResults{}, fmt.Errorf("failed to read from word index for NOT condition: %w", err)
				}
				if err := b.UnmarshalBinary(data); err != nil {
					return SearchResults{}, fmt.Errorf("failed to unmarshal bitmap for NOT condition: %w", err)
				}
				temp.Or(b)
			}
		}
		resultBitmap.AndNot(temp)
	}

	// Open cache files
	cache, err := os.Open(filepath.Join(*cfg.String(kCacheDir), cacheFile))
	if err != nil {
		return SearchResults{}, fmt.Errorf("failed to open cache file: %w", err)
	}
	defer cache.Close()

	cacheIdx, err := os.Open(filepath.Join(*cfg.String(kCacheDir), cacheIndexFile))
	if err != nil {
		return SearchResults{}, fmt.Errorf("failed to open cache index file: %w", err)
	}
	defer cacheIdx.Close()

	// Initialize results
	results := SearchResults{
		Categories: make(map[string][]string),
		HitCounts:  make(map[string]int),
		Matches:    make(map[string][]MatchDetail),
	}

	// Build page ID to offset mapping
	scanner := bufio.NewScanner(cacheIdx)
	idToOffset := make(map[int][2]int64)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) != 3 {
			continue // Skip malformed lines
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			return SearchResults{}, fmt.Errorf("failed to parse page ID: %w", err)
		}
		offset, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return SearchResults{}, fmt.Errorf("failed to parse offset: %w", err)
		}
		length, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return SearchResults{}, fmt.Errorf("failed to parse length: %w", err)
		}
		idToOffset[id] = [2]int64{offset, length}
	}
	if err := scanner.Err(); err != nil {
		return SearchResults{}, fmt.Errorf("error reading cache index: %w", err)
	}

	// Process matching pages
	itr := resultBitmap.Iterator()
	for itr.HasNext() {
		pageID := int(itr.Next())
		offsetLen, ok := idToOffset[pageID]
		if !ok {
			continue // Page ID not found in index
		}
		data := make([]byte, offsetLen[1])
		if _, err := cache.Seek(offsetLen[0], io.SeekStart); err != nil {
			return SearchResults{}, fmt.Errorf("failed to seek in cache file: %w", err)
		}
		if _, err := cache.Read(data); err != nil {
			return SearchResults{}, fmt.Errorf("failed to read from cache file: %w", err)
		}
		var page PageData
		if err := json.Unmarshal(data, &page); err != nil {
			return SearchResults{}, fmt.Errorf("failed to unmarshal page data: %w", err)
		}
		// Track which categories this page matches
		categoryMatched := make(map[string]bool)

		// Extract query terms from AND conditions
		for _, andCond := range analysis.Ands {
			words := strings.Fields(andCond)
			for _, word := range words {
				queryGematria := gematria.FromString(word)

				// Check exact match
				if matchesExactTextee(word, page.Textee) {
					categoryMatched["exact/textee"] = true
				}

				// Check fuzzy matches using existing functions
				for _, algo := range []string{"jaro", "jaro-winkler", "soundex", "hamming", "ukkonen", "wagner-fisher"} {
					category := "fuzzy/" + algo
					for pw := range page.Textee.Gematrias {
						if matchesConditionSingle(word, pw, algo) {
							categoryMatched[category] = true
							break // Found a match for this algo, move to next
						}
					}
				}

				// Check gematria matches using existing functions
				gematriaTypes := []string{"simple", "english", "jewish", "eights", "mystery", "majestic"}
				for _, gemType := range gematriaTypes {
					category := "gematria/" + gemType
					for _, pg := range page.Textee.Gematrias {
						if matchesConditionGematria(pg, queryGematria) {
							categoryMatched[category] = true
							break // Found a match for this gematria type
						}
					}
				}
			}
		}

		// Populate results for matched categories
		for category := range categoryMatched {
			results.Categories[category] = append(results.Categories[category], page.PageIdentifier)
			results.HitCounts[page.PageIdentifier]++
		}
	}

	return results, nil
}
