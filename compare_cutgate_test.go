package main

import (
	"fmt"
	"testing"
)

func mkEngine() *compareEngine {
	e := newCompareEngine()
	ml := []mlItem{
		{ID: 1, Brand: "AOF", NamaItem: "NECK TRIM", ItemCode: "NK90", ItemDesc: "NECK TRIM --- 90VL --- EST.969 US"},
		{ID: 2, Brand: "AOF", NamaItem: "NECK TRIM", ItemCode: "NK95", ItemDesc: "NECK TRIM --- 95CL --- EST.969 US"},
		{ID: 3, Brand: "AOF", NamaItem: "SHORT LOIN", ItemCode: "SL", ItemDesc: "SHORT LOIN (AOF) --- EST.402 US"},
		{ID: 4, Brand: "AOF", NamaItem: "STRIPLOIN", ItemCode: "ST", ItemDesc: "STRIPLOIN (AOF) --- EST.402 US"},
		{ID: 5, Brand: "AOF", NamaItem: "SIRLOIN", ItemCode: "SR", ItemDesc: "SIRLOIN (AOF) --- EST.402 US"},
	}
	e.build(ml, map[string]*reviewDecision{})
	return e
}

func plRow(nama, kode string) *plItem {
	return &plItem{NamaItem: nama, ItemCode: kode, Brand: "AOF"}
}

func TestCutGateShortLoinVsStriploin(t *testing.T) {
	e := mkEngine()
	hit := e.cutGate(plRow("SHORT LOIN", ""), e.entries[3], "SHORT LOIN", newSet(), nil)
	if hit == nil {
		t.Fatal("SHORT LOIN vs STRIPLOIN sudah dinyatakan berbeda, harus NOT_MATCH")
	}
	fmt.Printf("  SHORT LOIN vs STRIPLOIN -> %-6s review=%-5v %s\n", hit.status, hit.review, hit.note)
	if hit.review {
		t.Error("tidak boleh masuk review, ini fakta yang sudah dinyatakan")
	}
}

func TestCutGateTanpaAturanHarusLolos(t *testing.T) {
	e := mkEngine()
	for _, c := range [][2]string{{"SIRLOIN", "TENDERLOIN"}, {"AYAM POTONG", "SHORT LOIN"}} {
		hit := e.cutGate(plRow(c[0], ""), e.entries[2], c[0], newSet(), nil)
		fmt.Printf("  %-14s vs kandidat -> %v\n", c[0], hit)
		if hit != nil {
			t.Errorf("%s: belum ada aturan, gerbang harusThicknessnya nihil", c[0])
		}
	}
}

func TestCutGateCLVL(t *testing.T) {
	e := mkEngine()

	hit := e.cutGate(plRow("NECK TRIM 90VL", "NK90"), e.entries[1], "NECK TRIM 90VL", newSetFromSlice([]string{"90VL"}), nil)
	if hit == nil {
		t.Fatal("CL/VL beda harus ada keputusan")
	}
	fmt.Printf("  90VL vs 95CL -> %-6s review=%-5v %s\n", hit.status, hit.review, hit.note)
	if hit.status != "NEW" || hit.review {
		t.Errorf("CL/VL beda harus NOT_MATCH tanpa review, dapat status=%s review=%v", hit.status, hit.review)
	}

	hit2 := e.cutGate(plRow("NECK TRIM", "NK90"), e.entries[0], "NECK TRIM", newSet(), nil)
	fmt.Printf("  (tanpa spec) vs 90VL -> %-6s review=%-5v %s\n", hit2.status, hit2.review, hit2.note)
	if hit2 == nil || !hit2.review {
		t.Error("CL/VL hilang di satu sisi harus REVIEW")
	}
}
