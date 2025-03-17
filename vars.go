package main

import (
	"github.com/andreimerlescu/configurable"
	"regexp"
	"sync"
	"sync/atomic"
)

var (
	pageCache     map[string]*PageData
	documentCache map[string]string
	cacheMutex    sync.RWMutex
	isCacheReady  atomic.Bool
	dataChanged   bool // Flag for data changes during hourly scan
	cacheFile     = "apario-search-cache.json"
	//dir           = flag.String("dir", ".", "Directory to scan for ocr.*.txt files")
	groupingRegex = regexp.MustCompile(`\((?:[^()]+|\([^()]*\))+\)`)
	cfg           configurable.IConfigurable

	searchManager = &SearchManager{
		activeSearches: make(map[string]*SearchSession),
		cache:          make(map[string]*SearchResult),
	}
)

// Initialize pageCache (replace with your existing initialization)
func init() {
	pageCache = make(map[string]*PageData)
	documentCache = make(map[string]string)
}
