package main

import (
	"context"
	"github.com/andreimerlescu/gematria"
	"github.com/andreimerlescu/go-smartchan"
	"github.com/gin-gonic/gin"
	"net/http"
	"sort"
	"sync"
)

func handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing query"})
		return
	}

	sortParam := c.Query("sort")
	rank := sortParam == "ranked"

	results := search(query)
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

// search performs a categorized search and returns results with hit counts and match details
func search(query string) SearchResults {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	analysis := AnalyzeQuery(query)
	results := make(map[string]struct{}) // Temporary set for initial filtering
	categorizedResults := make(map[string][]string)
	hitCounts := make(map[string]int)
	matches := make(map[string][]MatchDetail)

	ctx := context.Background()
	sch := go_smartchan.NewSmartChan(1000)
	var wg sync.WaitGroup

	// Process the entire analysis in one go
	wg.Add(1)
	go func() {
		defer wg.Done()
		findPagesForWord(ctx, sch, analysis)
	}()

	go func() {
		wg.Wait()
		sch.Close()
	}()

	// Collect results
	for data := range sch.Chan() {
		if pageID, ok := data.(string); ok {
			results[pageID] = struct{}{}
		}
	}

	// Categorize results and track hits with details
	algo := *cfg.String(kAlgo)
	for pageID := range results {
		data, exists := pageCache[pageID]
		if !exists {
			continue
		}

		for _, andCond := range analysis.Ands {
			queryGematria := gematria.FromString(andCond)

			// Exact match on Textee words
			if matchesExactTextee(andCond, data.Textee) {
				categorizedResults["exact/textee"] = append(categorizedResults["exact/textee"], pageID)
				hitCounts[pageID]++
				matches[pageID] = append(matches[pageID], MatchDetail{
					Text:       andCond,
					Gematria:   queryGematria,
					TexTeeTexT: data.Textee.Input,
					Category:   "exact/textee",
				})
			}

			// Fuzzy match with selected algorithm
			for word := range data.Textee.Gematrias {
				if matchesConditionSingle(andCond, word, algo) {
					categorizedResults["fuzzy/"+algo] = append(categorizedResults["fuzzy/"+algo], pageID)
					hitCounts[pageID]++
					matches[pageID] = append(matches[pageID], MatchDetail{
						Text:       word,
						Gematria:   data.Textee.Gematrias[word],
						TexTeeTexT: data.Textee.Input,
						Category:   "fuzzy/" + algo,
					})
				}
			}

			// Gematria match
			for word, gematriaVal := range data.Textee.Gematrias {
				if matchesConditionGematria(gematriaVal, queryGematria) {
					categorizedResults["gematria/simple"] = append(categorizedResults["gematria/simple"], pageID)
					hitCounts[pageID]++
					matches[pageID] = append(matches[pageID], MatchDetail{
						Text:       word,
						Gematria:   gematriaVal,
						TexTeeTexT: data.Textee.Input,
						Category:   "gematria/simple",
					})
				}
			}
		}
	}

	return SearchResults{
		Categories: categorizedResults,
		HitCounts:  hitCounts,
		Matches:    matches,
	}
}
