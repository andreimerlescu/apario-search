package main

import (
	"context"
	"github.com/andreimerlescu/gematria"
	go_smartchan "github.com/andreimerlescu/go-smartchan"
	"log"
)

func findPagesForWord(ctx context.Context, sch *go_smartchan.SmartChan, analysis SearchAnalysis) {
	algo := *cfg.String(kAlgo)

	send := func(ctx context.Context, pageID string) {
		select {
		case <-ctx.Done():
			return
		default:
			if sch.CanWrite() {
				if err := sch.Write(pageID); err != nil {
					log.Println(err)
				}
			}
		}
	}

	for pageID, data := range pageCache {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check all AND conditions (page must match all)
		matchesAllAnds := true
		if len(analysis.Ands) == 0 {
			matchesAllAnds = true // No AND conditions means true by default
		} else {
			for _, andCond := range analysis.Ands {
				andGematria := gematria.FromString(andCond)
				if !matchesCondition(andCond, data.Textee.Gematrias, andGematria, algo) {
					matchesAllAnds = false
					break
				}
			}
		}

		if !matchesAllAnds {
			continue
		}

		// Check NOT conditions (page must not match any)
		for _, notCond := range analysis.Nots {
			notGematria := gematria.FromString(notCond)
			if matchesCondition(notCond, data.Textee.Gematrias, notGematria, algo) {
				goto nextPage
			}
		}

		// If all ANDs are satisfied and no NOTs are matched, include the page
		send(ctx, pageID)

	nextPage:
		continue
	}
}
