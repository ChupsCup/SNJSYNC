package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PhotosFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Config struct {
	FolderName   string `json:"folderName"`
	DataFolderID string `json:"dataFolderId"`

	PhotosFolders []PhotosFolder `json:"photosFolders,omitempty"`

	PhotosFolderID   string              `json:"photosFolderId"`
	PhotosFolderName string              `json:"photosFolderName"`
	CountryCodes     map[string][]string `json:"countryCodes,omitempty"`
}

func (c Config) PhotoRoots() []PhotosFolder {
	src := c.PhotosFolders
	if len(src) == 0 && c.PhotosFolderID != "" {
		src = []PhotosFolder{{ID: c.PhotosFolderID, Name: c.PhotosFolderName}}
	}
	out := make([]PhotosFolder, 0, len(src))
	seen := map[string]bool{}
	for _, f := range src {
		if f.ID == "" || seen[f.ID] {
			continue
		}
		seen[f.ID] = true
		out = append(out, f)
	}
	return out
}

func photoRootKey(roots []PhotosFolder) string {
	ids := make([]string, len(roots))
	for i, f := range roots {
		ids[i] = f.ID
	}
	return strings.Join(ids, ",")
}

func (c *Config) setPhotoRoots(roots []PhotosFolder) {
	c.PhotosFolders = roots
	if len(roots) > 0 {
		c.PhotosFolderID = roots[0].ID
		c.PhotosFolderName = roots[0].Name
		return
	}
	c.PhotosFolderID = ""
	c.PhotosFolderName = ""
}

func photoRootNames(roots []PhotosFolder) string {
	names := make([]string, 0, len(roots))
	for _, f := range roots {
		if f.Name != "" {
			names = append(names, f.Name)
		}
	}
	return strings.Join(names, " + ")
}

func defaultConfig() Config {
	return Config{FolderName: "CTF", DataFolderID: ""}
}

func loadConfig(root string) Config {
	c := defaultConfig()
	b, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		return c
	}
	if err := json.Unmarshal(stripBOM(b), &c); err != nil {
		return defaultConfig()
	}
	return c
}

func (a *App) saveConfig() {
	b, err := json.MarshalIndent(a.cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(a.root, "config.json"), b, 0o600)
}

func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

func readJSONSafe(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	b = stripBOM(b)
	if !json.Valid(b) {
		return nil
	}
	return b
}

type Barcode struct {
	Text   string `json:"text"`
	Format string `json:"format,omitempty"`
}

