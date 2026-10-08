package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestXLSNumFlagSama(t *testing.T) {
	path := os.Getenv("REAL_XLS_FILE")
	if path == "" {
		t.Skip("REAL_XLS_FILE belum di-set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if findPython() == "" || findSidecar() == "" {
		t.Skip("python/xlrd tidak tersedia")
	}
	goSheets, err := readXLSGo(data)
	if err != nil {
		t.Fatalf("readXLSGo: %v", err)
	}
	pySheets, err := readXLSPython(data)
	if err != nil {
		t.Fatalf("readXLSPython: %v", err)
	}

	numGo, numPy, sDiff, numDiff, cells := 0, 0, 0, 0, 0
	for i := range goSheets {
		g, p := goSheets[i], pySheets[i]
		for r := range g.Rows {
			if r >= len(p.Rows) {
				break
			}
			for c := range g.Rows[r] {
				if c >= len(p.Rows[r]) {
					break
				}
				gc, pc := g.Rows[r][c], p.Rows[r][c]
				cells++
				if gc.s != pc.s {
					sDiff++
				}
				if gc.num {
					numGo++
				}
				if pc.num {
					numPy++
				}
				if gc.num != pc.num {
					numDiff++
					if numDiff <= 8 {
						t.Errorf("flag num beda di %q r%d c%d: go=%v (%q) py=%v (%q)",
							g.Name, r, c, gc.num, gc.s, pc.num, pc.s)
					}
				}
			}
		}
	}
	fmt.Printf("sel diperiksa      : %d\n", cells)
	fmt.Printf("beda teks          : %d\n", sDiff)
	fmt.Printf("beda flag num      : %d\n", numDiff)
	fmt.Printf("sel numerik go=%d py=%d\n", numGo, numPy)
	if numDiff > 0 {
		t.Fatalf("%d sel punya flag num berbeda - tampilan UI bisa berubah", numDiff)
	}
}

func TestCleanerOutputIdentik(t *testing.T) {
	path := os.Getenv("REAL_XLS_FILE")
	if path == "" {
		t.Skip("REAL_XLS_FILE belum di-set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if findPython() == "" || findSidecar() == "" {
		t.Skip("python/xlrd tidak tersedia")
	}

	digest := func(sheets []sheetData) (rows, cols int, nonEmpty int, sample string) {
		for _, sh := range sheets {
			for _, r := range sh.Rows {
				rows++
				if len(r) > cols {
					cols = len(r)
				}
				for _, c := range r {
					if strings.TrimSpace(c.s) != "" {
						nonEmpty++
					}
				}
			}
			if sample == "" && len(sh.Rows) > 2 {
				sample = sh.Name
			}
		}
		return
	}

	opts := cleanOpts{RemoveBlank: true, RemoveEmpty: true}
	_ = opts

	os.Setenv("XLS_READER", "go")
	goClean, errGo := buildCleanedExcel(data, cleanOpts{RemoveBlank: true, RemoveEmpty: true})
	os.Setenv("XLS_READER", "python")
	pyClean, errPy := buildCleanedExcel(data, cleanOpts{RemoveBlank: true, RemoveEmpty: true})
	os.Unsetenv("XLS_READER")

	if errGo != nil {
		t.Fatalf("Cleaner via Go gagal: %v", errGo)
	}
	if errPy != nil {
		t.Fatalf("Cleaner via Python gagal: %v", errPy)
	}

	fmt.Printf("Cleaner pure-Go : inRows=%d outList=%d dashIn=%d\n",
		goClean.InRows, len(goClean.OutList), goClean.DashIn)
	fmt.Printf("Cleaner xlrd    : inRows=%d outList=%d dashIn=%d\n",
		pyClean.InRows, len(pyClean.OutList), pyClean.DashIn)

	if goClean.InRows != pyClean.InRows {
		t.Errorf("InRows beda: Go=%d Python=%d", goClean.InRows, pyClean.InRows)
	}
	if len(goClean.OutList) != len(pyClean.OutList) {
		t.Errorf("OutList beda: Go=%d Python=%d", len(goClean.OutList), len(pyClean.OutList))
	}
	if goClean.DashIn != pyClean.DashIn {
		t.Errorf("DashIn beda: Go=%d Python=%d", goClean.DashIn, pyClean.DashIn)
	}
	n := len(goClean.OutList)
	if len(pyClean.OutList) < n {
		n = len(pyClean.OutList)
	}
	diff := 0
	for i := 0; i < n; i++ {
		a, b := goClean.OutList[i], pyClean.OutList[i]
		if a != b {
			diff++
			if diff <= 8 {
				t.Errorf("OutList[%d] beda:\n  go=%+v\n  py=%+v", i, a, b)
			}
		}
	}
	if diff > 0 {
		t.Fatalf("%d entri OutList berbeda", diff)
	}

	gr, gc, gne, _ := digest(mustGo(t, data))
	pr, pc, pne, _ := digest(mustPy(t, data))
	fmt.Printf("sel yang terbaca : go=%d baris/%d kolom/%d terisi | py=%d baris/%d kolom/%d terisi\n",
		gr, gc, gne, pr, pc, pne)
	if gr != pr || gc != pc || gne != pne {
		t.Fatalf("bacaan data beda: go=%d/%d/%d py=%d/%d/%d", gr, gc, gne, pr, pc, pne)
	}
}

func mustGo(t *testing.T, data []byte) []sheetData {
	t.Helper()
	s, err := readXLSGo(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustPy(t *testing.T, data []byte) []sheetData {
	t.Helper()
	s, err := readXLSPython(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
