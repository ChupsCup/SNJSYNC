package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type pedomanCache struct {
	Codes  map[string]bool
	Names  []string
	Mereks []string
	Total  int
}

type cleanOpts struct {
	CodeCols        string
	DescCols        string
	RemoveBlank     bool
	RemoveEmpty     bool
	KeepSnj         bool
	FilterByPedoman bool
	Pedoman         *pedomanCache
}

type cleanResult struct {
	Buffer  []byte
	InRows  int
	OutRows int
	OutCols int
	Sheets  []string
	DashIn  int
	DashOut int
	OutList []cleanOutItem
}

func colLettersToRange(s string) [2]int {
	var zero [2]int
	if s == "" {
		return zero
	}
	t := strings.ToUpper(strings.TrimSpace(s))
	if m := regexp.MustCompile(`^([A-Z]{1,2})-([A-Z]{1,2})$`).FindStringSubmatch(t); m != nil {
		a, b := decodeCol(m[1]), decodeCol(m[2])
		if b < a {
			return zero
		}
		return [2]int{a, b}
	}
	if m := regexp.MustCompile(`^([A-Z]{1,2})$`).FindStringSubmatch(t); m != nil {
		a := decodeCol(m[1])
		return [2]int{a, a}
	}
	return zero
}

var (
	reGroupNoBarang = regexp.MustCompile(`no\s*\.?\s*barang`)
	reGroupDesc     = regexp.MustCompile(`deskripsi|description`)
	reBadSheetChars = regexp.MustCompile(`[\\/:*?\[\]<>|]`)
)

func detectGroupRow(rows [][]cellVal, headerIdx int, re *regexp.Regexp) int {
	if headerIdx >= len(rows) {
		return -1
	}
	h := rows[headerIdx]
	for i, c := range h {
		if re.MatchString(strings.ToLower(strings.TrimSpace(c.s))) {
			return i
		}
	}
	return -1
}

