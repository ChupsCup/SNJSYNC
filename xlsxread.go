package main

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const maxSheetRows = 50000

type cellVal struct {
	s   string
	num bool
}

func (c cellVal) jsStr() string {
	s := c.s
	if s == "" {
		return ""
	}
	if c.num {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f == 0 {
			return ""
		}
	}
	return s
}

var reNumericLiteral = regexp.MustCompile(`^-?(\d+(\.\d+)?|\.\d+)([eE][-+]?\d+)?$`)

func looksNumeric(s string) bool { return reNumericLiteral.MatchString(s) }

func sheetRows(f *excelize.File, sheet string, keepBlank bool) [][]cellVal {
	rows, err := f.Rows(sheet)
	if err != nil {
		return nil
	}
	defer rows.Close()
	width := 0
	if dim, err := f.GetSheetDimension(sheet); err == nil && dim != "" {
		if _, mx, _, ok := parseRangeRef(dim); ok {
			width = mx
		}
	}
	out := [][]cellVal{}
	n := 0
	for rows.Next() {
		n++
		if n > maxSheetRows {
			break
		}
		vals, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			break
		}
		if len(vals) == 0 && !keepBlank {
			continue
		}
		r := make([]cellVal, len(vals))
		empty := true
		for i, v := range vals {
			r[i] = cellVal{s: v, num: looksNumeric(v)}
			if v != "" {
				empty = false
			}
		}
		if empty && !keepBlank {
			continue
		}
		if width > len(r) {
			pad := make([]cellVal, width-len(r))
			r = append(r, pad...)
		}
		out = append(out, r)
	}
	return out
}

func parseRangeRef(ref string) (sc, ec, er int, ok bool) {
	i := strings.Index(ref, ":")
	if i < 0 {
		return 0, 0, 0, false
	}
	end := ref[i+1:]
	col, row, ok := parseCellRef(end)
	if !ok {
		return 0, 0, 0, false
	}
	return 0, col, row, true
}

func parseCellRef(ref string) (col, row int, ok bool) {
	c := 0
	i := 0
	for i < len(ref) {
		ch := ref[i]
		if ch >= 'A' && ch <= 'Z' {
			c = c*26 + int(ch-'A') + 1
			i++
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			c = c*26 + int(ch-'a') + 1
			i++
			continue
		}
		break
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	r := 0
	for ; i < len(ref); i++ {
		if ref[i] < '0' || ref[i] > '9' {
			return 0, 0, false
		}
		r = r*10 + int(ref[i]-'0')
	}
	return c - 1, r - 1, true
}

func decodeCol(letters string) int {
	c := 0
	for _, ch := range strings.ToUpper(letters) {
		if ch < 'A' || ch > 'Z' {
			return -1
		}
		c = c*26 + int(ch-'A') + 1
	}
	return c - 1
}

func encodeCol(idx int) string {
	s := ""
	idx++
	for idx > 0 {
		idx--
		s = string(rune('A'+idx%26)) + s
		idx /= 26
	}
	return s
}

func cellAt(r []cellVal, c int) cellVal {
	if c < 0 || c >= len(r) {
		return cellVal{}
	}
	return r[c]
}

func cellText(r []cellVal, c int) string { return cellAt(r, c).jsStr() }

var hari = []string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}
var bulan = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func fmtDate(d time.Time) string {
	return hari[int(d.Weekday())] + ", " + strconv.Itoa(d.Day()) + " " + bulan[int(d.Month())-1] + " " + strconv.Itoa(d.Year())
}

func serialToDate(v float64) (time.Time, bool) {
	ms := math.Round((v - 25569) * 86400 * 1000)
	if math.IsNaN(ms) || ms <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(int64(ms)).Local(), true
}

var reFiveDigits = regexp.MustCompile(`^\d{5}$`)
var reDMY = regexp.MustCompile(`^(\d{1,2})[/\-](\d{1,2})[/\-](\d{4})$`)
var reYear20xx = regexp.MustCompile(`20\d\d`)

func parseEta(v string, isNum bool) string {
	if isNum {
		if d, ok := serialToDate(mustFloat(v)); ok {
			return fmtDate(d)
		}
		return v
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	if reFiveDigits.MatchString(s) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if d, ok := serialToDate(f); ok {
				return fmtDate(d)
			}
		}
		return s
	}
	if m := reDMY.FindStringSubmatch(s); m != nil {
		day, _ := strconv.Atoi(m[1])
		mon, _ := strconv.Atoi(m[2])
		yr, _ := strconv.Atoi(m[3])
		d := time.Date(yr, time.Month(mon), day, 0, 0, 0, 0, time.Local)
		if day <= 31 && mon >= 1 && mon <= 12 {
			return fmtDate(d)
		}
		return s
	}
	if reYear20xx.MatchString(s) {
		if d, ok := parseLooseDate(s); ok {
			return fmtDate(d)
		}
	}
	return s
}

func mustFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

var looseLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02",
	"2 January 2006",
	"2 Jan 2006",
	"January 2, 2006",
	"Jan 2, 2006",
	"02-Jan-2006",
	"2-Jan-2006",
	"20060102",
}

func parseLooseDate(s string) (time.Time, bool) {
	for _, l := range looseLayouts {
		if d, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return d, true
		}
	}
	return time.Time{}, false
}

type parseRow struct {
	Sheet       string
	NoContainer string
	Brand       string
	NamaItem    string
	ETA         string
	ItemCode    string
	ItemDesc    string
	Gabungan    string
	Importir    string
}

var reTotalRow = regexp.MustCompile(`(?i)TOTAL|SUB-?TOTAL|GRAND ?TOTAL`)
var rePureNum = regexp.MustCompile(`^\d+(\.\d+)?$`)
var reWSpace = regexp.MustCompile(`\s+`)

