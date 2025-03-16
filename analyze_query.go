package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

func AnalyzeQuery(q string) SearchAnalysis {
	sa := SearchAnalysis{
		Ors:  make(map[uint]string),
		Ands: []string{},
		Nots: []string{},
	}

	// Normalize query
	q = strings.ToLower(q)
	q = strings.ReplaceAll(q, " && ", " and ")
	q = strings.ReplaceAll(q, " & ", " and ")
	q = strings.ReplaceAll(q, " !", " not ")
	q = strings.ReplaceAll(q, ", ", " or ")
	q = strings.ReplaceAll(q, ",", " or ")
	q = strings.ReplaceAll(q, "||", " or ")
	q = strings.ReplaceAll(q, "{", "(")
	q = strings.ReplaceAll(q, "}", ")")
	q = strings.ReplaceAll(q, "[", "(")
	q = strings.ReplaceAll(q, "]", ")")
	q = "and " + q

	// Extract OR groupings
	matches := groupingRegex.FindAllString(q, -1)
	orCounter := uint(0)
	for _, match := range matches {
		if strings.Contains(match, " or ") {
			orCounter++
			orContent := match[1 : len(match)-1] // Remove parentheses
			sa.Ors[orCounter] = orContent
			q = strings.Replace(q, match, fmt.Sprintf("OR_%d", orCounter), 1)
		}
	}

	// Parse query word-by-word
	words := strings.Fields(q)
	var buffer string
	addToAnd := true

	for i, word := range words {
		switch word {
		case "and":
			if buffer != "" {
				if addToAnd {
					sa.Ands = append(sa.Ands, strings.TrimSpace(buffer))
				} else {
					sa.Nots = append(sa.Nots, strings.TrimSpace(buffer))
				}
				buffer = ""
			}
			addToAnd = true
		case "not":
			if buffer != "" {
				if addToAnd {
					sa.Ands = append(sa.Ands, strings.TrimSpace(buffer))
				} else {
					sa.Nots = append(sa.Nots, strings.TrimSpace(buffer))
				}
				buffer = ""
			}
			addToAnd = false
		case "or":
			continue // Handled by regex
		default:
			if strings.HasPrefix(word, "OR_") {
				orID, _ := strconv.Atoi(strings.TrimPrefix(word, "OR_"))
				orContent := "(" + sa.Ors[uint(orID)] + ")" // Preserve grouping
				if addToAnd {
					sa.Ands = append(sa.Ands, orContent)
				} else {
					sa.Nots = append(sa.Nots, orContent)
				}
			} else {
				buffer += " " + word
			}
			if i == len(words)-1 && buffer != "" {
				if addToAnd {
					sa.Ands = append(sa.Ands, strings.TrimSpace(buffer))
				} else {
					sa.Nots = append(sa.Nots, strings.TrimSpace(buffer))
				}
			}
		}
	}

	sa.Ands = removeDuplicates(sa.Ands)
	sa.Nots = removeDuplicates(sa.Nots)
	log.Printf("Analyzed query: Ors=%v, Ands=%v, Nots=%v", sa.Ors, sa.Ands, sa.Nots)
	return sa
}

func removeSoloOrs(partialWord string) string {
	re, err := regexp.Compile(`\((.*?)or(.*?)\)`)
	if err != nil {
		log.Println(err)
		return partialWord
	}

	matches := re.FindAllStringSubmatch(partialWord, -1)
	if len(matches) < 2 {
		// match
		partialWord = strings.Replace(partialWord, `and (`, ``, -1)
		partialWord = strings.Replace(partialWord, `)`, ``, -1)
	}

	return partialWord
}

func removeDuplicates(slice []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range slice {
		if s == "" {
			continue
		}
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}