func buildCleanedExcel(data []byte, opts cleanOpts) (*cleanResult, error) {

	sheets, _, err := openSheetsOpt(data, true)
	if err != nil {
		return nil, err
	}
	codeOpt := colLettersToRange(opts.CodeCols)
	descOpt := colLettersToRange(opts.DescCols)
	removeBlank := opts.RemoveBlank
	removeEmpty := opts.RemoveEmpty
	hasCodeOpt := codeOpt != [2]int{}
	hasDescOpt := descOpt != [2]int{}

	out := excelize.NewFile()
	outList := []cleanOutItem{}
	res := &cleanResult{OutList: outList}
	sheetNames := []string{}
	first := true

	for _, sh := range sheets {
		name := sh.Name
		rows := sh.Rows
		if len(rows) == 0 {
			continue
		}
		headerIdx := -1
		for i := 0; i < len(rows) && i < 10; i++ {
			count := 0
			for _, c := range rows[i] {
				if strings.TrimSpace(c.s) != "" {
					count++
				}
			}
			if count >= 2 {
				headerIdx = i
				break
			}
		}
		if headerIdx == -1 {
			continue
		}
		dashSource := 0
		for i := headerIdx + 1; i < len(rows); i++ {
			anyNonEmpty := false
			dash := false
			for _, c := range rows[i] {
				if strings.TrimSpace(c.s) != "" {
					anyNonEmpty = true
				}
				if strings.Contains(normStr(c.s), "---") {
					dash = true
				}
			}
			if !anyNonEmpty {
				continue
			}
			if dash {
				dashSource++
			}
		}
		res.DashIn += dashSource

		codeStart := detectGroupRow(rows, headerIdx, reGroupNoBarang)
		if codeStart < 0 {
			if hasCodeOpt {
				codeStart = codeOpt[0]
			} else {
				codeStart = 0
			}
		}
		descStart := detectGroupRow(rows, headerIdx, reGroupDesc)
		if descStart < 0 {
			if hasDescOpt {
				descStart = descOpt[0]
			} else {
				descStart = 4
			}
		}
		var hIdx []int
		for i, c := range rows[headerIdx] {
			if strings.TrimSpace(c.s) != "" {
				hIdx = append(hIdx, i)
			}
		}
		nextH := func(i int) int {
			for _, x := range hIdx {
				if x > i {
					return x
				}
			}
			return -1
		}
		codeEnd := codeStart + 2
		if len(hIdx) > 0 {
			if nh := nextH(codeStart); nh >= 0 {
				codeEnd = nh - 1
			} else {
				codeEnd = codeStart + 3
			}
		}
		descEnd := descStart + 2
		if len(hIdx) > 0 {
			if nh := nextH(descStart); nh >= 0 {
				descEnd = nh - 1
			} else {
				descEnd = descStart + 3
			}
		}
		if codeEnd < codeStart {
			codeEnd = codeStart + 2
		}
		if descEnd < descStart {
			descEnd = descStart + 2
		}
		c1, c2 := codeStart, codeEnd
		d1, d2 := descStart, descEnd
		maxCol := c2 + 1
		if d2+1 > maxCol {
			maxCol = d2 + 1
		}
		for _, r := range rows {
			if len(r) > maxCol {
				maxCol = len(r)
			}
		}
		inGroup := func(c int) bool { return (c >= c1 && c <= c2) || (c >= d1 && c <= d2) }
		keep := []int{}
		for c := 0; c < maxCol; c++ {
			if c == c1 || c == d1 {
				keep = append(keep, c)
			} else if !inGroup(c) {
				keep = append(keep, c)
			}
		}
		k1, k2 := -1, -1
		for i, c := range keep {
			if c == c1 {
				k1 = i
			}
			if c == d1 {
				k2 = i
			}
		}
		if k1 < 0 || k2 < 0 {
			continue
		}
		headerRow := make([]string, len(keep))
		for i, c := range keep {
			if c == c1 {
				headerRow[i] = "No. Barang (Gabungan)"
			} else if c == d1 {
				headerRow[i] = "Deskripsi (Gabungan)"
			} else {
				headerRow[i] = strings.TrimSpace(cellText(rows[headerIdx], c))
			}
		}
		getCol := func(src []cellVal, c int) string {
			if c < len(src) {
				return src[c].s
			}
			return ""
		}
		merge := func(src []cellVal, a, b int) string {
			var parts []string
			for i := a; i <= b; i++ {
				s := strings.TrimSpace(getCol(src, i))
				if s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, "")
		}
		dataRows := [][]string{}
		for i := headerIdx + 1; i < len(rows); i++ {
			src := rows[i]
			mergedCode := merge(src, c1, c2)
			mergedDesc := merge(src, d1, d2)
			row := make([]string, len(keep))
			for j, c := range keep {
				switch c {
				case c1:
					row[j] = mergedCode
				case d1:
					row[j] = mergedDesc
				default:
					row[j] = getCol(src, c)
				}
			}
			dataRows = append(dataRows, row)
			res.InRows++
		}
		for _, row := range dataRows {
			codeVal := strings.TrimSpace(row[k1])
			descVal := strings.TrimSpace(row[k2])
			hasOther := false
			for j, x := range row {
				if j != k1 && j != k2 && strings.TrimSpace(x) != "" {
					hasOther = true
					break
				}
			}
			if codeVal == "" && descVal == "" && hasOther {
				row[k1] = "TOTAL"
				row[k2] = "TOTAL"
			}
		}
		if removeBlank {
			filtered := make([][]string, 0, len(dataRows))
			for _, row := range dataRows {
				empty := true
				for _, x := range row {
					if strings.TrimSpace(x) != "" {
						empty = false
						break
					}
				}
				if !empty {
					filtered = append(filtered, row)
				}
			}
			dataRows = filtered
		}
		keepSnjMode := opts.KeepSnj && !(opts.FilterByPedoman && opts.Pedoman != nil)
		pedMode := opts.FilterByPedoman && opts.Pedoman != nil
		if keepSnjMode || pedMode {
			kept := make([][]string, 0, len(dataRows))
			for _, row := range dataRows {
				codeVal := strings.TrimSpace(row[k1])
				descVal := strings.TrimSpace(row[k2])
				if keepSnjMode {
					if strings.Contains(normStr(descVal), "---") {
						kept = append(kept, row)
					} else {
						res.OutList = append(res.OutList, cleanOutItem{Sheet: name, Code: codeVal, Desc: descVal,
							Alasan: "tidak ada tanda --- (bukan kepemilikan SNJ)"})
					}
					continue
				}
				nkode := normKode(codeVal)
				if strings.Contains(normStr(descVal), "---") {
					kept = append(kept, row)
					continue
				}
				match := false
				if nkode != "" && opts.Pedoman.Codes[nkode] {
					match = true
				}
				if !match {
					ndesc := normStr(descVal)
					if ndesc != "" {
						for _, nm := range opts.Pedoman.Names {
							if strings.Contains(ndesc, nm) {
								match = true
								break
							}
						}
					}
					if !match {
						for _, mk := range opts.Pedoman.Mereks {
							if mk != "" && strings.Contains(normStr(descVal), mk) {
								match = true
								break
							}
						}
					}
				}
				if match {
					kept = append(kept, row)
				} else {
					alasan := "kode kosong"
					if nkode != "" {
						alasan = "kode tak ada di pedoman"
					}
					res.OutList = append(res.OutList, cleanOutItem{Sheet: name, Code: codeVal, Desc: descVal, Alasan: alasan})
				}
			}
			dataRows = kept
		}
		if removeEmpty {
			for c := len(headerRow) - 1; c >= 0; c-- {
				hasData := strings.TrimSpace(headerRow[c]) != ""
				if !hasData {
					for _, r := range dataRows {
						if c < len(r) && strings.TrimSpace(r[c]) != "" {
							hasData = true
							break
						}
					}
				}
				if hasData {
					continue
				}
				headerRow = append(headerRow[:c], headerRow[c+1:]...)
				for ri := range dataRows {
					if c < len(dataRows[ri]) {
						dataRows[ri] = append(dataRows[ri][:c], dataRows[ri][c+1:]...)
					}
				}
			}
		}
		if len(dataRows) == 0 {
			continue
		}
		for _, r := range dataRows {
			for _, x := range r {
				if strings.Contains(normStr(x), "---") {
					res.DashOut++
					break
				}
			}
		}
		sheetName := reBadSheetChars.ReplaceAllString(name, "_")
		if sheetName == "" {
			sheetName = "Sheet1"
		}
		if first {
			out.SetSheetName("Sheet1", sheetName)
			first = false
		} else if idx, _ := out.GetSheetIndex(sheetName); idx != -1 {
			sheetName = sheetName + "_" + itoa(res.OutRows+1)
			if idx, _ := out.GetSheetIndex(sheetName); idx != -1 {
				continue
			}
			if _, err := out.NewSheet(sheetName); err != nil {
				continue
			}
		} else {
			if _, err := out.NewSheet(sheetName); err != nil {
				continue
			}
		}
		style, _ := out.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: "FFFFFFFF", Size: 11},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF97316"}},
			Alignment: &excelize.Alignment{Vertical: "middle"},
		})
		hdrRow := make([]interface{}, len(headerRow))
		for i, h := range headerRow {
			hdrRow[i] = h
		}
		_ = out.SetSheetRow(sheetName, "A1", &hdrRow)
		if style != 0 {
			_ = out.SetCellStyle(sheetName, "A1", cellName(len(headerRow), 1), style)
		}
		for i, r := range dataRows {
			vals := make([]interface{}, len(r))
			for j, v := range r {
				vals[j] = v
			}
			_ = out.SetSheetRow(sheetName, "A"+itoa(i+2), &vals)
		}
		lastCol := encodeCol(len(headerRow) - 1)
		_ = out.SetPanes(sheetName, &excelize.Panes{
			Freeze: true, Split: false, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
			Selection: []excelize.Selection{{SQRef: "A2", ActiveCell: "A2", Pane: "bottomLeft"}},
		})
		_ = out.AutoFilter(sheetName, "A1:"+lastCol+strconv.Itoa(len(dataRows)+1), nil)
		all := append([][]string{headerRow}, dataRows...)
		for c := 0; c < len(headerRow); c++ {
			maxLen := 8
			for _, r := range all {
				v := ""
				if c < len(r) {
					v = r[c]
				}
				if L := len(v); L > maxLen {
					if L > 60 {
						L = 60
					}
					maxLen = L
				}
			}
			cn := cellName(len(headerRow), c+1)
			_ = out.SetColWidth(sheetName, cn, cn, float64(maxLen+2))
		}
		res.OutRows += len(dataRows)
		if len(headerRow) > res.OutCols {
			res.OutCols = len(headerRow)
		}
		sheetNames = append(sheetNames, name)
	}
	if len(sheetNames) == 0 {
		return nil, fmt.Errorf("Tidak ada sheet dengan data yang bisa diproses.")
	}
	if res.DashOut < res.DashIn {
		return nil, fmt.Errorf("PENGAMAN SNJ: %d baris bertanda --- hilang dari output (input %d, output %d). Baris SNJ TIDAK BOLEH hilang cukup MAU.",
			res.DashIn-res.DashOut, res.DashIn, res.DashOut)
	}
	buf, err := out.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	res.Buffer = buf.Bytes()
	res.Sheets = sheetNames
	return res, nil
}

