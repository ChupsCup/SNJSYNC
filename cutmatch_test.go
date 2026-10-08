package main

import (
	"fmt"
	"testing"
)

func verdictFor(plName, plSpec, mlName, mlSpec string) cutVerdict {
	return cutVerdictOf(plName, plSpec, mlName, mlSpec)
}

func TestTigaHasilTerpisah(t *testing.T) {
	for _, c := range []struct {
		pl, plSpec, ml, mlSpec string
		want                   cutOutcome
		note                   string
	}{
		{"NECK TRIM 90VL", "", "NECK TRIM 90VL", "", outMatch, "identik"},
		{"NECK TRIM 90VL", "", "NECK TRIM", "95CL", outNotMatch, "CL/VL beda"},
		{"NECK TRIM", "", "NECK TRIM 95CL", "", outReview, "CL/VL hilang di satu sisi"},
		{"SHORT LOIN", "", "STRIPLOIN", "", outNotMatch, "sudah dinyatakan berbeda"},
		{"BRISKET BONE", "", "BRISKET BONE - E", "", outNotMatch, "grade E"},
		{"BRISKET BONE - P", "", "BRISKET BONE", "", outNotMatch, "grade P"},
		{"SIRLOIN", "", "TENDERLOIN", "", outReview, "belum ada aturan"},
	} {
		v := verdictFor(c.pl, c.plSpec, c.ml, c.mlSpec)
		fmt.Printf("  %-22s vs %-22s => %-10s (%s)\n", c.pl, c.ml, v.Outcome, c.note)
		for _, r := range v.Reason {
			fmt.Printf("        %s\n", r)
		}
		if v.Outcome != c.want {
			t.Errorf("%s vs %s: dapat %s, harusnya %s", c.pl, c.ml, v.Outcome, c.want)
		}
	}
}

func TestSpesifikasiTidakPernahLolos(t *testing.T) {
	daftar := []struct{ pl, plSpec, ml, mlSpec string }{
		{"NECK TRIM 90VL", "", "NECK TRIM", "95CL"},
		{"NECK TRIM 90VL", "", "NECK TRIM", ""},
		{"NECK TRIM", "", "NECK TRIM 95CL", ""},
		{"NECK TRIM 95CL", "", "NECK TRIM 90VL", ""},
	}
	for _, c := range daftar {
		v := verdictFor(c.pl, c.plSpec, c.ml, c.mlSpec)
		if v.Outcome == outMatch {
			t.Errorf("%s/%s vs %s/%s: spesifikasi tidak boleh jadi MATCH",
				c.pl, c.plSpec, c.ml, c.mlSpec)
		}
	}
}

func TestTanpaAturanJadiReview(t *testing.T) {
	for _, c := range [][2]string{
		{"SIRLOIN", "TENDERLOIN"},
		{"ROLL", "CUBE ROLL"},
		{"SHANK", "RUMP"},
		{"TOP ROUND", "BOTTOM ROUND"},
		{"SHIN", "SILVERSIDE"},
	} {
		v := verdictFor(c[0], "", c[1], "")
		fmt.Printf("  %-14s vs %-14s => %s\n", c[0], c[1], v.Outcome)
		if v.Outcome != outReview {
			t.Errorf("%s vs %s: belum ada aturan, harus REVIEW bukan %s", c[0], c[1], v.Outcome)
		}
		if !v.UnknownKey {
			t.Errorf("%s vs %s: harus ditandai belum ada aturan", c[0], c[1])
		}
	}
}

func TestKodeBedaTetapFakta(t *testing.T) {
	v := cutVerdictAdvanced("NECK TRIM 90VL", "", "NECK TRIM 90VL", "", "NK90", "NK95")
	fmt.Printf("  kode NK90 vs NK95 (spec sama) => %s\n", v.Outcome)
	for _, r := range v.Reason {
		fmt.Printf("        %s\n", r)
	}
	if v.Outcome != outNotMatch {
		t.Errorf("kode barang beda harus NOT_MATCH, dapat %s", v.Outcome)
	}

	v2 := cutVerdictAdvanced("AYAM POTONG", "", "AYAM POTONG", "", "AY1", "SL")
	fmt.Printf("  kode AY1 vs SL (bentuk beda) => %s\n", v2.Outcome)
	if v2.Outcome == outNotMatch {
		t.Error("kode dari sistem penomoran berbeda tidak boleh jadi NOT_MATCH")
	}
}

func TestKunciSamaSpesifikBeda(t *testing.T) {
	v := verdictFor("NECK TRIM 90VL", "", "NECK TRIM", "95CL")
	if !v.SameKey {
		t.Error("harus terdeteksi nama potongan sama")
	}
	if v.Outcome != outNotMatch {
		t.Errorf("CL/VL beda harus NOT_MATCH, dapat %s", v.Outcome)
	}
	if len(v.SpecDiff) == 0 {
		t.Error("perbedaan spesifikasi harus terdeteksi")
	}
}

func TestSpesifikasiDariKolomSpec(t *testing.T) {
	v := verdictFor("NECK TRIM", "", "NECK TRIM", "95 CL")
	if v.Outcome != outReview {
		t.Errorf("spec hanya di ML harus REVIEW, dapat %s", v.Outcome)
	}
	v2 := verdictFor("NECK TRIM 90VL", "", "NECK TRIM 90VL", "")
	if v2.Outcome != outMatch {
		t.Errorf("90VL vs 90VL harus sama, dapat %s", v2.Outcome)
	}
}
