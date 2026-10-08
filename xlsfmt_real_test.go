package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileAsliUser(t *testing.T) {
	roots := []string{
		filepath.Join(os.Getenv("USERPROFILE"), "Downloads"),
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop"),
	}
	exts := map[string]bool{".xlsx": true, ".xls": true, ".xlsm": true, ".csv": true}

	var files []string
	for _, r := range roots {
		entries, err := os.ReadDir(r)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if exts[strings.ToLower(filepath.Ext(e.Name()))] {
				files = append(files, filepath.Join(r, e.Name()))
			}
		}
	}
	if len(files) == 0 {
		t.Skip("tidak ada file excel di Downloads/Desktop")
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		kind, why := sniffFormat(data)
		rows, err := parseExcelBuf(data)
		note := ""
		if err != nil {
			note = "PARSE GAGAL: " + err.Error()
			if len(note) > 110 {
				note = note[:110] + "..."
			}
		} else {
			note = "parse OK"
			if len(rows) > 0 {
				s := rows[0]
				note += " | contoh brand=" + s.Brand + " nama=" + s.NamaItem
			}
		}
		t.Logf("%-42s %-10s (%s)\n     %s",
			filepath.Base(f), kind, why, note)
	}
}
