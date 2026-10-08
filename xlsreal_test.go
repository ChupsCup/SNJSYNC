package main

import (
	"os"
	"testing"
)

func TestBuildCleanedExcelXLSAsli(t *testing.T) {
	path := os.Getenv("REAL_XLS_FILE")
	if path == "" {
		t.Skip("REAL_XLS_FILE belum di-set, test file nyata dilewati")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file test tidak bisa dibaca: %v", err)
	}

	kind, why := sniffFormat(data)
	if kind != fmtOLE2 {
		t.Fatalf("file test bukan OLE2, dapat %v (%s)", kind, why)
	}

	sheets, kind, err := openSheetsOpt(data, true)
	if err != nil {
		t.Fatalf("openSheetsOpt gagal: %v", err)
	}
	if kind != fmtOLE2 {
		t.Fatalf("format tidak terdeteksi sebagai OLE2: %v", kind)
	}
	if len(sheets) == 0 {
		t.Fatal("tidak ada sheet yang terbaca")
	}
	total := 0
	for _, sh := range sheets {
		total += len(sh.Rows)
	}
	t.Logf("parser: %d sheet, %d baris, sheet[0]=%q (%d baris)",
		len(sheets), total, sheets[0].Name, len(sheets[0].Rows))
	if len(sheets[0].Rows) < 100 {
		t.Fatalf("baris terbaca terlalu sedikit: %d", len(sheets[0].Rows))
	}

	res, err := buildCleanedExcel(data, cleanOpts{})
	if err != nil {
		t.Fatalf("buildCleanedExcel gagal: %v", err)
	}
	t.Logf("cleaner: inRows=%d outList=%d", res.InRows, len(res.OutList))
	if res.InRows == 0 {
		t.Fatal("Cleaner tidak menghasilkan baris masuk")
	}
}

func TestXLSPembacaIdentik(t *testing.T) {
	path := os.Getenv("REAL_XLS_FILE")
	if path == "" {
		t.Skip("REAL_XLS_FILE belum di-set, test file nyata dilewati")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file test tidak bisa dibaca: %v", err)
	}

	if findPython() == "" || findSidecar() == "" {
		t.Skip("python/xlrd tidak tersedia, perbandingan dilewati")
	}
	if xlsHasFormulas(data) {
		t.Skip("file punya FORMULA, pembaca utama memang xlrd")
	}

	goSheets, err := readXLSGo(data)
	if err != nil {
		t.Fatalf("readXLSGo gagal: %v", err)
	}
	pySheets, err := readXLSPython(data)
	if err != nil {
		t.Fatalf("readXLSPython gagal: %v", err)
	}

	if len(goSheets) != len(pySheets) {
		t.Fatalf("jumlah sheet beda: Go=%d Python=%d", len(goSheets), len(pySheets))
	}
	diff := 0
	shown := 0
	for i := range goSheets {
		g, p := goSheets[i], pySheets[i]
		if g.Name != p.Name {
			t.Errorf("sheet %d nama beda: Go=%q Python=%q", i, g.Name, p.Name)
		}
		if len(g.Rows) != len(p.Rows) {
			t.Fatalf("sheet %q jumlah baris beda: Go=%d Python=%d", g.Name, len(g.Rows), len(p.Rows))
		}
		for r := range g.Rows {
			gr, pr := g.Rows[r], p.Rows[r]
			if len(gr) != len(pr) {
				t.Fatalf("sheet %q baris %d jumlah kolom beda: Go=%d Python=%d", g.Name, r, len(gr), len(pr))
			}
			for c := range gr {
				if gr[c].s != pr[c].s {
					diff++
					if shown < 10 {
						t.Errorf("beda di %q r%d c%d: Go=%q Python=%q", g.Name, r, c, gr[c].s, pr[c].s)
						shown++
					}
				}
			}
		}
	}
	if diff > 0 {
		t.Fatalf("total %d sel berbeda antara pure-Go dan xlrd - output modul bisa berubah!", diff)
	}
	t.Log("pure-Go dan xlrd menghasilkan SEL YANG IDENTIK")
}
