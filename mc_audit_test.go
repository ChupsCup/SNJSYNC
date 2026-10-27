package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const auditDir = "/mnt/c/Users/Ucup/AppData/Local/Temp/opencode/"

func auditCSV(t *testing.T, name string) [][]string {
	f, err := os.Open(auditDir + name)
	if err != nil {
		t.Skipf("tidak ada %s: %v", name, err)
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatalf("csv %s: %v", name, err)
	}
	return rows
}

type auditData struct {
	masters   []mlItem
	pl        []plItem
	decisions map[string]*reviewDecision
	eng       *compareEngine
}

func loadAudit(t *testing.T) *auditData {
	mrows := auditCSV(t, "prod_master.csv")
	prows := auditCSV(t, "prod_packing.csv")
	drows := auditCSV(t, "prod_decisions.csv")
	masters := make([]mlItem, 0, len(mrows)-1)
	for _, r := range mrows[1:] {
		id, _ := strconv.Atoi(r[0])
		masters = append(masters, mlItem{ID: id, ItemCode: r[1], ItemDesc: r[2]})
	}
	decisions := map[string]*reviewDecision{}
	for _, r := range drows[1:] {
		decisions[r[0]] = &reviewDecision{Key: r[0], Decision: r[1], MasterCode: r[2], MasterDesc: r[3]}
	}
	pl := make([]plItem, 0, len(prows)-1)
	for _, r := range prows[1:] {
		id, _ := strconv.Atoi(r[0])
		uid, _ := strconv.Atoi(r[1])
		pl = append(pl, plItem{ID: id, UploadID: uid, Sheet: r[2], NoContainer: r[3],
			Brand: r[4], NamaItem: r[5], ETA: r[6], ItemCode: r[7], ItemDesc: r[8],
			Gabungan: r[9], Importir: r[10],
			File: "KC KERBAU DAN SAPI PPI 2026 Menarik 20Oct - Copy.xlsx"})
	}
	eng := newCompareEngine()
	eng.build(masters, decisions)
	return &auditData{masters: masters, pl: pl, decisions: decisions, eng: eng}
}

type strongHit struct {
	ent    *masterEntry
	viaCode bool
}

func scanStrong(ad *auditData, r *plItem) []strongHit {
	brand := normBrand(r.Brand)
	if brand == "" {
		return nil
	}
	cands := ad.eng.brandIndex[brand]
	if len(cands) == 0 {
		return nil
	}
	rawNama := normME(normVL(plur(r.NamaItem)))
	itemDesc := normME(normVL(plur(r.ItemDesc)))
	gab := normME(normVL(plur(r.Gabungan)))
	plCodes := extractCodes2(rawNama + " " + itemDesc)
	plImportir := r.importirPL(itemDesc, gab)
	plProds := newSetFromSlice(prodKeys(rawNama + " " + itemDesc))
	out := []strongHit{}
	for _, ent := range cands {
		if plImportir != "" && ent.importir != "" && ent.importir != plImportir {
			continue
		}
		viaCode := false
		if plCodes.Len() > 0 {
			for _, c := range plCodes.List() {
				if ent.snjCodes.Has(c) {
					viaCode = true
					break
				}
			}
		}
		prodHit := false
		if plProds.Len() > 0 {
			for _, p := range plProds.List() {
				if ent.prods.Has(p) {
					prodHit = true
					break
				}
			}
		}
		if viaCode || prodHit {
			out = append(out, strongHit{ent: ent, viaCode: viaCode})
		}
	}
	return out
}

func TestAuditKeputusanManual(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("=== AUDIT 1: KEPUTUSAN MANUAL (review_decisions) ===")
	keys := []string{}
	for k := range ad.decisions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		dec := ad.decisions[k]
		parts := strings.SplitN(k, "|", 2)
		brand, nama := "", ""
		if len(parts) == 2 {
			brand, nama = parts[0], parts[1]
		}
		fake := &plItem{Brand: brand, NamaItem: nama, Gabungan: brand + " " + nama}
		hit := ad.eng.lookupMaster(fake)
		strong := scanStrong(ad, fake)
		if dec.Decision == "new" {
			status := "AMAN (tidak ada di master)"
			if len(strong) > 0 {
				status = "BAKI -> item sudah ada di master!"
			}
			fmt.Printf("  [%s] %-42s hasil=%-8s kandidat=%d  %s\n", dec.Decision, k, hit.status, len(strong), status)
			for _, s := range strong {
				fmt.Printf("        kandidat: %-20s %s  (viaKode=%v)\n", s.ent.m.ItemCode, s.ent.m.ItemDesc, s.viaCode)
			}
		} else {
			mc, ok := ad.eng.masterByCode[dec.MasterCode]
			desc := "-"
			if ok {
				desc = mc.m.ItemDesc
			}
			brandDiMaster := ok && (strings.Contains(strings.ToUpper(mc.m.ItemDesc), "("+strings.ToUpper(brand)+")") ||
				strings.Contains(normBrand(mc.m.ItemDesc), normBrand(brand)))
			fmt.Printf("  [%s] %-42s master=%-20s ada=%-5v brandDiMaster=%-5v\n", dec.Decision, k, dec.MasterCode, ok, brandDiMaster)
			fmt.Printf("        master: %s\n", desc)
		}
	}
}

