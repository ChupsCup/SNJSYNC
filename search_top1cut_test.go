package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var reParen = regexp.MustCompile(`\(([A-Za-z0-9 .&'-]{2,30})\)`)

func cutFromLabel(lab string) string {
	words := splitWords(normText(lab))
	for n := 3; n >= 1; n-- {
		for i := 0; i+n <= len(words); i++ {
			k := cutKey(strings.Join(words[i:i+n], ""))
			if cutKnown(k) && len(k) >= 4 {
				return k
			}
		}
	}
	return ""
}

func TestTopOneCutAccuracy(t *testing.T) {
	app := loadRealApp(t)
	si := app.getSearchIndex()

	type pair struct{ brand, cut string }
	seen := map[pair]int{}
	for _, en := range si.entries {
		lab := prodLabel(en.e.Dir)
		if strings.Contains(strings.ToUpper(lab), "BUFFALO") {
			continue
		}
		cut := cutFromLabel(lab)
		if cut == "" {
			continue
		}
		m := reParen.FindStringSubmatch(lab)
		if m == nil {
			continue
		}
		b := strings.TrimSpace(m[1])
		if len(b) < 2 {
			continue
		}
		seen[pair{b, cut}]++
	}
	keys := make([]pair, 0, len(seen))
	for k, v := range seen {
		if v >= 2 {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].brand != keys[j].brand {
			return keys[i].brand < keys[j].brand
		}
		return keys[i].cut < keys[j].cut
	})

	ok, bad, miss := 0, 0, 0
	var badList []string
	for _, k := range keys {
		q := k.brand + " " + strings.ToLower(k.cut)
		resp := app.search(q)
		if len(resp.Results) == 0 {
			miss++
			if len(badList) < 40 {
				badList = append(badList, fmt.Sprintf("KOSONG %-34s partial=%d top=%s", q, len(resp.Partial), firstDir(resp.Partial)))
			}
			continue
		}
		nb, nc := flat(k.brand), flat(k.cut)
		good := false
		rank := 0
		for r := 0; r < len(resp.Results) && r < 5; r++ {
			n := flat(resp.Results[r].Dir)
			if strings.Contains(n, nb) {
				good = true
				rank = r + 1
				break
			}
		}
		if good && rank == 1 {
			cok := strings.Contains(flat(resp.Results[0].Dir), nc)
			if cok {
				ok++
				continue
			}
			bad++
			if len(badList) < 40 {
				badList = append(badList, fmt.Sprintf("CUTBEDA %-33s top1=%s", q, trunc(prodLabel(resp.Results[0].Dir), 55)))
			}
			continue
		}
		if good {
			bad++
			if len(badList) < 40 {
				badList = append(badList, fmt.Sprintf("TURUN#%d %-33s top1=%s", rank, q, trunc(prodLabel(resp.Results[0].Dir), 55)))
			}
			continue
		}
		bad++
		if len(badList) < 40 {
			badList = append(badList, fmt.Sprintf("HILANG %-34s top1=%s", q, trunc(prodLabel(resp.Results[0].Dir), 55)))
		}
	}
	n := len(keys)
	fmt.Printf("\npasangan merek+potongan (>=2 foto): %d\n", n)
	fmt.Printf("top1 benar  : %d (%.1f%%)\n", ok, pct(ok, n))
	fmt.Printf("salah       : %d (%.1f%%)\n", bad, pct(bad, n))
	fmt.Printf("kosong      : %d (%.1f%%)\n", miss, pct(miss, n))
	for _, s := range badList {
		fmt.Println("   " + s)
	}
}

func firstDir(list []photoResult) string {
	if len(list) == 0 {
		return "-"
	}
	return trunc(prodLabel(list[0].Dir), 55)
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}
