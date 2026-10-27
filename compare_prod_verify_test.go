package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const prodDir = "/mnt/c/Users/Ucup/AppData/Local/Temp/opencode/"

func readCSV(t *testing.T, name string) [][]string {
	f, err := os.Open(prodDir + name)
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

func TestProduksiLokalVsTersimpan(t *testing.T) {
	mrows := readCSV(t, "prod_master.csv")
	prows := readCSV(t, "prod_packing.csv")
	drows := readCSV(t, "prod_decisions.csv")
	rrows := readCSV(t, "prod_results.csv")

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
			Gabungan: r[9], Importir: r[10], File: "KC KERBAU DAN SAPI PPI 2026 Menarik 20Oct - Copy.xlsx"})
	}

	var stored []compareRow
	if err := json.Unmarshal([]byte(rrows[1][7]), &stored); err != nil {
		t.Fatalf("parse results_json: %v", err)
	}
	t.Logf("master=%d packing=%d decisions=%d stored_results=%d", len(masters), len(pl), len(decisions), len(stored))
	t.Logf("tersimpan: exist=%s new=%s review=%s total=%s", rrows[1][4], rrows[1][5], rrows[1][6], rrows[1][3])

	eng := newCompareEngine()
	eng.build(masters, decisions)

	rowCache := map[string]lookupHit{}
	results := make([]compareRow, 0, len(pl))
	exist, newc, rev, skipped := 0, 0, 0, 0
	for i := range pl {
		r := &pl[i]
		if strings.TrimSpace(r.NamaItem) == "" {
			skipped++
			continue
		}
		sig := normStr(r.Brand + "|" + r.NamaItem + "|" + r.ItemDesc + "|" + r.Gabungan +
			"|" + r.Sheet + "|" + r.Importir + "|" + r.File)
		hit, cached := rowCache[sig]
		if !cached {
			hit = eng.lookupMaster(r)
			rowCache[sig] = hit
		}
		flag := "NEW ITEM"
		if hit.status == "EXISTING" {
			flag = "EXISTING"
		}
		if flag == "EXISTING" {
			exist++
		} else {
			newc++
		}
		if hit.review {
			rev++
		}
		mc, md := "-", "-"
		if hit.m != nil {
			mc = orDash(hit.m.ItemCode)
			md = orName(hit.m.ItemDesc, hit.m.NamaItem)
		} else if hit.cand != nil {
			mc = orDash(hit.cand.ItemCode)
			md = orName(hit.cand.ItemDesc, hit.cand.NamaItem)
		}
		results = append(results, compareRow{
			No: len(results) + 1, NoContainer: r.NoContainer, Brand: r.Brand, NamaItem: r.NamaItem,
			ETA: r.ETA, ItemCode: r.ItemCode, ItemDesc: r.ItemDesc, Sheet: r.Sheet,
			Importir: r.importirPL(r.ItemDesc, r.Gabungan), Flag: flag, NeedsReview: hit.review,
			Note: hit.note, Score: round2(hit.sym), MasterCode: mc, MasterDesc: md,
		})
	}
	t.Logf("BARU : exist=%d new=%d review=%d total=%d skipped=%d", exist, newc, rev, len(results), skipped)

	if len(results) != len(stored) {
		t.Fatalf("jumlah baris beda: baru=%d tersimpan=%d", len(results), len(stored))
	}

	changed := map[string]int{}
	shown := 0
	trans := map[string]int{}
	for i := range results {
		o, n := stored[i], results[i]
		if o.Flag == n.Flag && o.MasterCode == n.MasterCode {
			continue
		}
		tk := o.Flag + " -> " + n.Flag
		if o.NeedsReview != n.NeedsReview {
			tk += " (review " + boolStr(o.NeedsReview) + "->" + boolStr(n.NeedsReview) + ")"
		}
		trans[tk]++
		key := o.Brand + " / " + o.NamaItem
		changed[key]++
		if shown < 40 {
			fmt.Printf("  %-8s %-26s %-10s -> %-10s  %s => %s\n", o.Sheet, key, o.Flag, n.Flag, o.MasterCode, n.MasterCode)
			shown++
		}
	}
	fmt.Printf("\nTOTAL BARIS BERUBAH: %d\n", countChanged(results, stored))
	fmt.Println("TRANSISI:")
	tkeys := []string{}
	for k := range trans {
		tkeys = append(tkeys, k)
	}
	sort.Slice(tkeys, func(i, j int) bool { return trans[tkeys[i]] > trans[tkeys[j]] })
	for _, k := range tkeys {
		fmt.Printf("  %5d  %s\n", trans[k], k)
	}
	fmt.Println("RINCIAN PER ITEM:")
	for k, v := range changed {
		fmt.Printf("  %-40s %d baris\n", k, v)
	}
}

func boolStr(b bool) string {
	if b {
		return "Y"
	}
	return "T"
}

func countChanged(new, old []compareRow) int {
	n := 0
	for i := range new {
		if new[i].Flag != old[i].Flag || new[i].MasterCode != old[i].MasterCode {
			n++
		}
	}
	return n
}
