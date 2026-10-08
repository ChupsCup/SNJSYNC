package main

import (
	"fmt"
	"testing"
)

func TestParseDescKasusNyata(t *testing.T) {
	cases := []string{
		"BRISKET BONE (CEDAR MEATS)  --- EST.206 AU",
		"BRISKET BONE - E (SWIFT)  --- EST.206 AU",
		"BRISKET BONE-P (SWIFT)  --- EST.206 AU",
		"BEEF TONGUE (SWIFT)  --- EST.969 US",
		"TONGUE ROOT (SWIFT)  --- 90VL --- EST.969 US",
		"NECK TRIM 90VL (SWIFT)",
		"BULK 95CL",
		"50CL",
		"STRIPLOIN (SWIFT) ---0X1 S/T USDA PRIME--- EST.969 US",
		"TENDERLOIN LOKAL",
		"RIBEYE A",
	}
	for _, c := range cases {
		p := parseDesc(c)
		key := normCutName(p.Name)
		base, suf := cutKeys(p.Name)
		fmt.Printf("%-50s\n", trunc(c, 50))
		fmt.Printf("    nama=%-24q brand=%-14q spec=%-20q country=%q\n", p.Name, p.Brand, p.Spec, p.Country)
		fmt.Printf("    key=%-20q base=%-18q suffix=%-4q specCL/VL=%v family=%q\n",
			key, base, suf, cutSpecKeys(p.Name), cutFamily(key))
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "~"
}

func TestKeluargaBrisket(t *testing.T) {
	plain := normCutName("BRISKET BONE")
	e := normCutName("BRISKET BONE - E")
	p := normCutName("BRISKET BONE-P")
	b1, s1 := cutKeys(plain)
	b2, s2 := cutKeys(e)
	b3, s3 := cutKeys(p)
	fmt.Printf("BRISKET BONE    key=%-14q base=%-14q suffix=%-3q family=%q\n", plain, b1, s1, cutFamily(plain))
	fmt.Printf("BRISKET BONE - E key=%-14q base=%-14q suffix=%-3q family=%q\n", e, b2, s2, cutFamily(e))
	fmt.Printf("BRISKET BONE-P  key=%-14q base=%-14q suffix=%-3q family=%q\n", p, b3, s3, cutFamily(p))

	if b1 != b2 || b1 != b3 {
		t.Errorf("base harus sama, dapat %q/%q/%q", b1, b2, b3)
	}
	if s1 != "" || s2 != "E" || s3 != "P" {
		t.Errorf("suffix salah: %q/%q/%q", s1, s2, s3)
	}
	if f1, f2, f3 := cutFamily(plain), cutFamily(e), cutFamily(p); f1 != "brisket" || f2 != "brisket" || f3 != "brisket" {
		t.Errorf("family harus brisket: %q %q %q", f1, f2, f3)
	}
	if plain == e || e == p {
		t.Errorf("kunci dengan suffix beda harus tetap berbeda")
	}
}

func TestSuffixTidakMenebak(t *testing.T) {
	for _, n := range []string{"TENDERLOIN", "STRIPLOIN", "SHORTLOIN", "RIBEYE", "CARCASS"} {
		b, s := cutKeys(n)
		if s != "" {
			t.Errorf("%q dipotong jadi %q + %q", n, b, s)
		}
		if b != normCutName(n) {
			t.Errorf("%q base berubah: %q != %q", n, b, normCutName(n))
		}
	}

	b, s := cutKeys("MISCELLANEOUSXYZ")
	if s != "" || b != "MISCELLANEOUSXYZ" {
		t.Errorf("nama tak dikenal harus utuh: %q %q", b, s)
	}
}

func TestPrefixSpesiesDiabaikan(t *testing.T) {
	beef := normCutName("BEEF TONGUE")
	plain := normCutName("TONGUE")
	fmt.Printf("BEEF TONGUE -> %q\nTONGUE      -> %q\n", beef, plain)
	if beef != plain {
		t.Errorf("prefix spesies harus diabaikan: %q != %q", beef, plain)
	}
	rf := normCutName("BEEF RIB FINGER MEAT")
	rp := normCutName("RIB FINGER")
	fmt.Printf("BEEF RIB FINGER MEAT -> %q\nRIB FINGER           -> %q\n", rf, rp)
	if rf != rp {
		t.Errorf("kata MEAT di ujung harus dilepas: %q != %q", rf, rp)
	}
	tl := normCutName("TENDERLOIN LOKAL")
	if tl != "TENDERLOIN" {
		t.Errorf("LOKAL harus dilepas: %q", tl)
	}
}

func TestCLVLJadiSpesifikasi(t *testing.T) {
	for _, c := range [][2]string{
		{"NECK TRIM 90VL", "NECKTRIM"},
		{"NECK TRIM 90 VL", "NECKTRIM"},
		{"BULK 95CL", "BULK"},
		{"BRISKET BONE 85CL", "BRISKETBONE"},
	} {
		got := normCutName(c[0])
		fmt.Printf("%-22q -> %-14q (spec: %v)\n", c[0], got, cutSpecKeys(c[0]))
		if got != c[1] {
			t.Errorf("%q: dapat %q, mau %q", c[0], got, c[1])
		}
		if len(cutSpecKeys(c[0])) == 0 {
			t.Errorf("%q: spesifikasi CL/VL hilang", c[0])
		}
	}
	if s := cutSpecKeys("50CL"); len(s) != 1 || s[0] != "50CL" {
		t.Errorf("50CL spec = %v", s)
	}
	if normCutName("50CL") != "" {
		t.Errorf("50CL tidak punya nama potongan, dapat %q", normCutName("50CL"))
	}
}

func TestShortloinVsStriploin(t *testing.T) {
	sl := normCutName("SHORT LOIN")
	st := normCutName("STRIPLOIN")
	fmt.Printf("SHORT LOIN -> %q family=%q\nSTRIPLOIN  -> %q family=%q\n", sl, cutFamily(sl), st, cutFamily(st))
	if sl == st {
		t.Errorf("potongan harus beda: %q vs %q", sl, st)
	}
	if cutFamily(sl) != "loin" || cutFamily(st) != "loin" {
		t.Errorf("family harus sama-sama loin: %q %q", cutFamily(sl), cutFamily(st))
	}
	if !cutKnownKey(sl) || !cutKnownKey(st) {
		t.Errorf("keduanya harus dikenal sebagai potongan")
	}
}
