package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const xlsSidecarName = "xls_sidecar.py"

type pyCell struct {
	S   string `json:"s"`
	Num bool   `json:"num"`
}

type pySheet struct {
	Name string     `json:"name"`
	Rows [][]pyCell `json:"rows"`
}

type pyBook struct {
	Sheets []pySheet `json:"sheets"`
	Error  string    `json:"error"`
}

func readXLSPython(data []byte) ([]sheetData, error) {
	python := findPython()
	if python == "" {
		return nil, fmt.Errorf("python tidak ditemukan di PATH")
	}
	script := findSidecar()
	if script == "" {
		return nil, fmt.Errorf("%s tidak ditemukan", xlsSidecarName)
	}

	tmp, err := os.CreateTemp("", "snjsync-*.xls")
	if err != nil {
		return nil, fmt.Errorf("tidak bisa membuat file sementara: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("tidak bisa menulis file sementara: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("tidak bisa menutup file sementara: %v", err)
	}

	ctx, cancel := ctxTimeout(5 * time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, script, tmp.Name())
	cmd.Dir = exeDir()

	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("membaca .xls timed out setelah 5 menit")
		}
		return nil, fmt.Errorf("gagal menjalankan pembaca .xls: %v%s", err, tailStderr(stderr.String()))
	}

	var book pyBook
	if err := json.Unmarshal(stdout, &book); err != nil {
		return nil, fmt.Errorf("output pembaca .xls tidak valid: %v", err)
	}
	if book.Error != "" {
		return nil, fmt.Errorf("%s", book.Error)
	}

	out := make([]sheetData, 0, len(book.Sheets))
	for _, sh := range book.Sheets {
		rows := make([][]cellVal, 0, len(sh.Rows))
		for _, r := range sh.Rows {
			cells := make([]cellVal, len(r))
			for i, c := range r {
				cells[i] = cellVal{s: sanitizeXLSString(c.S), num: c.Num}
			}
			rows = append(rows, cells)
		}
		if len(rows) > maxSheetRows {
			rows = rows[:maxSheetRows]
		}
		out = append(out, sheetData{Name: sanitizeXLSString(sh.Name), Rows: rows})
	}
	if len(out) == 0 {
		return nil, errNoSheet
	}
	return out, nil
}

func findSidecar() string {
	if p := os.Getenv("XLS_SIDECAR_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	var dirs []string
	if d := exeDir(); d != "" {
		dirs = append(dirs, d)
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	for _, d := range dirs {
		p := filepath.Join(d, xlsSidecarName)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func tailStderr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 4 {
		lines = lines[len(lines)-4:]
	}
	return " (python: " + strings.Join(lines, " | ") + ")"
}

func findPython() string {
	for _, name := range []string{"python", "python3", "py"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		cmd := exec.Command(p, "-c", "import sys;print(sys.version_info[0])")
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		if v := strings.TrimSpace(string(out)); v == "3" {
			return p
		}
	}
	return ""
}