func parseExcelBuf(data []byte) ([]parseRow, error) {
	sheets, _, err := openSheets(data)
	if err != nil {
		return nil, err
	}
	allRows := []parseRow{}
	for _, sh := range sheets {
		rows := sh.Rows
		if len(rows) == 0 {
			continue
		}
		headerIdx := -1
		for i := 0; i < len(rows) && i < 5; i++ {
			count := 0
			for _, c := range rows[i] {
				if strings.TrimSpace(c.s) != "" {
					count++
				}
			}
			if count >= 3 {
				headerIdx = i
				break
			}
		}
		if headerIdx == -1 {
			continue
		}
		headers := make([]string, len(rows[headerIdx]))
		for i, c := range rows[headerIdx] {
			headers[i] = strings.ToLower(strings.TrimSpace(c.s))
		}
		idx := func(pred func(string) bool) int {
			for i, h := range headers {
				if pred(h) {
					return i
				}
			}
			return -1
		}
		has := func(h, sub string) bool { return strings.Contains(h, sub) }
		cNo := idx(func(h string) bool { return has(h, "no container") })
		cBrand := idx(func(h string) bool { return has(h, "brand") || has(h, "merek") })
		cNama := idx(func(h string) bool { return has(h, "nama item") || has(h, "nama barang") || has(h, "item name") })
		cEta := idx(func(h string) bool { return has(h, "eta") || has(h, "tgl") })
		cItemCode := idx(func(h string) bool {
			return has(h, "item code") || has(h, "kode barang") || has(h, "no. barang") || has(h, "no barang") ||
				(has(h, "kode") && !has(h, "akun"))
		})
		cItemDesc := idx(func(h string) bool { return has(h, "item desc") || has(h, "description") || has(h, "deskripsi") })
		cGabungan := idx(func(h string) bool {
			return h == "gabungan" || (has(h, "gabungan") && !has(h, "barang") && !has(h, "deskripsi"))
		})
		cImportir := idx(func(h string) bool { return has(h, "importir") || has(h, "kepemilikan") })

		blankStreak := 0
		for i := headerIdx + 1; i < len(rows); i++ {
			r := rows[i]
			get := func(col int) string {
				if col >= 0 {
					return strings.TrimSpace(cellText(r, col))
				}
				return ""
			}
			noContainer := get(cNo)
			brand := get(cBrand)
			namaItem := get(cNama)
			eta := parseEta(get(cEta), cEta >= 0 && cellAt(r, cEta).num)
			itemCode := get(cItemCode)
			itemDesc := get(cItemDesc)
			imp := get(cImportir)
			gabunganFromCol := get(cGabungan)
			gabungan := gabunganFromCol
			if gabungan == "" {
				gabungan = strings.TrimSpace(reWSpace.ReplaceAllString(brand+" "+namaItem, " "))
			}
			hasAny := noContainer != "" || brand != "" || namaItem != "" || itemCode != "" || itemDesc != "" ||
				gabunganFromCol != "" || gabungan != ""
			if !hasAny {
				blankStreak++
				if blankStreak >= 3 {
					break
				}
				continue
			}
			blankStreak = 0
			if reTotalRow.MatchString(itemCode+" "+itemDesc) &&
				!rePureNum.MatchString(itemCode) && !rePureNum.MatchString(itemDesc) {
				continue
			}
			allRows = append(allRows, parseRow{
				Sheet: sh.Name, NoContainer: noContainer, Brand: brand, NamaItem: namaItem,
				ETA: eta, ItemCode: itemCode, ItemDesc: itemDesc, Gabungan: gabungan, Importir: imp,
			})
		}
	}
	return allRows, nil
}

type pedomanRow struct {
	KodeBarang  string
	NamaBarang  string
	MerekBarang string
	NonAktif    string
}

var reKodeBarang = regexp.MustCompile(`(?i)kode barang`)
var reNamaBarang = regexp.MustCompile(`(?i)nama barang`)
var reMerek = regexp.MustCompile(`(?i)merek barang|merek`)
var reNonAktif = regexp.MustCompile(`(?i)non aktif|nonaktif`)
var reKode = regexp.MustCompile(`(?i)kode`)
var reNama = regexp.MustCompile(`(?i)nama`)

func parsePedomanBuf(data []byte) ([]pedomanRow, error) {
	sheets, _, err := openSheets(data)
	if err != nil {
		return nil, err
	}
	rowsOut := []pedomanRow{}
	for _, sh := range sheets {
		rows := sh.Rows
		if len(rows) == 0 {
			continue
		}
		headers := make([]string, len(rows[0]))
		for i, c := range rows[0] {
			headers[i] = strings.TrimSpace(c.s)
		}
		keyOf := func(re *regexp.Regexp) int {
			for i, h := range headers {
				if re.MatchString(h) {
					return i
				}
			}
			return -1
		}
		kK, kN := keyOf(reKodeBarang), keyOf(reNamaBarang)
		if kK < 0 {
			kK = keyOf(reKode)
		}
		if kN < 0 {
			kN = keyOf(reNama)
		}
		kM, kA := keyOf(reMerek), keyOf(reNonAktif)
		if kK < 0 || kN < 0 {
			continue
		}
		for _, r := range rows[1:] {
			label := strings.TrimSpace(cellText(r, kK))
			nama := strings.TrimSpace(cellText(r, kN))
			if label == "" && nama == "" {
				continue
			}
			rowsOut = append(rowsOut, pedomanRow{
				KodeBarang:  label,
				NamaBarang:  nama,
				MerekBarang: strings.TrimSpace(cellText(r, kM)),
				NonAktif:    strings.TrimSpace(cellText(r, kA)),
			})
		}
		break
	}
	return rowsOut, nil
}
