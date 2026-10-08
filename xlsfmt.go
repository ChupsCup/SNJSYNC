package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type bookFormat int

const (
	fmtUnknown bookFormat = iota
	fmtXLSX
	fmtHTML
	fmtCSV
	fmtOLE2
)

func (f bookFormat) String() string {
	switch f {
	case fmtXLSX:
		return "Excel .xlsx (OOXML)"
	case fmtHTML:
		return "tabel HTML (.xls/.xlsx hasil export, bukan Excel asli)"
	case fmtCSV:
		return "CSV"
	case fmtOLE2:
		return "Excel .xls lama / file terenkripsi"
	}
	return "format tidak dikenali"
}

func sniffFormat(data []byte) (bookFormat, string) {
	b := stripBOM(data)
	if len(b) == 0 {
		return fmtUnknown, "file kosong (0 byte)"
	}

	if len(b) >= 8 && bytes.Equal(b[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return fmtOLE2, "file berformat OLE2 (komponen Compound File Binary)"
	}

	if len(b) >= 4 && bytes.Equal(b[:4], []byte{'P', 'K', 0x03, 0x04}) {
		return fmtXLSX, "arsip ZIP/OOXML"
	}

	txt := decodeText(data)
	head := txt
	if len(head) > 8192 {
		head = head[:8192]
	}
	lt := strings.ToLower(head)
	if strings.Contains(lt, "<html") || strings.Contains(lt, "<table") ||
		strings.Contains(lt, "<!doctype") || strings.Contains(lt, "<meta") {
		return fmtHTML, "berisi markup HTML"
	}
	if strings.HasPrefix(strings.TrimSpace(lt), "{") && strings.Contains(lt, "\"") {
		return fmtUnknown, "berisi JSON, bukan tabel Excel"
	}
	if isDelimitedText(head) {
		return fmtCSV, "teks delimited"
	}
	return fmtUnknown, "tidak ditemukan signature ZIP, HTML, atau teks delimited"
}

func decodeText(data []byte) string {
	b := stripBOM(data)
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		r := make([]rune, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			r = append(r, rune(uint16(b[i])|uint16(b[i+1])<<8))
		}
		return string(r)
	}
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		r := make([]rune, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			r = append(r, rune(uint16(b[i+1])|uint16(b[i])<<8))
		}
		return string(r)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

var delims = []string{";", ",", "\t", "|"}

func isDelimitedText(head string) bool {
	var lines []string
	for _, l := range strings.Split(head, "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
		if len(lines) >= 5 {
			break
		}
	}
	if len(lines) < 2 {
		return false
	}
	for _, d := range delims {
		hits := 0
		for _, l := range lines {
			if strings.Contains(l, d) {
				hits++
			}
		}
		if hits == len(lines) {
			return true
		}
	}
	return false
}

func sniffDelimiter(lines []string) rune {
	best, bestScore := rune(0), -1
	for _, d := range delims {
		score := 0
		for _, l := range lines {
			score += strings.Count(l, d)
		}
		if score > bestScore {
			best, bestScore = []rune(d)[0], score
		}
	}
	if best == 0 {
		best = ','
	}
	return best
}

func diagnoseZip(data []byte) string {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return " (zip-nya sendiri tidak bisa dibuka: " + err.Error() + ")"
	}
	names := make([]string, 0, len(zr.File))
	has := func(sub string) bool {
		for _, f := range zr.File {
			if strings.Contains(f.Name, sub) {
				return true
			}
		}
		return false
	}
	for i, f := range zr.File {
		if i >= 12 {
			names = append(names, "...")
			break
		}
		names = append(names, f.Name)
	}
	info := "  Isi file (maks. 12 entri): " + strings.Join(names, ", ")
	switch {
	case has("xl/workbook.bin"):
		info += "\n  -> Ini file .xlsb (Excel Binary Workbook, format 2007). Buka di Excel lalu Save As .xlsx."
	case has("xl/workbook.xml"):
		info += "\n  -> Struktur xlsx terdeteksi tapi excelize menolaknya. Kemungkinan file korup/terhenti saat disimpan, atau dienkripsi. Coba buka di Excel lalu Save As ulang (ganti nama file)."
	case has("word/document.xml"), has("ppt/"):
		info += "\n  -> Ini dokumen Word/PowerPoint, bukan Excel."
	case has("[Content_Types].xml"):
		info += "\n  -> File OOXML tapi bukan workbook Excel."
	default:
		info += "\n  -> Ini file ZIP biasa (bukan Excel). Jangan di-rename jadi .xlsx; ekstrak dulu isinya."
	}
	return info
}

type sheetData struct {
	Name string
	Rows [][]cellVal
}

func openSheets(data []byte) ([]sheetData, bookFormat, error) {
	return openSheetsOpt(data, false)
}

