package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestDumpHasilLokal(t *testing.T) {
	mrows := readCSV(t, "prod_master.csv")
	prows := readCSV(t, "prod_packing.csv")
	drows := readCSV(t, "prod_decisions.csv")
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
	rowCache := map[string]lookupHit{}
	results := make([]compareRow, 0, len(pl))
	exist, newc, rev := 0, 0, 0
	for i := range pl {
		r := &pl[i]
		if strings.TrimSpace(r.NamaItem) == "" {
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
	f, err := os.Create("/mnt/c/Users/Ucup/AppData/Local/Temp/opencode/hasil_lokal.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := json.Marshal(results)
	f.Write(b)
	t.Logf("dump: exist=%d new=%d review=%d total=%d", exist, newc, rev, len(results))
}
