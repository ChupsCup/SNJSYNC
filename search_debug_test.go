package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func loadRealApp(t *testing.T) *App {
	t.Helper()
	if os.Getenv("CTF_DEBUG") == "" {
		t.Skip("audit data asli (set CTF_DEBUG=1 untuk menjalankan)")
	}
	root := "D:/Website/SNJSYNC"
	if v := os.Getenv("CTF_ROOT"); v != "" {
		root = v
	}
	app := &App{root: root, dataDir: root + "/data", ready: make(chan struct{})}
	close(app.ready)
	b, err := os.ReadFile(root + "/data/index_cache.json")
	if err != nil {
		t.Skip("index_cache.json tidak ada: " + err.Error())
	}
	var idx []PhotoEntry
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatal(err)
	}
	pb, err := os.ReadFile(root + "/data/photo_list.json")
	if err != nil {
		t.Skip("photo_list.json tidak ada: " + err.Error())
	}
	var pl struct {
		Root  string       `json:"root"`
		Saved int64        `json:"saved"`
		Files []PhotoEntry `json:"files"`
	}
	if err := json.Unmarshal(pb, &pl); err != nil {
		t.Fatal(err)
	}
	app.idx = idx
	app.photoList = pl.Files
	app.buildPhotoUniverse()
	app.loadAliasMap()
	return app
}

func TestDebugRealQuery(t *testing.T) {
	app := loadRealApp(t)
	queries := []string{
		"SLICE (RUSTAM)   --- FQ11",
		"SLICE",
		"TOPSIDE (BLACK GEMS) --- HQ41",
		"TONGUE ROOT (SWIFT) --- 90VL --- EST.969 US",
		"SHORTLOIN",
	}
	for _, q := range queries {
		t1 := time.Now()
		resp := app.search(q)
		fmt.Printf("\n=== q=%q ms=%d exact=%d partial=%d\n",
			q, time.Since(t1).Milliseconds(), resp.ExactCount, resp.PartialCount)
		seen := map[string]bool{}
		n := 0
		for _, p := range resp.Results {
			leaf := leafOf(p.Dir)
			if seen["E:"+leaf] {
				continue
			}
			seen["E:"+leaf] = true
			fmt.Printf("  EXACT (%d/%d) %s\n", p.Matched, p.Total, leaf)
			n++
		}
		for _, p := range resp.Partial {
			leaf := leafOf(p.Dir)
			if seen["P:"+leaf] {
				continue
			}
			seen["P:"+leaf] = true
			fmt.Printf("  PART  (%d/%d) %s\n", p.Matched, p.Total, leaf)
			n++
			if n > 40 {
				break
			}
		}
	}
}

func leafOf(dir string) string {
	parts := splitFields(dir, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return dir
	}
	return parts[len(parts)-1]
}
