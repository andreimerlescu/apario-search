package main

import (
	"github.com/andreimerlescu/configurable"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
)

var (
	dataChanged    bool // Flag for data changes during hourly scan
	isCacheReady   atomic.Bool
	cacheMutex     sync.RWMutex
	cfg            configurable.IConfigurable
	configFile     = filepath.Join(".", "config.yaml")
	configEnvKey   = "CONFIG_FILE"
	cacheFile      = "apario-search-cache.jsonl"
	cacheIndexFile = "cache_index.txt"
	wordIndexFile  = "word_index.bin"
	gemIndexFile   = "gematria_index.bin"
	groupingRegex  = regexp.MustCompile(`\((?:[^()]+|\([^()]*\))+\)`)

	searchManager = &SearchManager{
		activeSearches: make(map[string]*SearchSession),
		cache:          make(map[string]*SearchResult),
	}
)
