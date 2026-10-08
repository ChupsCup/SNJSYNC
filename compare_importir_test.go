package main

import "testing"

func TestImportirDariNamaFilePL(t *testing.T) {
	cases := []struct {
		name string
		row  plItem
		want string
	}{
		{"file berdikari", plItem{File: "PL BERDIKARI Oktober.xlsx"}, "BKR"},
		{"file bkr", plItem{File: "packing list bkr 2026.xlsx"}, "BKR"},
		{"file ppi", plItem{File: "PL PPI 07-10.xlsx"}, "PPI"},
		{"file polos, sheet berdikari", plItem{Sheet: "BERDIKARI"}, "BKR"},
		{"kolom importir terisi", plItem{Importir: "PT BERDIKARI DAGING NUSANTARA"}, "BKR"},
		{"file menang atas kolom", plItem{File: "PL berdikari.xlsx", Importir: "PPI"}, "BKR"},
		{"deskripsi menyebut ppi", plItem{ItemDesc: "SLICE BEEF @18KG PPI"}, "PPI"},
		{"tanpa sinyal sama sekali", plItem{File: "PL jaya abadi.xlsx"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.row.importirPL(c.row.ItemDesc, c.row.Gabungan)
			if got != c.want {
				t.Errorf("importirPL() = %q, mau %q (row=%+v)", got, c.want, c.row)
			}
		})
	}
}
