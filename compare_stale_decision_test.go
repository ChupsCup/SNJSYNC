package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestKeputusanManualBasiDiabaikan(t *testing.T) {
	masters := []mlItem{
		{ID: 1, ItemCode: "120080-14.076IN.00941", ItemDesc: "TOPSIDE (AOV) --- HQ41 BKR --- EST. IN"},
		{ID: 2, ItemCode: "120080-14.076IN.04110", ItemDesc: "TOPSIDE (AOV) ---HQ41 PPI--- EST. IN"},
		{ID: 3, ItemCode: "300190-14.084IN.02585", ItemDesc: "FQ ROLL (RUSTAM) --- FQ106 BKR --- EST.000 IN"},
		{ID: 4, ItemCode: "300190-14.079IN.03432", ItemDesc: "FQ ROLL (IAF) ---FQ105 PPI--- EST. IN"},
	}
	dec := map[string]*reviewDecision{
		"aov|hq-41 topside": {Key: "aov|hq-41 topside", Decision: "new"},
		"rustam|fq 105 roll": {Key: "rustam|fq 105 roll", Decision: "new"},
	}
	eng := newCompareEngine()
	eng.build(masters, dec)

	hit := eng.lookupMaster(&plItem{
		Sheet: "KERBAU PPI 2026", Brand: "AOV", NamaItem: "HQ-41 Topside", Gabungan: "AOV HQ-41 Topside",
	})
	if hit.status != "EXISTING" {
		t.Fatalf("keputusan 'new' basi harus diabaikan, dapat status=%s note=%q", hit.status, hit.note)
	}
	if hit.m == nil || hit.m.ItemCode != "120080-14.076IN.04110" {
		t.Fatalf("harus ke kode 120080-14.076IN.04110, dapat %+v", hit.m)
	}

	hit2 := eng.lookupMaster(&plItem{
		Sheet: "KERBAU PPI 2026", Brand: "RUSTAM", NamaItem: "FQ 105 Roll", Gabungan: "RUSTAM FQ 105 Roll",
	})
	if hit2.status != "NEW" {
		t.Fatalf("keputusan 'new' yang benar harus tetap NEW, dapat status=%s note=%q", hit2.status, hit2.note)
	}
}

func TestAOVDenganMasterAsli(t *testing.T) {
	path := "/mnt/c/Users/Ucup/AppData/Local/Temp/opencode/master_dump.json"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("master dump tidak ada: %v", err)
	}
	var rows []struct {
		ID        int    `json:"id"`
		ItemCode  string `json:"item_code"`
		Deskripsi string `json:"deskripsi"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("parse dump: %v", err)
	}
	masters := make([]mlItem, 0, len(rows))
	for _, r := range rows {
		masters = append(masters, mlItem{ID: r.ID, ItemCode: r.ItemCode, ItemDesc: r.Deskripsi})
	}
	t.Logf("master items: %d", len(masters))

	dec := map[string]*reviewDecision{}
	for _, k := range []string{
		"aov|fq-106 fq roll", "aov|hq-41 topside", "aov|hq-42 thick flank",
		"aov|hq-44 silverside", "aov|hq-45 rump steak",
	} {
		dec[k] = &reviewDecision{Key: k, Decision: "new"}
	}
	eng := newCompareEngine()
	eng.build(masters, dec)

	cases := []struct {
		brand, nama, kode string
	}{
		{"AOV", "HQ-41 Topside", "120080-14.076IN.04110"},
		{"AOV", "HQ-42 Thick Flank", "140030-14.076IN.04112"},
		{"AOV", "HQ-44 Silverside", "120070-14.076IN.04111"},
		{"AOV", "HQ-45 Rump Steak", "120110-14.076IN.04113"},
		{"AOV", "FQ-106 FQ Roll", "100020-14.076IN.04142"},
	}
	for _, c := range cases {
		hit := eng.lookupMaster(&plItem{
			Sheet: "KERBAU PPI 2026", Brand: c.brand, NamaItem: c.nama,
			Gabungan: c.brand + " " + c.nama,
		})
		if hit.status != "EXISTING" || hit.m == nil || hit.m.ItemCode != c.kode {
			t.Errorf("%s / %s: dapat status=%s kode=%v (harus EXISTING %s) note=%q",
				c.brand, c.nama, hit.status, hit.m, c.kode, hit.note)
			continue
		}
		fmt.Printf("  OK %-6s %-16s -> %s\n", c.brand, c.nama, hit.m.ItemCode)
	}
}