func TestAuditFalseNew(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("\n=== AUDIT 2: FALSE NEW (baris NEW/REVIEW yang sebenarnya ada di master) ===")
	rowCache := map[string]lookupHit{}
	grp := map[string]int{}
	grpEx := map[string]bool{}
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
		if hit.status == "EXISTING" {
			continue
		}
		strong := scanStrong(ad, r)
		if len(strong) == 0 {
			continue
		}
		total++
		viaCode := false
		cands := []string{}
		for _, s := range strong {
			if s.viaCode {
				viaCode = true
			}
			cands = append(cands, s.ent.m.ItemCode+" "+s.ent.m.ItemDesc)
		}
		k := r.Brand + " / " + r.NamaItem
		grp[k]++
		if viaCode {
			grpEx[k] = true
			detail[k] = "KODE-SAMA: " + strings.Join(cands, " ;; ")
		} else if detail[k] == "" {
			detail[k] = "PRODUK-SAMA: " + strings.Join(cands, " ;; ")
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
	fmt.Printf("total baris NEW/REVIEW dengan kandidat master: %d (dari %d item unik)\n", total, len(keys))
	for _, k := range keys {
		tag := "PRODUK-SAMA"
		if grpEx[k] {
			tag = ">>> KODE-SAMA (pasti salah) <<<"
		}
		fmt.Printf("  %-46s %3d baris  %s\n", k, grp[k], tag)
		fmt.Printf("        %s\n", detail[k])
	}
}

func TestAuditFalseExisting(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("\n=== AUDIT 3: EXISTING tapi master-nya tidak punya brand PL ===")
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
		br := normBrand(r.Brand)
		if br == "" {
			continue
		}
		mdesc := normBrand(hit.m.ItemDesc)
		if strings.Contains(mdesc, br) {
			continue
		}
		total++
		k := r.Brand + " / " + r.NamaItem
		grp[k]++
		if detail[k] == "" {
			detail[k] = hit.m.ItemCode + " " + hit.m.ItemDesc
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
	fmt.Printf("total baris EXISTING tanpa jejak brand PL: %d (dari %d item unik)\n", total, len(keys))
	for i, k := range keys {
		if i >= 40 {
			fmt.Printf("  ... %d item unik lain\n", len(keys)-40)
			break
		}
		fmt.Printf("  %-46s %3d baris -> %s\n", k, grp[k], detail[k])
	}
}

func TestAuditMasterDuplikatDanCoverage(t *testing.T) {
	ad := loadAudit(t)
	fmt.Println("\n=== AUDIT 4: KODE MASTER DUPLIKAT ===")
	codeCount := map[string]int{}
	codeDesc := map[string]string{}
	for _, m := range ad.masters {
		if m.ItemCode == "" {
			continue
		}
		codeCount[m.ItemCode]++
		if codeDesc[m.ItemCode] == "" {
			codeDesc[m.ItemCode] = m.ItemDesc
		}
	}
	dup := 0
	for _, c := range codeCount {
		if c > 1 {
			dup++
		}
	}
	fmt.Printf("kode duplikat: %d dari %d kode unik\n", dup, len(codeCount))
	n := 0
	for _, m := range ad.masters {
		if m.ItemCode != "" && codeCount[m.ItemCode] > 1 {
			fmt.Printf("  %-22s x%d  %s\n", m.ItemCode, codeCount[m.ItemCode], m.ItemDesc)
			n++
			if n >= 15 {
				break
			}
		}
	}

	fmt.Println("\n=== AUDIT 5: ITEM MASTER YANG TIDAK PERNAH KE-MATCH ===")
	matched := map[int]bool{}
	rowCache := map[string]lookupHit{}
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
		if hit.m != nil {
			matched[hit.m.ID] = true
		}
	}
	unmatched := 0
	byBrand := map[string]int{}
	for _, m := range ad.masters {
		if !matched[m.ID] {
			unmatched++
			ent := ad.eng.masterByCode[m.ItemCode]
			br := "?"
			if ent != nil {
				br = strings.Join(ent.parens.List(), "|")
			}
			byBrand[br]++
		}
	}
	fmt.Printf("master item tidak pernah jadi match: %d dari %d\n", unmatched, len(ad.masters))
	type kv struct {
		k string
		v int
	}
	list := []kv{}
	for k, v := range byBrand {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	for i, e := range list {
		if i >= 20 {
			break
		}
		fmt.Printf("  brand %-22s %d item\n", e.k, e.v)
	}
}
