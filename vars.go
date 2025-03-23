package main

import (
	"github.com/andreimerlescu/configurable"
	"log"
	"path/filepath"
	"regexp"
	"sync"
)

var (
	dataChanged    bool // Flag for data changes during hourly scan
	errorLogger    *log.Logger
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
