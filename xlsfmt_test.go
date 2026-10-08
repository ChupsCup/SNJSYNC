package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSniffAndParseFormat(t *testing.T) {
	dir := t.TempDir()

	htmlFile := filepath.Join(dir, "batch_sesi.xls")
	htmlBody := "\xEF\xBB\xBF<html><body><table>" +
		"<tr><th>No Container</th><th>Brand</th><th>Nama Item</th><th>ETA</th></tr>" +
		"<tr><td>1</td><td>SAPI</td><td>BEEF CHUCK &amp; RIB 1KG</td><td>12/10/2026</td></tr>" +
		"<tr><td>2</td><td>KERBAU</td><td>CHICKEN LEG 2KG</td><td>13/10/2026</td></tr>" +
		"</table></body></html>"
	if err := os.WriteFile(htmlFile, []byte(htmlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(htmlFile)
	kind, _ := sniffFormat(data)
	if kind != fmtHTML {
		t.Errorf("HTML terdeteksi sebagai %v, ingin fmtHTML", kind)
	}
	rows, err := parseExcelBuf(data)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("harus 2 baris, dapat %d", len(rows))
	}
	if rows[0].Brand != "SAPI" || rows[0].NamaItem != "BEEF CHUCK & RIB 1KG" {
		t.Errorf("baris 1 salah: brand=%q nama=%q", rows[0].Brand, rows[0].NamaItem)
	}
	if rows[1].Brand != "KERBAU" {
		t.Errorf("baris 2 salah: %q", rows[1].Brand)
	}

	if rows[0].Gabungan == "" {
		t.Error("gabungan kosong padahal brand+nama ada")
	}

	csvFile := filepath.Join(dir, "data.csv")
	csvBody := "No Container;Brand;Nama Item;ETA\n1;SAPI;BEEF SHANK;01/10/2026\n2;AYAM;AYAM POTONG;02/10/2026\n"
	if err := os.WriteFile(csvFile, []byte(csvBody), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(csvFile)
	kind, _ = sniffFormat(data)
	if kind != fmtCSV {
		t.Errorf("CSV terdeteksi sebagai %v, ingin fmtCSV", kind)
	}
	rows, err = parseExcelBuf(data)
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 2 || rows[0].Brand != "SAPI" || rows[1].NamaItem != "AYAM POTONG" {
		t.Errorf("CSV salah: %d baris, baris1=%+v", len(rows), rows)
	}

	oleFile := filepath.Join(dir, "lama.xls")
	oleHead := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	if err := os.WriteFile(oleFile, append(oleHead, make([]byte, 512)...), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(oleFile)
	kind, _ = sniffFormat(data)
	if kind != fmtOLE2 {
		t.Errorf("OLE2 terdeteksi sebagai %v, ingin fmtOLE2", kind)
	}
	if _, err = parseExcelBuf(data); err == nil {
		t.Error("OLE2 palsu (hanya magic bytes) harus error")
	} else if msg := err.Error(); !strings.Contains(msg, "Save As") &&
		!strings.Contains(msg, "xlrd") && !strings.Contains(msg, "xlwt") &&
		!strings.Contains(msg, "Compound Document") && !strings.Contains(msg, "gagal menjalankan") &&
		!strings.Contains(msg, "Version number") && !strings.Contains(msg, "Workbook") {
		t.Errorf("pesan error OLE2 harus menjelaskan penyebab atau cara memperbaikinya, dapat: %v", err)
	}

	if _, err = parseExcelBuf([]byte("halo dunia, ini bukan spreadsheets sama sekali")); err == nil {
		t.Error("file sampah harus error")
	}

	if _, err = parseExcelBuf(nil); err == nil {
		t.Error("file kosong harus error")
	}

	pedFile := filepath.Join(dir, "pedoman.xls")
	pedBody := "<html><table><tr><th>Kode Barang</th><th>Nama Barang</th><th>Merek</th></tr>" +
		"<tr><td>BRG001</td><td>BEEF CHUCK</td><td>SAPI</td></tr></table></html>"
	if err := os.WriteFile(pedFile, []byte(pedBody), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(pedFile)
	pr, err := parsePedomanBuf(data)
	if err != nil {
		t.Fatalf("parse pedoman HTML: %v", err)
	}
	if len(pr) != 1 || pr[0].KodeBarang != "BRG001" || pr[0].NamaBarang != "BEEF CHUCK" {
		t.Errorf("pedoman salah: %+v", pr)
	}
}

func TestSniffBukanTabel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("data", "photo_list.json"))
	if err != nil {
		t.Skip("photo_list.json tidak ada")
	}
	kind, why := sniffFormat(data)
	if kind == fmtXLSX {
		t.Error("JSON tidak boleh terdeteksi XLSX")
	}
	if kind == fmtUnknown && why == "" {
		t.Error("alasan harus diisi")
	}
}
