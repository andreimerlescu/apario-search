package main

import (
	check "github.com/andreimerlescu/checkfs"
	"github.com/andreimerlescu/checkfs/file"
	"github.com/andreimerlescu/configurable"
	"os"
	"path/filepath"
)

func init() {
	cfg = configurable.New()
	cfg.NewString(kDir, ".", "Directory to scan for ocr.*.txt files")
	cfg.NewString(kPort, "18004", "HTTP port to use 1000-65534")
	cfg.NewString(kCacheDir, filepath.Join(".", "cache"), "Path to the search cache index directory")
	cfg.NewString(kReaderDomain, "idoread.com", "Domain name of the project excluding protocol path or query from the URL")
	cfg.NewFloat64(kJaroThreshold, 0.71, "1.0 means exact match 0.0 means no match; default is 0.71")
	cfg.NewFloat64(kJaroWinklerThreshold, 0.71, "using the JaroWinkler method, define the threshold that is tolerated; default is 0.71")
	cfg.NewFloat64(kJaroWinklerBoostThreshold, 0.7, "weight applied to common prefixes in matched strings comparing dictionary terms, page word data, and search query params")
	cfg.NewInt(kJaroWinklerPrefixSize, 3, "length of a jarrow weighted prefix string")
	cfg.NewInt(kUkkonenICost, 1, "insert cost ; when adding a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kUkkonenSCost, 2, "substitution cost ; when replacing a char increase the score by this number ; default = 2")
	cfg.NewInt(kUkkonenDCost, 1, "delete cost ; when removing a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kUkkonenMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accurate ; lower value = higher accuracy ; min = 0; default = 2")
	cfg.NewInt(kWagnerFisherICost, 1, "insert cost ; when adding a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kWagnerFisherSCost, 2, "substitution cost ; when replacing a char increase the score by this number ; default = 2")
	cfg.NewInt(kWagnerFisherDCost, 1, "delete cost ; when removing a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kWagnerFisherMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accurate ; lower value = higher accuracy ; min = 0; default = 2")
	cfg.NewInt(kHammingMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accuracy ; min = 1 ; default = 2")
}

func loadConfigs() error {
	if fn := os.Getenv(configEnvKey); len(fn) > 0 {
		if err := check.File(fn, file.Options{Exists: true}); err == nil {
			if err = cfg.Parse(fn); err != nil {
				return err
			} else {
				return nil
			}
		}
	} else if err := check.File(configFile, file.Options{Exists: true}); err == nil {
		if err = cfg.Parse(configFile); err != nil {
			return err
		} else {
			return nil
		}
	} else {
		if err = cfg.Parse(""); err != nil {
			return err
		} else {
			return nil
		}
	}
	return nil
}