func openSheetsOpt(data []byte, keepBlank bool) ([]sheetData, bookFormat, error) {
	kind, why := sniffFormat(data)
	switch kind {
	case fmtXLSX:
		f, err := excelize.OpenReader(bytes.NewReader(stripBOM(data)))
		if err != nil {
			return nil, kind, fmt.Errorf("xlsx tidak bisa dibaca: %v\n%s", err, diagnoseZip(data))
		}
		defer f.Close()
		var out []sheetData
		for _, name := range f.GetSheetList() {
			if rows := sheetRows(f, name, keepBlank); len(rows) > 0 {
				out = append(out, sheetData{Name: name, Rows: rows})
			}
		}
		return out, kind, nil

	case fmtHTML:
		sheets, err := parseHTMLSheets(data)
		if err != nil {
			return nil, kind, err
		}
		return sheets, kind, nil

	case fmtCSV:
		rows, err := parseCSVSheet(data)
		if err != nil {
			return nil, kind, err
		}
		return []sheetData{{Name: "CSV", Rows: rows}}, kind, nil

	case fmtOLE2:

		sheets, err := readXLS(data)
		if err != nil {
			return nil, kind, err
		}
		if !keepBlank {
			var kept []sheetData
			for _, sh := range sheets {
				if rows := dropBlankRows(sh.Rows); len(rows) > 0 {
					sh.Rows = rows
					kept = append(kept, sh)
				}
			}
			sheets = kept
		}
		return sheets, kind, nil

	default:
		return nil, kind, fmt.Errorf(
			"format file tidak dikenali (%s). Isi file %d byte. Hanya menerima .xlsx, .xls, .csv, "+
				"atau tabel HTML hasil export.",
			why, len(data))
	}
}

func dropBlankRows(rows [][]cellVal) [][]cellVal {
	out := make([][]cellVal, 0, len(rows))
	for _, r := range rows {
		empty := true
		for _, c := range r {
			if strings.TrimSpace(c.s) != "" {
				empty = false
				break
			}
		}
		if !empty {
			out = append(out, r)
		}
	}
	return out
}

var (
	reHTMLTable = regexp.MustCompile(`(?is)<table[^>]*>(.*?)</table>`)
	reHTMLRow   = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	reHTMLCell  = regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]>`)
	reHTMLTag   = regexp.MustCompile(`(?is)<[^>]+>`)
	reHTMLBrk   = regexp.MustCompile(`(?is)<\s*br\s*/?\s*>`)
	reWSpace2   = regexp.MustCompile(`[ \t]+`)
)

func parseHTMLSheets(data []byte) ([]sheetData, error) {
	tables := reHTMLTable.FindAllStringSubmatch(decodeText(data), -1)
	if len(tables) == 0 {
		return nil, fmt.Errorf("tidak ada <table> di dalam file HTML ini")
	}
	var out []sheetData
	for ti, t := range tables {
		rows := [][]cellVal{}
		for _, rm := range reHTMLRow.FindAllStringSubmatch(t[1], -1) {
			var cells []cellVal
			for _, cm := range reHTMLCell.FindAllStringSubmatch(rm[1], -1) {
				s := html.UnescapeString(cm[1])
				s = reHTMLBrk.ReplaceAllString(s, " ")
				s = reHTMLTag.ReplaceAllString(s, "")
				s = strings.TrimSpace(reWSpace2.ReplaceAllString(s, " "))
				cells = append(cells, cellVal{s: s, num: looksNumeric(s)})
			}
			if len(cells) == 0 {
				continue
			}
			rows = append(rows, cells)
			if len(rows) >= maxSheetRows {
				break
			}
		}
		if len(rows) > 0 {
			name := "Tabel " + strconv.Itoa(ti+1)
			if len(tables) == 1 {
				name = "Sheet1"
			}
			out = append(out, sheetData{Name: name, Rows: rows})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tabel HTML kosong (tidak ada baris berisi data)")
	}
	return out, nil
}

func parseCSVSheet(data []byte) ([][]cellVal, error) {
	txt := decodeText(data)
	if strings.TrimSpace(txt) == "" {
		return nil, fmt.Errorf("file CSV kosong")
	}
	lines := strings.Split(txt, "\n")
	head := lines
	if len(head) > 5 {
		head = head[:5]
	}
	d := sniffDelimiter(head)
	rd := csv.NewReader(strings.NewReader(txt))
	rd.Comma = d
	rd.FieldsPerRecord = -1
	rd.LazyQuotes = true
	rd.TrimLeadingSpace = true
	recs, err := rd.ReadAll()
	if err != nil && len(recs) == 0 {
		return nil, fmt.Errorf("CSV tidak bisa dibaca: %v", err)
	}
	rows := make([][]cellVal, 0, len(recs))
	for _, rec := range recs {
		if len(rec) == 0 {
			continue
		}
		cells := make([]cellVal, 0, len(rec))
		for _, v := range rec {
			s := strings.TrimSpace(v)
			cells = append(cells, cellVal{s: s, num: looksNumeric(s)})
		}
		rows = append(rows, cells)
		if len(rows) >= maxSheetRows {
			break
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("CSV tidak berisi baris data")
	}
	return rows, nil
}