type PhotoEntry struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Dir      string    `json:"dir"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Text     string    `json:"text"`
	Barcodes []Barcode `json:"barcodes"`
	OCRConf  float64   `json:"ocrConf"`
}

type cleanOutRec struct {
	list   []cleanOutItem
	inRows int
	outRow int
}

type cleanOutItem struct {
	Sheet  string `json:"sheet"`
	Code   string `json:"code"`
	Desc   string `json:"desc"`
	Alasan string `json:"alasan"`
}

type oauthState struct {
	verifier string
	state    string
	redirect string
}

type App struct {
	root    string
	dataDir string

	mu          sync.RWMutex
	cfg         Config
	idx         []PhotoEntry
	idxMeta     string
	idxFetched  int64
	photoList   []PhotoEntry
	photoMeta   string
	photoSaved  int64
	universe    []PhotoEntry
	universeAt  int64
	oauth       *oauthState
	cleanSeq    int64
	cleanOuts   map[string]*cleanOutRec
	cleanOutsMu sync.Mutex

	searchMu       sync.Mutex
	searchIx       *SearchIndex
	searchStale    *SearchIndex
	searchBuilding bool
	searchDirty    bool

	ready chan struct{}

	dbMu    sync.Mutex
	db      *sql.DB
	dbRetry time.Time

	seenMu sync.Mutex
	seenAt map[int]time.Time
}

func (a *App) requireDB(w http.ResponseWriter) *sql.DB {
	if db := a.dbHandle(); db != nil {
		return db
	}

	st := pgStatus()
	if st != "ready" {
		ensurePostgres()
		a.dbMu.Lock()
		a.dbRetry = time.Time{}
		a.dbMu.Unlock()
	}

	if db := a.waitDBUp(20 * time.Second); db != nil {
		return db
	}
	sendErr(w, 503, pgInfoMsg(pgStatus()))
	return nil
}

func (a *App) waitDBUp(max time.Duration) *sql.DB {
	deadline := time.Now().Add(max)
	for {
		a.dbMu.Lock()
		a.dbRetry = time.Time{}
		a.dbMu.Unlock()
		if db := a.dbHandle(); db != nil {
			return db
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(700 * time.Millisecond)
	}
}

func (a *App) dbHandle() *sql.DB {
	a.waitReady()
	a.dbMu.Lock()
	defer a.dbMu.Unlock()
	if a.db != nil {
		if err := a.db.Ping(); err == nil {
			return a.db
		}
		a.db.Close()
		a.db = nil
	}
	if time.Since(a.dbRetry) < 5*time.Second {
		return nil
	}
	a.dbRetry = time.Now()
	if db := openDB(); db != nil {
		a.db = db
		return db
	}
	return nil
}

func newApp(root string) *App {
	return &App{
		root:      root,
		dataDir:   filepath.Join(root, "data"),
		cfg:       loadConfig(root),
		cleanOuts: map[string]*cleanOutRec{},
		ready:     make(chan struct{}),
	}
}

func (a *App) waitReady() bool {
	select {
	case <-a.ready:
		return true
	case <-time.After(60 * time.Second):
		return false
	}
}

func (a *App) indexPath() string     { return filepath.Join(a.dataDir, "index_cache.json") }
func (a *App) photoListPath() string { return filepath.Join(a.dataDir, "photo_list.json") }
func (a *App) tokenPath() string     { return filepath.Join(a.dataDir, "gdrive_token.json") }

func (a *App) snapshotIdx() []PhotoEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]PhotoEntry(nil), a.idx...)
}

func (a *App) snapshotUniverse() []PhotoEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]PhotoEntry(nil), a.universe...)
}

func (a *App) loadCache() {
	if c := readJSONSafe(a.indexPath()); c != nil {
		var arr []PhotoEntry
		if err := json.Unmarshal(c, &arr); err == nil {
			a.idx = arr
		}
	}
	if c := readJSONSafe(a.photoListPath()); c != nil {
		var pl struct {
			Root  string       `json:"root"`
			Saved int64        `json:"saved"`
			Files []PhotoEntry `json:"files"`
		}
		if err := json.Unmarshal(c, &pl); err == nil && pl.Root != "" && len(pl.Files) > 0 {

			key := photoRootKey(a.cfg.PhotoRoots())
			if key == "" || pl.Root == key {
				a.photoList = pl.Files
				a.photoMeta = pl.Root
				a.photoSaved = pl.Saved
			}
		}
	}
	a.buildPhotoUniverse()
}

func (a *App) buildPhotoUniverse() {
	byID := make(map[string]PhotoEntry, len(a.photoList)+len(a.idx))
	for _, p := range a.photoList {
		e := p
		e.Text = ""
		e.Barcodes = []Barcode{}
		e.OCRConf = 0
		byID[p.ID] = e
	}
	for _, o := range a.idx {
		if e, ok := byID[o.ID]; ok {
			e.Text = o.Text
			if o.Barcodes != nil {
				e.Barcodes = o.Barcodes
			}
			e.OCRConf = o.OCRConf
			byID[o.ID] = e
		}
	}
	for _, o := range a.idx {
		if _, ok := byID[o.ID]; !ok {
			byID[o.ID] = o
		}
	}
	u := make([]PhotoEntry, 0, len(byID))
	for _, v := range byID {
		u = append(u, v)
	}
	a.universe = u
	invalidatePhotoIDs()
}

type jstime time.Time

func (t jstime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z") + `"`), nil
}
