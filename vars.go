package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/andreimerlescu/figs"
	"github.com/andreimerlescu/sema"
)

var (
	// runModes define the runtime of the gin web server, development local release and staging are search specific though
	runModes = []string{"local", "development", "release", "staging", "production"}

	// dataChanged indicates whether new data (e.g., OCR files) has been detected during periodic directory scans.
	// It triggers cache updates when set to true by the watcher.
	dataChanged bool

	// errorLogger is a logger instance that writes error messages to the configured error log file (e.g., "error.log").
	// It’s initialized in main.go to handle runtime errors and exceptions.
	errorLogger *log.Logger

	// cacheMutex provides read/write locking for concurrent access to cache-related files and operations.
	// It ensures thread safety during cache initialization, updates, and searches.
	cacheMutex sync.RWMutex

	// cfigs holds the application’s configuration settings, loaded from a YAML file or environment defaults.
	// It includes paths (e.g., cache directory), ports, and matching algorithm thresholds.
	cfigs figs.Fruit

	// configFile specifies the default path to the configuration file ("./config.yaml") if no environment variable override is provided.
	// Used by loadConfigs() as a fallback when configEnvKey is unset or invalid.
	configFile = filepath.Join(".", "config.yaml")

	// configEnvKey is the environment variable name ("CONFIG_FILE") checked for an alternative config file path.
	// If set (e.g., CONFIG_FILE=/path/to/config.yaml), it takes precedence over configFile.
	configEnvKey = "CONFIG_FILE"

	// cacheFile is the path to the cache file ("apario-search-cache.jsonl") storing serialized PageData structs.
	// Each line is a JSON-encoded entry containing a page’s text (via Textee), identifiers, and gematria mappings.
	// Used to retrieve full page data for search results, accessed via offsets from cacheIndexFile.
	cacheFile = "apario-search-cache.jsonl"

	// cacheIndexFile is the path to the cache index file ("cache_index.txt") mapping page IDs to their locations in cacheFile.
	// Each line follows the format "pageID offset length":
	//   - pageID: Integer ID (e.g., 0, 1, 2) corresponding to a page’s PageIdentifier.
	//   - offset: Byte position in cacheFile where the JSON line starts.
	//   - length: Byte length of the JSON line in cacheFile.
	// Example: "0 0 123" means page 0’s data starts at byte 0 and is 123 bytes long.
	cacheIndexFile = "cache_index.txt"

	// wordIndexFile is the path to the word index file ("word_index.bin"), a binary inverted index for word-based searches.
	// Structure:
	//   - Header (JSON): Maps words (e.g., "secret") to [offset, length] pairs, where offset is the byte position in the file’s body,
	//                    and length is the size of the Roaring Bitmap.
	//   - Body (binary): Roaring Bitmaps listing page IDs (e.g., [0, 5, 12]) where each word appears.
	// Used for fast word lookups and set operations during query processing.
	wordIndexFile = "word_index.bin"

	// gemIndexFile is the path to the gematria index file ("gematria_index.bin"), a binary index for gematria-based searches.
	// Structure:
	//   - Header (JSON): Maps gematria keys (e.g., "english_123", "simple_456") to [offset, length] pairs, where offset points to
	//                    the bitmap in the file’s body, and length is the bitmap’s size.
	//   - Body (binary): Roaring Bitmaps listing page IDs where each gematria value occurs.
	// Enables matching words by their numerical gematria values (e.g., English, Simple, Jewish).
	gemIndexFile = "gematria_index.bin"

	// groupingRegex is a compiled regular expression to match parenthetical groupings in search queries (e.g., "(top secret or confidential)").
	// It captures nested parentheses and their contents, used by AnalyzeQuery to identify OR conditions.
	groupingRegex = regexp.MustCompile(`\((?:[^()]+|\([^()]*\))+\)`)

	// searchManager is a global instance managing active search sessions and cached results.
	// - activeSearches: Tracks ongoing searches by keyword, mapping to SearchSession structs with channels and WebSocket clients.
	// - cache: Stores completed search results by keyword for quick reuse, avoiding redundant searches within an hour.
	searchManager = &SearchManager{
		activeSearches: make(map[string]*SearchSession),
		cache:          make(map[string]*SearchResult),
	}

	// wordIndexHeader is the in-memory map of words to [offset, length] pairs from word_index.bin.
	// Loaded at startup to avoid reading the file on every search request.
	wordIndexHeader map[string][2]int64 // TODO cant use this because duplicates for the key are needed to be preserved for multiple page results

	// wordIndexHandle is the file handle for word_index.bin, kept open for the lifetime of the application.
	// Used to read bitmaps during search without reopening the file.
	wordIndexHandle *os.File

	// cacheIdToOffset is the in-memory map of page IDs to [offset, length] pairs from cache_index.txt.
	// Loaded at startup to avoid reading the file on every search request.
	cacheIdToOffset map[int][2]int64

	// cacheFileHandle is the file handle for apario-search-cache.jsonl, kept open for the lifetime of the application.
	// Used to read PageData structs during search without reopening the file.
	cacheFileHandle *os.File

	// searchSemaphores provide per-IP limits on concurrent searches allowed and enforced with a semaphore instead of rate limiting alone
	searchSemaphores     = make(map[string]sema.Semaphore)
	searchSemaphoresLock = sync.RWMutex{}

	// systemSearchSemaphore is used for an application-wide limit on max concurrent searches allowed for all sessions
	systemSearchSemaphore sema.Semaphore

	// wordIndexGematrias maps gematria keys (e.g., "english_123") to the offset and length of their Roaring Bitmaps
	// in gematria_index.bin, containing page IDs where the gematria value appears.
	wordIndexGematrias map[string][2]int64

	// gemIndexHandle is the file handle for gematria_index.bin, kept open for the lifetime of the application.
	gemIndexHandle *os.File
)

const (
	kilobyte = 1024
	megabyte = 1024 * kilobyte
	gigabyte = 1024 * megabyte
	terabyte = 1024 * gigabyte
	petabyte = 1024 * terabyte
)

const DefaultRobotsTxt = `User-agent: *
Disallow: /`

const DefaultAdsTxt = ``

const DefaultSecurityTxt = ``
