package main

import (
	"log"
	"strings"

	"github.com/andreimerlescu/gematria"
	"github.com/andreimerlescu/textee"
	"github.com/xrash/smetrics"
)

func matchesExactTextee(query string, textee *textee.Textee) bool {
	for word := range textee.Gematrias {
		if word == query {
			return true
		}
	}
	return false
}

func matchesCondition(query string, pageWords map[string]gematria.Gematria, queryGematria gematria.Gematria, algo string) bool {
	if strings.Contains(query, " ") {
		words := strings.Fields(query)
		for _, qw := range words {
			qwGematria := gematria.FromString(qw)
			found := false
			for pw, pg := range pageWords {
				if matchesConditionSingle(qw, pw, algo) || matchesConditionGematria(pg, qwGematria) {
					found = true
					break
				}
			}
			if !found {
				return false // If any word in the phrase isn’t found, the phrase doesn’t match
			}
		}
		return true
	} else {
		for pw, pg := range pageWords {
			if matchesConditionSingle(query, pw, algo) || matchesConditionGematria(pg, queryGematria) {
				return true // Any match means the single word condition is satisfied
			}
		}
		return false // No match found
	}
}

func matchesConditionGematria(gematrix gematria.Gematria, queryGematria gematria.Gematria) bool {
	if queryGematria.English == gematrix.English ||
		queryGematria.Simple == gematrix.Simple ||
		queryGematria.Jewish == gematrix.Jewish ||
		queryGematria.Eights == gematrix.Eights ||
		queryGematria.Mystery == gematrix.Mystery ||
		queryGematria.Majestic == gematrix.Majestic {
		return true
	}
	return false
}

func matchesConditionSingle(query, word string, algo string) bool {
	// If no gematria match, use the specified string similarity algorithm
	switch algo {
	case "jaro":
		return smetrics.Jaro(query, word) >= *cfigs.Float64(kJaroThreshold)
	case "jaro-winkler":
		return smetrics.JaroWinkler(query, word, *cfigs.Float64(kJaroWinklerBoostThreshold), *cfigs.Int(kJaroWinklerPrefixSize)) >= *cfigs.Float64(kJaroWinklerThreshold)
	case "soundex":
		return smetrics.Soundex(query) == smetrics.Soundex(word)
	case "hamming":
		subs, err := smetrics.Hamming(query, word)
		return err == nil && subs <= *cfigs.Int(kHammingMaxSubs)
	case "ukkonen":
		score := smetrics.Ukkonen(query, word, *cfigs.Int(kUkkonenICost), *cfigs.Int(kUkkonenDCost), *cfigs.Int(kUkkonenSCost))
		return score <= *cfigs.Int(kUkkonenMaxSubs)
	case "wagner-fisher":
		score := smetrics.WagnerFischer(query, word, *cfigs.Int(kWagnerFisherICost), *cfigs.Int(kWagnerFisherDCost), *cfigs.Int(kWagnerFisherSCost))
		return score <= *cfigs.Int(kWagnerFisherMaxSubs)
	default:
		log.Printf("Unknown algorithm: %s", algo)
		return false
	}
}
