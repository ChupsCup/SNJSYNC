package main

import "testing"

func TestBedaDeskripsi(t *testing.T) {
	cases := []struct {
		name string
		lama string
		baru string
		want bool
	}{
		{"revisi minor (tambah huruf)",
			"CARCASS (CEDAR MEAT) --- VEAL --- EST.206 AU",
			"CARCASS (CEDAR MEATS) --- VEAL --- EST.206 AU", true},
		{"teks sama",
			"CARCASS (CEDAR MEATS) --- VEAL --- EST.206 AU",
			"CARCASS (CEDAR MEATS) --- VEAL --- EST.206 AU", false},
		{"beda spasi ujung bukan revisi",
			" CARCASS (CEDAR MEAT) ", "CARCASS (CEDAR MEAT)", false},
		{"kosong jadi terisi", "", "CARCASS (CEDAR MEAT)", true},
		{"terisi jadi kosong", "CARCASS (CEDAR MEAT)", "", true},
		{"dua-duanya kosong", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bedaDeskripsi(c.lama, c.baru); got != c.want {
				t.Errorf("bedaDeskripsi(%q, %q) = %v, mau %v", c.lama, c.baru, got, c.want)
			}
		})
	}
}

func TestRevisiValues(t *testing.T) {
	cases := []struct {
		kolom, baris int
		want         string
	}{
		{2, 3, "($1,$2),($3,$4),($5,$6)"},
		{5, 1, "($1,$2,$3,$4,$5)"},
		{5, 2, "($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)"},
		{1, 1, "($1)"},
		{5, 0, ""},
	}
	for _, c := range cases {
		if got := revisiValues(c.kolom, c.baris); got != c.want {
			t.Errorf("revisiValues(%d, %d) = %q, mau %q", c.kolom, c.baris, got, c.want)
		}
	}
}
