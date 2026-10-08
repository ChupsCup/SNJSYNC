package main

import (
	"strings"
	"testing"
)

func TestSearchRustamSliceFQ11(t *testing.T) {
	mockEntries := []PhotoEntry{
		{
			ID:   "rustam-fq11-1",
			Name: "img1.jpg",
			Dir:  "KERBAU/KERBAU 2024/3. KERBAU - PPI/4. RUSTAM/RUSTAM BUFFALO FQ-11 @18 KG - PPI",
			Text: "FOREQUARTER/PROSOT DEPAN FQ11\nBERAT BERSIH:900 Gram\nRUSTAM FOODS PRIVATE LIMITED",
		},
		{
			ID:   "rustam-fq18-1",
			Name: "img2.jpg",
			Dir:  "KERBAU/KERBAU 2024/3. KERBAU - PPI/4. RUSTAM/RUSTAM BUFFALO FQ-18 - PPI",
			Text: "RUSTAM FOODS PVT.LTD\nFQ18\nBUFFALO MEAT",
		},
		{
			ID:   "allana-fq11-1",
			Name: "img3.jpg",
			Dir:  "KERBAU/KERBAU 2026/1. BERDIKARI/ALLANA/ALLANA - BUFFALO FQ-11 @20 KG (TRAYS SLICE 1KG) - BKR",
			Text: "ALLANA BUFFALO FQ-11 TRAYS SLICE 1KG",
		},
	}

	app := &App{
		universe: mockEntries,
	}
	app.resetSearchIndex()

	resp := app.search("SLICE (RUSTAM)   --- FQ11")
	if len(resp.Results) != 1 {
		t.Fatalf("ekspektasi 1 hasil exact (Rustam FQ-11), dapat: %d", len(resp.Results))
	}
	if resp.Results[0].ID != "rustam-fq11-1" {
		t.Errorf("hasil exact bukan Rustam FQ-11: ID = %s", resp.Results[0].ID)
	}

	for _, p := range resp.Partial {
		if strings.Contains(p.Dir, "FQ-18") {
			t.Errorf("FQ-18 tidak boleh cocok dengan FQ-11!")
		}
	}
}

func TestSearchCodeNormalization(t *testing.T) {
	mockEntries := []PhotoEntry{
		{
			ID:   "topside-hq41",
			Name: "img.jpg",
			Dir:  "120080-14.146IN.03827 TOPSIDE (BLACK GEMS)---HQ41 PPI---EST. IN",
			Text: "EST. 132",
		},
	}

	app := &App{
		universe: mockEntries,
	}

	queries := []string{"HQ41", "HQ-41", "hq 41", "TOPSIDE HQ41", "TOPSIDE (BLACK GEMS) --- HQ41"}
	for _, q := range queries {
		app.resetSearchIndex()
		resp := app.search(q)
		if len(resp.Results) == 0 {
			t.Errorf("query %q harusnya menemukan TOPSIDE HQ41 sebagai exact match, dapat: %d", q, len(resp.Results))
		}
	}
}

func TestNoFuzzyCodeMutation(t *testing.T) {

	s := newSet()
	s.Add("hq41")
	s.Add("hq42")
	fz := map[string]*strSet{"h|4": s}
	cands := fuzzyCands(fz, "hq41")
	if len(cands) != 0 {
		t.Errorf("fuzzyCands untuk kode alfanumerik 'hq41' harus mengembalikan nil/kosong, dapat: %v", cands)
	}
}
