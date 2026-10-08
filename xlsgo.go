package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/shakinm/xlsReader/cfb"
	"github.com/shakinm/xlsReader/xls"
	"github.com/shakinm/xlsReader/xls/structure"

	"golang.org/x/text/encoding/charmap"
)

type xlsCellData = structure.CellData

var (
	errNoSheet          = errors.New("file .xls tidak punya sheet yang bisa dibaca")
	errNoWorkbookStream = errors.New("stream Workbook/Book tidak ditemukan di file OLE2")
)

func readXLSGo(data []byte) ([]sheetData, error) {
	wb, err := xls.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	n := wb.GetNumberSheets()
	if n == 0 {
		return nil, errNoSheet
	}

	out := make([]sheetData, 0, n)
	for i := 0; i < n; i++ {
		sh, err := wb.GetSheet(i)
		if err != nil || sh == nil {
			continue
		}
		nRows := sh.GetNumberRows()
		if nRows > maxSheetRows {
			nRows = maxSheetRows
		}

		maxCols := 0
		allRows := make([][]xlsCellData, 0, nRows)
		for r := 0; r < nRows; r++ {
			row, err := sh.GetRow(r)
			var cols []xlsCellData
			if err == nil && row != nil {
				cols = row.GetCols()
			}
			if len(cols) > maxCols {
				maxCols = len(cols)
			}
			allRows = append(allRows, cols)
		}
		rows := make([][]cellVal, 0, nRows)
		for _, cols := range allRows {
			cells := make([]cellVal, maxCols)
			for c, cd := range cols {
				if cd == nil {
					continue
				}
				if isXLSNumericCell(cd.GetType()) {
					cells[c] = cellVal{s: formatXLSNumber(cd.GetFloat64()), num: true}
				} else {
					cells[c] = cellVal{s: sanitizeXLSString(cd.GetString())}
				}
			}
			rows = append(rows, cells)
		}
		out = append(out, sheetData{Name: sanitizeXLSString(sh.GetName()), Rows: rows})
	}
	if len(out) == 0 {
		return nil, errNoSheet
	}
	return out, nil
}

func isXLSNumericCell(t string) bool {
	return strings.Contains(t, "Number") ||
		strings.Contains(t, "Rk") ||
		strings.Contains(t, "Formula")
}

func formatXLSNumber(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', 15, 64)
}

func sanitizeXLSString(s string) string {
	if s == "" || utf8.ValidString(s) {
		return s
	}
	if decoded, err := charmap.Windows1252.NewDecoder().String(s); err == nil {
		return decoded
	}
	return strings.ToValidUTF8(s, "")
}

func xlsHasFormulas(data []byte) bool {
	stream, err := xlsWorkbookStream(data)
	if err != nil {
		return false
	}
	for p := 0; p+4 <= len(stream); {
		id := stream[p : p+2]
		length := int(binary.LittleEndian.Uint16(stream[p+2 : p+4]))

		switch {
		case id[0] == 0x06 && id[1] == 0x00:
			return true
		case id[0] == 0xBC && id[1] == 0x04:
			return true
		case id[0] == 0x21 && id[1] == 0x02:
			return true
		case id[0] == 0x07 && id[1] == 0x02:
			return true
		}
		if length <= 0 {
			p += 4
			continue
		}
		p += 4 + length
	}
	return false
}

func xlsWorkbookStream(data []byte) ([]byte, error) {
	adaptor, err := cfb.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer adaptor.CloseFile()

	var book, root *cfb.Directory
	for _, d := range adaptor.GetDirs() {
		switch d.Name() {
		case "Workbook":
			if book == nil {
				book = d
			}
		case "Book":
			book = d
		case "Root Entry":
			root = d
		}
	}
	if book == nil {
		return nil, errNoWorkbookStream
	}
	size := int(book.GetStreamSize())
	if size <= 0 || size > len(data)*8 {
		return nil, errNoWorkbookStream
	}
	rd, err := adaptor.OpenObject(book, root)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(rd, buf); err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf, nil
}

func readXLS(data []byte) ([]sheetData, error) {

	switch os.Getenv("XLS_READER") {
	case "python":
		if sheets, err := readXLSPython(data); err == nil {
			return sheets, nil
		} else {
			logf("[XLS] XLS_READER=python tapi xlrd gagal: %v", err)
		}
	case "go":
		if sheets, err := readXLSGo(data); err == nil {
			return sheets, nil
		} else {
			logf("[XLS] XLS_READER=go tapi pembaca Go gagal: %v", err)
		}
	}

	if !xlsHasFormulas(data) {
		sheets, err := readXLSGo(data)
		if err == nil {
			return sheets, nil
		}
		logf("[XLS] pembaca Go gagal (%v), mencoba xlrd", err)
		if ps, perr := readXLSPython(data); perr == nil {
			return ps, nil
		}
		return nil, err
	}

	logf("[XLS] file memakai FORMULA, dibaca dengan xlrd supaya nilai sel tidak kosong")
	if sheets, err := readXLSPython(data); err == nil {
		return sheets, nil
	} else if errMissingPython(err) {
		logf("[XLS] python/xlrd tidak tersedia, kembali ke pembaca Go. " +
			"Sel berisi formula kemungkinan kosong.")
	}
	sheets, err := readXLSGo(data)
	if err != nil {
		return nil, err
	}
	return sheets, nil
}

func errMissingPython(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "python tidak ditemukan") ||
		strings.Contains(msg, xlsSidecarName)
}
