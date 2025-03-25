package main

import (
	"encoding/json"
	"github.com/andreimerlescu/sema"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/andreimerlescu/gematria"
	"github.com/gin-gonic/gin"
)

func handleSearch(c *gin.Context) {
	ip := FilteredIP(c)
	// kPerIPSearchLimit restricts FilteredIP through a semaphore, so in order to get the
	// semaphore for the FilteredIP, we need to perform this series of reader lock/unlocks
	// and relevant writer lock/unlocks while getting the semaphore and acquire a lock on it
	searchSemaphoresLock.RLock()                     // lock the sema reader
	sem, ok := searchSemaphores[ip].(sema.Semaphore) // perform the read on the sema
	if !ok || sem == nil {                           // perform the logic on the sema
		searchSemaphoresLock.RUnlock()                               // unlock the sema reader
		searchSemaphoresLock.Lock()                                  // lock the sema writer
		searchSemaphores[ip] = sema.New(*cfg.Int(kPerIPSearchLimit)) // create new semaphore
		searchSemaphoresLock.Unlock()                                // unlock the sema writer
	} else { // we are ok and we have a semaphore for the ip in question
		searchSemaphoresLock.RUnlock() // unlock the sema reader
	}

	searchSemaphoresLock.RLock()   // lock the sema reader
	searchSemaphores[ip].Acquire() // acquire a lock for the ip
	searchSemaphoresLock.RUnlock() // unlock the sema reader
	defer func() { // when results delivered to user
		searchSemaphoresLock.RLock()   // lock the sema reader
		searchSemaphores[ip].Release() // release the lock for the ip
		searchSemaphoresLock.RUnlock() // unlock the sema reader
	}()
	query := c.Query("q")
	if len(query) == 0 {
		query = c.Query("query")
		if len(query) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing query"})
			return
		}
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
	// the system has a limit on the number of concurrent searches that can be performed
	// across the entire appliance regardless of the status of the searchSemaphores map[ip]sema
	// that was released allowing them to search... the system needs to release a spot before
	// a new search can be performed since this is a resource intensive process. This semaphore
	// allows you to install the search application on a small virtual machine and serve 140GB
	// of assets in a cached search that is blazing fast, like 30ms response times!
	systemSearchSemaphore.Acquire()
	defer systemSearchSemaphore.Release()

	// Start timing the search for performance logging
	startTime := time.Now()

	// Analyze the query
	analysis := AnalyzeQuery(query)
	resultBitmap := roaring.New()
	resultMutex := sync.RWMutex{}
	fuzzyAlgos := []string{"jaro", "jaro-winkler", "soundex", "hamming", "ukkonen", "wagner-fisher"}
	gematriaTypes := []string{"simple", "english", "jewish", "eights", "mystery", "majestic"}

	// concurrently process the AND and NOT conditions
	wg := sync.WaitGroup{}
	wg.Add(2)

	// Process AND conditions using the in-memory word index header
	go func() {
		defer wg.Done()
		for _, andCond := range analysis.Ands {
			temp := roaring.New()
			// Handle OR groups or single terms
			var words []string
			if strings.HasPrefix(andCond, "(") && strings.HasSuffix(andCond, ")") {
				words = strings.Split(strings.Trim(andCond, "()"), " or ")
				for i, w := range words {
					words[i] = strings.TrimSpace(w)
				}
			} else {
				words = []string{andCond}
			}

			for _, word := range words {
				// Exact match
				if offsetLen, ok := wordIndexHeader[word]; ok {
					if offsetLen[0] >= 0 && offsetLen[1] > 0 {
						b := roaring.New()
						data := make([]byte, offsetLen[1])
						if _, err := wordIndexHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
							errorLogger.Printf("Seek error: %v", err)
							continue
						}
						if _, err := wordIndexHandle.Read(data); err != nil {
							errorLogger.Printf("Read error: %v", err)
							continue
						}
						if err := b.UnmarshalBinary(data); err != nil {
							errorLogger.Printf("Unmarshal error: %v", err)
							continue
						}
						temp.Or(b)
					}
				}

				// Fuzzy matches
				for _, algo := range fuzzyAlgos {
					for indexWord := range wordIndexHeader {
						if matchesConditionSingle(word, indexWord, algo) {
							if offsetLen, ok := wordIndexHeader[indexWord]; ok {
								if offsetLen[0] >= 0 && offsetLen[1] > 0 {
									b := roaring.New()
									data := make([]byte, offsetLen[1])
									if _, err := wordIndexHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
										errorLogger.Printf("Seek error: %v", err)
										continue
									}
									if _, err := wordIndexHandle.Read(data); err != nil {
										errorLogger.Printf("Read error: %v", err)
										continue
									}
									if err := b.UnmarshalBinary(data); err != nil {
										errorLogger.Printf("Unmarshal error: %v", err)
										continue
									}
									temp.Or(b)
								}
							}
						}
					}
				}

				// Gematria matches
				queryGematria := gematria.FromString(word)
				for gemKey, offsetLen := range wordIndexGematrias {
					parts := strings.SplitN(gemKey, "_", 2)
					if len(parts) != 2 {
						continue
					}
					gemType, gemValueStr := parts[0], parts[1]
					gemValue, err := strconv.ParseUint(gemValueStr, 10, 64)
					if err != nil {
						errorLogger.Printf("ParseUint error: %v", err)
						continue
					}
					// Match against queryGematria based on gemType
					matches := false
					switch gemType {
					case "simple":
						matches = queryGematria.Simple == gemValue
					case "english":
						matches = queryGematria.English == gemValue
					case "jewish":
						matches = queryGematria.Jewish == gemValue
					case "eights":
						matches = queryGematria.Eights == gemValue
					case "mystery":
						matches = queryGematria.Mystery == gemValue
					case "majestic":
						matches = queryGematria.Majestic == gemValue
					}
					if matches {
						if offsetLen[0] >= 0 && offsetLen[1] > 0 {
							b := roaring.New()
							data := make([]byte, offsetLen[1])
							if _, err := gemIndexHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
								errorLogger.Printf("Seek error for %s: %v", gemKey, err)
								continue
							}
							if _, err := gemIndexHandle.Read(data); err != nil {
								errorLogger.Printf("Read error for %s: %v", gemKey, err)
								continue
							}
							if err := b.UnmarshalBinary(data); err != nil {
								errorLogger.Printf("Unmarshal error for %s: %v", gemKey, err)
								continue
							}
							temp.Or(b)
						}
					}
				}
			}
			resultMutex.Lock()
			if resultBitmap.IsEmpty() {
				resultBitmap = temp
			} else {
				resultBitmap.And(temp)
			}
			resultMutex.Unlock()
		}
	}()

	// Process NOT conditions (similar logic)
	go func() {
		defer wg.Done()
		for _, notCond := range analysis.Nots {
			temp := roaring.New()
			var words []string
			if strings.HasPrefix(notCond, "(") && strings.HasSuffix(notCond, ")") {
				words = strings.Split(strings.Trim(notCond, "()"), " or ")
				for i, w := range words {
					words[i] = strings.TrimSpace(w)
				}
			} else {
				words = []string{notCond}
			}

			for _, word := range words {
				if offsetLen, ok := wordIndexHeader[word]; ok {
					if offsetLen[0] >= 0 && offsetLen[1] > 0 {
						b := roaring.New()
						data := make([]byte, offsetLen[1])
						if _, err := wordIndexHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
							errorLogger.Printf("Seek error for %s: %v", word, err)
							continue
						}
						if _, err := wordIndexHandle.Read(data); err != nil {
							errorLogger.Printf("Seek error for %s: %v", word, err)
							continue
						}
						if err := b.UnmarshalBinary(data); err != nil {
							errorLogger.Printf("Unmarshal error for %s: %v", word, err)
							continue
						}
						temp.Or(b)
					}
				}

				for _, algo := range fuzzyAlgos {
					for indexWord := range wordIndexHeader {
						if matchesConditionSingle(word, indexWord, algo) {
							if offsetLen, ok := wordIndexHeader[indexWord]; ok {
								if offsetLen[0] >= 0 && offsetLen[1] > 0 {
									b := roaring.New()
									data := make([]byte, offsetLen[1])
									if _, err := wordIndexHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
										errorLogger.Printf("wordIndexHandle.Seek error for %s: %v", word, err)
										continue
									}
									if _, err := wordIndexHandle.Read(data); err != nil {
										errorLogger.Printf("wordIndexHandle.Read(data) err: %v", err)
										continue
									}
									if err := b.UnmarshalBinary(data); err != nil {
										errorLogger.Printf("Unmarshal error for %s: %v", algo, err)
										continue
									}
									temp.Or(b)
								}
							}
						}
					}
				}

				queryGematria := gematria.FromString(word)
				for gemKey, offsetLen := range wordIndexGematrias {
					parts := strings.SplitN(gemKey, "_", 2)
					if len(parts) != 2 {
						continue
					}
					gemType, gemValueStr := parts[0], parts[1]
					gemValue, err := strconv.ParseUint(gemValueStr, 10, 64)
					if err != nil {
						errorLogger.Printf("Parse gematria for %s: %v", gemKey, err)
						continue
					}
					matches := false
					switch gemType {
					case "simple":
						matches = queryGematria.Simple == gemValue
					case "english":
						matches = queryGematria.English == gemValue
					case "jewish":
						matches = queryGematria.Jewish == gemValue
					case "eights":
						matches = queryGematria.Eights == gemValue
					case "mystery":
						matches = queryGematria.Mystery == gemValue
					case "majestic":
						matches = queryGematria.Majestic == gemValue
					}
					if matches {
						if offsetLen[0] >= 0 && offsetLen[1] > 0 {
							b := roaring.New()
							data := make([]byte, offsetLen[1])
							if _, err := gemIndexHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
								errorLogger.Printf("Seek error for %s: %v", gemKey, err)
								continue
							}
							if _, err := gemIndexHandle.Read(data); err != nil {
								errorLogger.Printf("Read error for %s: %v", gemKey, err)
								continue
							}
							if err := b.UnmarshalBinary(data); err != nil {
								errorLogger.Printf("Unmarshal error for %s: %v", gemKey, err)
								continue
							}
							temp.Or(b)
						}
					}
				}
			}
			resultMutex.Lock()
			resultBitmap.AndNot(temp)
			resultMutex.Unlock()
		}
	}()

	// Initialize results
	results := SearchResults{
		Categories: make(map[string][]string),
		HitCounts:  make(map[string]int),
		Matches:    make(map[string][]MatchDetail),
	}

	wg.Wait()

	// Process matching pages using the in-memory cache index
	itr := resultBitmap.Iterator()
	for itr.HasNext() {
		pageID := int(itr.Next())
		offsetLen, ok := cacheIdToOffset[pageID]
		if !ok {
			errorLogger.Printf("Page ID %d not found", pageID)
			continue
		}
		data := make([]byte, offsetLen[1])
		if _, err := cacheFileHandle.Seek(offsetLen[0], io.SeekStart); err != nil {
			errorLogger.Printf("Seek error: %v", err)
			continue
		}
		if _, err := cacheFileHandle.Read(data); err != nil {
			errorLogger.Printf("Search error for query %q: %v", query, err)
			continue
		}
		var page PageData
		if err := json.Unmarshal(data, &page); err != nil {
			errorLogger.Printf("Error parsing page: %v", err)
			continue
		}
		categoryMatched := make(map[string]bool)

		for _, andCond := range analysis.Ands {
			words := strings.Fields(andCond)
			for _, word := range words {
				queryGematria := gematria.FromString(word)
				if matchesExactTextee(word, page.Textee) {
					categoryMatched["exact/textee"] = true
				}
				for _, algo := range fuzzyAlgos {
					category := "fuzzy/" + algo
					for pw := range page.Textee.Gematrias {
						if matchesConditionSingle(word, pw, algo) {
							categoryMatched[category] = true
							break
						}
					}
				}
				for _, gemType := range gematriaTypes {
					category := "gematria/" + gemType
					for _, pg := range page.Textee.Gematrias {
						if matchesConditionGematria(pg, queryGematria) {
							categoryMatched[category] = true
							break
						}
					}
				}
			}
		}

		for category := range categoryMatched {
			results.Categories[category] = append(results.Categories[category], page.PageIdentifier)
			results.HitCounts[page.PageIdentifier]++
		}
	}

	duration := time.Since(startTime)
	log.Printf("Search for query %q completed in %v", query, duration)
	return results, nil
}
