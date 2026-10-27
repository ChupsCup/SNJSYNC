package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func contentWords(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.Fields(strings.ToLower(s)) {
		t = strings.Trim(t, ".,()[]/&+-")
		if len(t) < 4 {
			continue
		}
		switch t {
		case "frozen", "fresh", "chilled", "beef", "bovine", "bone", "boneless", "carcass", "kilo", "kgs", "kg", "gram", "grams", "vial", "box", "pack", "pct", "est", "ind", "in", "of", "the", "and", "for", "with":
			continue
		}
		out[t] = true
	}
	return out
}

func TestAudit9JalurKodeProdukBeda(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("=== AUDIT 9: MATCH VIA KODE SNJ TAPI NAMA PRODUK BEDA ===")
	rowCache := map[string]lookupHit{}
	grp := map[string]int{}
	detail := map[string]string{}
	total := 0
	for i := range ad.pl {
		r := &ad.pl[i]
		if strings.TrimSpace(r.NamaItem) == "" {
			continue
		}
		sig := normStr(r.Brand + "|" + r.NamaItem + "|" + r.ItemDesc + "|" + r.Gabungan +
			"|" + r.Sheet + "|" + r.Importir + "|" + r.File)
		hit, cached := rowCache[sig]
		if !cached {
			hit = ad.eng.lookupMaster(r)
			rowCache[sig] = hit
		}
		if hit.status != "EXISTING" || !strings.HasPrefix(hit.note, "Kode kepemilikan SNJ") {
			continue
		}
		ent := ad.eng.masterByCode[hit.m.ItemCode]
		if ent == nil {
			continue
		}
		plw := contentWords(r.NamaItem + " " + r.Gabungan)
		mw := contentWords(ent.m.ItemDesc)
		brandWords := map[string]bool{}
		for _, p := range ent.parens.List() {
			brandWords[p] = true
		}
		overlap := false
		for w := range plw {
			if mw[w] && !brandWords[w] {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		total++
		k := r.Brand + " / " + r.NamaItem
		grp[k]++
		if detail[k] == "" {
			detail[k] = "PL=" + r.NamaItem + " master=" + ent.m.ItemCode + " " + ent.m.ItemDesc
		}
	}
	keys := []string{}
	for k := range grp {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if grp[keys[i]] != grp[keys[j]] {
			return grp[keys[i]] > grp[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Printf("match jalur kode tanpa overlap nama produk: %d baris (%d item unik)\n", total, len(keys))
	for i, k := range keys {
		if i >= 30 {
			fmt.Printf("  ... %d item unik lain\n", len(keys)-30)
			break
		}
		fmt.Printf("  %-44s %3d baris  %s\n", k, grp[k], detail[k])
	}
}

func TestAudit10BrandKomposit(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("\n=== AUDIT 10: BRAND PL KOMPOSIT (GABUNGAN BRAND) ===")
	rowCache := map[string]lookupHit{}
	grp := map[string]int{}
	detail := map[string]string{}
	for i := range ad.pl {
		r := &ad.pl[i]
		if strings.TrimSpace(r.NamaItem) == "" {
			continue
		}
		sig := normStr(r.Brand + "|" + r.NamaItem + "|" + r.ItemDesc + "|" + r.Gabungan +
			"|" + r.Sheet + "|" + r.Importir + "|" + r.File)
		hit, cached := rowCache[sig]
		if !cached {
			hit = ad.eng.lookupMaster(r)
			rowCache[sig] = hit
		}
		parts := canonParts(r.Brand)
		if len(parts) < 2 {
			continue
		}
		k := r.Brand + " / " + r.NamaItem
		grp[k]++
		if detail[k] == "" {
			detail[k] = fmt.Sprintf("status=%s note=%s | parts=%v", hit.status, hit.note, parts)
		}
	}
	keys := []string{}
	for k := range grp {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if grp[keys[i]] != grp[keys[j]] {
			return grp[keys[i]] > grp[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Printf("baris dengan brand komposit: %d baris (%d item unik)\n", len(keys), len(grp))
	for i, k := range keys {
		if i >= 25 {
			fmt.Printf("  ... %d lagi\n", len(keys)-25)
			break
		}
		fmt.Printf("  %-44s %3d baris  %s\n", k, grp[k], detail[k])
	}
}