func cellName(ncols, col int) string {
	return encodeCol(col - 1)
}

func (a *App) exportMCExcel(results []compareRow) ([]byte, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("Tidak ada data untuk diekspor.")
	}
	templatePath := filepath.Join(a.root, "template_output.xlsx")
	var f *excelize.File
	var err error
	if _, statErr := os.Stat(templatePath); statErr == nil {
		f, err = excelize.OpenFile(templatePath)
		if err != nil {
			return nil, err
		}
	} else {
		f = excelize.NewFile()
		hdr := []interface{}{"NO", "NO CONTAINER", "ETA", "BRAND", "ITEM NAME BY DOKUMEN IMPORT",
			"STATUS (EXISTING/NEW ITEM)", "ITEM CODE ACC", "ITEM DECS ACC", "CARTON", "KG/ CARTON",
			"TOTAL TONASE", "IMPORTIR (PPI/BERDIKARI)", "DATE CHECKED", "CHEKED BY OPS (V/X)", "NOTES"}
		_ = f.SetSheetRow("Sheet1", "A1", &hdr)
	}
	defer f.Close()
	sheet := ""
	if list := f.GetSheetList(); len(list) > 0 {
		sheet = list[0]
	} else {
		if _, err := f.NewSheet("Sheet1"); err != nil {
			return nil, err
		}
		sheet = "Sheet1"
	}
	lastRow := 1
	if rows, err := f.GetRows(sheet); err == nil {
		lastRow = len(rows)
	}
	if lastRow < 1 {
		lastRow = 1
	}
	for i, r := range results {
		noContainer := firstNonEmpty(r.NoContainer, r.NoContainerSnake)
		namaItem := firstNonEmpty(r.NamaItem, r.NamaItemSnake)
		masterCode := r.MasterCode
		if masterCode == "-" {
			masterCode = ""
		}
		masterDesc := r.MasterDesc
		if masterDesc == "-" {
			masterDesc = ""
		}
		vals := []interface{}{
			r.No, noContainer, r.ETA, r.Brand, namaItem, r.Flag,
			masterCode, masterDesc, "", "", "",
			r.Importir, "", "", r.Note,
		}
		_ = f.SetSheetRow(sheet, "A"+itoa(lastRow+i+1), &vals)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
