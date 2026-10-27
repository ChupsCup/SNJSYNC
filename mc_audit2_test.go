package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestAuditImportirMismatch(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("=== AUDIT 7: EXISTING tapi importir PL != importir master ===")

	brandProd := map[string]map[string]bool{}
	for _, ent := range ad.eng.entries {
		brand := strings.Join(ent.parens.List(), "+")
		prod := ""
		for _, p := range ent.prods.List() {
			prod = p
			break
		}
		if brand == "" || prod == "" {
			continue
		}
		k := brand + "|" + prod
		if brandProd[k] == nil {
			brandProd[k] = map[string]bool{}
		}
		brandProd[k][ent.importir] = true
	}

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
		if hit.status != "EXISTING" || hit.m == nil {
			continue
		}
		plImp := r.importirPL(normME(normVL(plur(r.ItemDesc))), normME(normVL(plur(r.Gabungan))))
		if plImp == "" {
			continue
		}
		ent := ad.eng.masterByCode[hit.m.ItemCode]
		if ent == nil {
			continue
		}
		if ent.importir == plImp {
			continue
		}
		prod := ""
		for _, p := range ent.prods.List() {
			prod = p
			break
		}
		alt := brandProd[strings.Join(ent.parens.List(), "+")+"|"+prod]
		adaYangCocok := false
		for imp := range alt {
			if imp == plImp {
				adaYangCocok = true
			}
		}
		if !adaYangCocok {
			continue
		}
		total++
		k := r.Brand + " / " + r.NamaItem
		grp[k]++
		if detail[k] == "" {
			detail[k] = fmt.Sprintf("PL=%s master=%s(%s) kandidat_lain: %v", plImp, hit.m.ItemCode, ent.importir, keysOf(alt))
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
	fmt.Printf("baris EXISTING dengan importir meleset padahal ada master yang cocok: %d (%d item unik)\n", total, len(keys))
	for i, k := range keys {
		if i >= 30 {
			fmt.Printf("  ... %d item unik lain\n", len(keys)-30)
			break
		}
		fmt.Printf("  %-44s %3d baris  %s\n", k, grp[k], detail[k])
	}
}

func keysOf(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestAuditBrandPLvsMaster(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("\n=== AUDIT 8: BRAND PL YANG TIDAK PUNYA MASTER (kandidat baru) ===")
	rowCache := map[string]lookupHit{}
	brands := map[string]int{}
	brandsEx := map[string]bool{}
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
		if hit.status == "EXISTING" || r.Brand == "" {
			continue
		}
		nb := normBrand(r.Brand)
		if nb == "" {
			continue
		}
		canon := nb
		if c, ok := brandAliases[nb]; ok {
			canon = c
		}
		if len(ad.eng.brandIndex[canon]) > 0 {
			continue
		}
		brands[r.Brand]++
		brandsEx[r.Brand] = brandsEx[r.Brand] || hit.status == "EXISTING"
	}
	keys := []string{}
	for k := range brands {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if brands[keys[i]] != brands[keys[j]] {
			return brands[keys[i]] > brands[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Printf("brand PL tanpa kandidat master: %d\n", len(keys))
	for i, k := range keys {
		if i >= 40 {
			fmt.Printf("  ... %d lagi\n", len(keys)-40)
			break
		}
		fmt.Printf("  %-32s %3d baris\n", k, brands[k])
	}
}
