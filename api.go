package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type mpPart struct {
	name     string
	filename string
	data     []byte
}

func parseMultipart(buf []byte, boundary string) []mpPart {
	sep := []byte("--" + boundary)
	var parts []mpPart
	pos := 0
	for {
		start := bytesIndex(buf, sep, pos)
		if start < 0 {
			break
		}
		next := bytesIndex(buf, sep, start+len(sep))
		if next < 0 {
			break
		}
		body := buf[start+len(sep) : next]
		hEnd := indexOfCRLFCRLF(body)
		if hEnd < 0 {
			pos = next
			continue
		}
		headers := string(body[:hEnd])
		data := body[hEnd+4:]
		if len(data) >= 2 {
			data = data[:len(data)-2]
		}
		parts = append(parts, mpPart{
			name:     matchGroup(reNameAttr, headers),
			filename: matchGroup(reFilenameAttr, headers),
			data:     data,
		})
		pos = next
	}
	return parts
}

var (
	reNameAttr     = regexp.MustCompile(`name="([^"]+)"`)
	reFilenameAttr = regexp.MustCompile(`filename="([^"]+)"`)
	reBoundary     = regexp.MustCompile(`boundary=(.+)`)
)

func matchGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func bytesIndex(b, sub []byte, from int) int {
	if from < 0 {
		from = 0
	}
	if from >= len(b) {
		return -1
	}
	i := bytes.Index(b[from:], sub)
	if i < 0 {
		return -1
	}
	return from + i
}

func indexOfCRLFCRLF(b []byte) int {
	return bytes.Index(b, []byte("\r\n\r\n"))
}

func multipartParts(r *http.Request) ([]mpPart, []byte, bool) {
	ct := r.Header.Get("Content-Type")
	m := reBoundary.FindStringSubmatch(ct)
	if m == nil {
		return nil, nil, false
	}
	buf, err := readBody(r)
	if err != nil {
		return nil, nil, false
	}
	return parseMultipart(buf, m[1]), buf, true
}

type dirStat struct {
	Dir   string `json:"dir"`
	Count int    `json:"count"`
}

type statsResp struct {
	Total          int       `json:"total"`
	WithOCR        int       `json:"withOcr"`
	WithBarcodes   int       `json:"withBarcodes"`
	IndexFetchedAt int64     `json:"indexFetchedAt"`
	Photos         int       `json:"photos"`
	OCRIndexed     int       `json:"ocrIndexed"`
	Dirs           []dirStat `json:"dirs"`

	PhotosSavedAt   int64          `json:"photosSavedAt"`
	PhotosFolderID  string         `json:"photosFolderId"`
	PhotosFolderNm  string         `json:"photosFolderName"`
	PhotosFolders   []PhotosFolder `json:"photosFolders"`
	DataFolderID    string         `json:"dataFolderId"`
	DataFolderName  string         `json:"dataFolderName"`
	PhotosFolderSet bool           `json:"photosFolderSet"`
	AliasCount      int            `json:"aliasCount"`
	WithoutOCR      int            `json:"withoutOcr"`
}

func (a *App) stats() statsResp {
	a.mu.RLock()
	cfg := a.cfg
	src := a.universe
	if len(src) == 0 {
		src = a.idx
	}
	fetched := a.idxFetched
	nPhotos := len(a.photoList)
	nIdx := len(a.idx)
	saved := a.photoSaved
	a.mu.RUnlock()

	counts := map[string]int{}
	withBC, withOCR := 0, 0
	for _, e := range src {
		counts[e.Dir]++
		if len(e.Barcodes) > 0 {
			withBC++
		}
		t := strings.TrimSpace(e.Text)
		if t != "" && !strings.HasPrefix(e.Text, "[OCR GAGAL]") {
			withOCR++
		}
	}
	dirs := make([]dirStat, 0, len(counts))
	for d, c := range counts {
		dirs = append(dirs, dirStat{Dir: d, Count: c})
	}
	sort.SliceStable(dirs, func(i, j int) bool {
		if dirs[i].Count != dirs[j].Count {
			return dirs[i].Count > dirs[j].Count
		}
		return dirs[i].Dir < dirs[j].Dir
	})
	roots := cfg.PhotoRoots()
	return statsResp{Total: len(src), WithOCR: withOCR, WithBarcodes: withBC,
		IndexFetchedAt: fetched, Photos: nPhotos, OCRIndexed: nIdx, Dirs: dirs,
		PhotosSavedAt: saved, PhotosFolderID: firstRootID(roots),
		PhotosFolderNm: photoRootNames(roots), PhotosFolders: roots,
		DataFolderID:   cfg.DataFolderID,
		DataFolderName: cfg.FolderName, PhotosFolderSet: len(roots) > 0,
		AliasCount: len(a.loadAliasMap()), WithoutOCR: len(src) - withOCR}
}

func (a *App) gdriveStatus(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	cs := a.loadClientSecret()
	tok := a.loadToken()
	base := map[string]any{
		"configured":       cs != nil,
		"email":            nil,
		"folderName":       cfg.FolderName,
		"admin":            a.isAdmin(r),
		"photosFolderId":   firstRootID(cfg.PhotoRoots()),
		"photosFolderName": photoRootNames(cfg.PhotoRoots()),
		"photosFolders":    cfg.PhotoRoots(),
	}
	if tok != nil && tok.Email != "" {
		base["email"] = tok.Email
	}
	if tok == nil || tok.RefreshToken == "" {
		base["authorized"] = false
		sendJSON(w, 200, base)
		return
	}
	token, err := a.gdriveAccessToken()
	if err != nil || token == "" {
		base["authorized"] = false
		sendJSON(w, 200, base)
		return
	}
	if d, err := a.driveAboutUser(token); err == nil && d != "" {
		base["email"] = d
		base["authorized"] = true
	} else {
		base["authorized"] = false
	}
	sendJSON(w, 200, base)
}

func (a *App) driveAboutUser(token string) (string, error) {

	aboutUserMu.Lock()
	if aboutUserEmail != "" && time.Since(aboutUserAt) < aboutUserTTL {
		email := aboutUserEmail
		aboutUserMu.Unlock()
		return email, nil
	}
	aboutUserMu.Unlock()

	req, _ := http.NewRequest("GET", "https://www.googleapis.com/drive/v3/about?fields=user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var d struct {
		User struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return "", err
	}

	aboutUserMu.Lock()
	aboutUserEmail = d.User.EmailAddress
	aboutUserAt = time.Now()
	aboutUserMu.Unlock()

	return d.User.EmailAddress, nil
}

func (a *App) gdriveAuth(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) {
		sendErr(w, 403, "Setup hanya bisa dari localhost.")
		return
	}
	u, err := a.gdriveAuthURL()
	if err != nil || u == "" {
		sendErr(w, 400, "client_secret.json belum ada.")
		return
	}
	sendJSON(w, 200, map[string]string{"url": u})
}

func (a *App) gdriveCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	a.mu.RLock()
	st := a.oauth
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if code == "" || st == nil || state != st.state {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("<h3>Autentikasi gagal atau kedaluwarsa. Coba lagi.</h3>"))
		return
	}
	if err := a.gdriveExchange(code); err != nil {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("<h3>Gagal: " + err.Error() + "</h3>"))
		return
	}
	_, _ = w.Write([]byte("<h3>Berhasil terhubung ke Google Drive!</h3><p>Tab ini bisa ditutup.</p>"))
}

func (a *App) gdriveList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	dq := "mimeType='application/vnd.google-apps.folder'"
	if q != "" {
		dq += " and name contains '" + quoteDriveQuery(q) + "'"
	}
	p := url.Values{}
	p.Set("q", dq)
	p.Set("fields", "files(id,name)")
	p.Set("pageSize", "50")
	p.Set("orderBy", "name")
	resp, err := a.gdriveAPI("files", p.Encode())
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	var d struct {
		Files []driveFile `json:"files"`
	}
	if err := json.Unmarshal(resp.body, &d); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	out := make([]map[string]string, 0, len(d.Files))
	for _, f := range d.Files {
		out = append(out, map[string]string{"id": f.ID, "name": f.Name})
	}
	sendJSON(w, 200, map[string]any{"files": out})
}

func (a *App) servePhoto(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("Bad request"))
		return
	}
	resp, err := a.driveDownloadURL(id)
	if err != nil || resp == nil {
		if err == nil {
			w.WriteHeader(401)
		} else {
			w.WriteHeader(502)
		}
		_, _ = w.Write([]byte("Error"))
		return
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}

	if strings.Contains(strings.ToLower(ct), "text/html") {
		w.WriteHeader(502)
		_, _ = w.Write([]byte("Foto tidak bisa diambil dari Drive"))
		return
	}
	w.Header().Set("Content-Type", ct)
	if strings.HasPrefix(strings.ToLower(ct), "image/") {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}

	if r.URL.Query().Get("download") == "1" {

		w.Header().Set("Content-Disposition",
			contentDispName(dispName(r.URL.Query().Get("name"), id)))
	}
	w.WriteHeader(200)
	_, _ = io.Copy(w, resp.Body)
}

func dispName(name, id string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		n = id
		if i := strings.LastIndexAny(n, "/\\"); i >= 0 {
			n = n[i+1:]
		}
	}
	n = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 || r < 32 {
			return -1
		}
		return r
	}, n)
	n = strings.TrimSpace(n)
	if n == "" || n == "." || n == ".." {
		return "foto.jpg"
	}

	if len(n) > 180 {
		n = n[:180]
	}
	if !strings.Contains(n, ".") {
		n += ".jpg"
	}
	return n
}

func contentDispName(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' || r == 0x7f {
			return -1
		}
		return r
	}, name)
	asciiOnly := true
	for _, r := range safe {
		if r > 126 {
			asciiOnly = false
			break
		}
	}
	if asciiOnly {
		return `attachment; filename="` + safe + `"`
	}

	fb := strings.Map(func(r rune) rune {
		if r > 126 || r < 32 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, safe)
	return `attachment; filename="` + fb + `"; filename*=UTF-8''` +
		url.PathEscape(safe)
}

func (a *App) batchCheckAPI(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Rows []batchRowIn `json:"rows"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, "Body tidak valid.")
		return
	}
	rows := make([]batchRowIn, 0, len(payload.Rows))
	for _, row := range payload.Rows {
		row.Name = strings.TrimSpace(row.Name)
		row.Desc = strings.TrimSpace(row.Desc)
		if row.Name != "" || row.Desc != "" {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		sendErr(w, 400, "rows wajib diisi.")
		return
	}
	sendJSON(w, 200, a.batchCheck(rows))
}

func (a *App) batchExportAPI(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		FolderID string            `json:"folderId"`
		Items    []batchExportItem `json:"items"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, "Body tidak valid.")
		return
	}
	if payload.FolderID == "" || len(payload.Items) == 0 {
		sendErr(w, 400, "folderId dan items wajib diisi.")
		return
	}
	res, err := a.batchExport(payload.FolderID, payload.Items)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, res)
}

func (a *App) gdriveToken(w http.ResponseWriter) {
	t, err := a.gdriveAccessToken()
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	if t == "" {
		sendErr(w, 401, "Belum terhubung.")
		return
	}
	sendJSON(w, 200, map[string]string{"token": t})
}

func (a *App) gdriveRevoke(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) {
		sendErr(w, 403, "Hanya admin.")
		return
	}
	_ = os.Remove(a.tokenPath())
	tokenMu.Lock()
	tokenCache = nil
	tokenMu.Unlock()
	invalidateAboutUser()
	sendJSON(w, 200, map[string]any{"ok": true, "msg": "Token Drive dihapus. Bisa hubungkan akun lain."})
}

func (a *App) gdriveSecret(w http.ResponseWriter, r *http.Request) {
	if !a.isAdmin(r) {
		sendErr(w, 403, "Setup hanya bisa dari localhost.")
		return
	}
	var obj map[string]any
	if err := readJSONBody(r, &obj); err != nil {
		sendErr(w, 400, "Bukan client_secret.json yang valid.")
		return
	}
	inst, _ := obj["installed"].(map[string]any)
	if inst == nil || inst["client_id"] == "" || inst["client_secret"] == "" {
		sendErr(w, 400, "Bukan client_secret.json yang valid.")
		return
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	if err := os.WriteFile(filepath.Join(a.root, "client_secret.json"), b, 0o644); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) cleanerOut(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	a.cleanOutsMu.Lock()
	rec := a.cleanOuts[id]
	a.cleanOutsMu.Unlock()
	if rec == nil {
		sendErr(w, 404, "Tidak ditemukan / kedaluwarsa")
		return
	}
	sendJSON(w, 200, map[string]any{
		"outList": rec.list, "inRows": rec.inRows, "outRows": rec.outRow,
	})
}

func (a *App) gdriveRefreshIndex(w http.ResponseWriter, r *http.Request) {
	force := qBool(r, "force")
	ph, perr := a.refreshPhotoList(force)
	if perr != nil {
		sendErr(w, 500, perr.Error())
		return
	}
	_, ierr := a.refreshIndex(force)
	sendJSON(w, 200, map[string]any{
		"cached":     ph.Cached,
		"total":      len(a.snapshotUniverse()),
		"withOcr":    len(a.snapshotIdx()),
		"photos":     len(a.snapshotUniverse()),
		"indexError": nullableErr(ierr),
		"photoError": nil,
	})
}

func nullableErr(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}

func (a *App) gdriveSyncPhotos(w http.ResponseWriter, r *http.Request) {
	res, err := a.refreshPhotoList(true)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, res)
}

func (a *App) setPhotos(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		FolderID   string         `json:"folderId"`
		FolderName string         `json:"folderName"`
		Folders    []PhotosFolder `json:"folders"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, "Body tidak valid.")
		return
	}
	roots := payload.Folders
	if len(roots) == 0 && payload.FolderID != "" {
		roots = []PhotosFolder{{ID: payload.FolderID, Name: payload.FolderName}}
	}

	clean := make([]PhotosFolder, 0, len(roots))
	seen := map[string]bool{}
	for _, f := range roots {
		if f.ID == "" || seen[f.ID] {
			continue
		}
		seen[f.ID] = true
		clean = append(clean, PhotosFolder{ID: f.ID, Name: f.Name})
	}
	roots = clean

	a.mu.RLock()
	oldKey := photoRootKey(a.cfg.PhotoRoots())
	a.mu.RUnlock()
	newKey := photoRootKey(roots)

	a.mu.Lock()
	a.cfg.setPhotoRoots(roots)

	a.photoMeta = ""
	a.buildPhotoUniverse()
	a.mu.Unlock()
	a.saveConfig()

	if oldKey != newKey {
		_ = os.Remove(a.photoListPath())
	}
	a.resetSearchIndex()

	folder, err := a.ensureDataFolder()
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	content, _ := json.Marshal(map[string]any{
		"photosFolderId":   firstRootID(roots),
		"photosFolderName": firstRootName(roots),
		"photosFolders":    roots,
	})
	if ex, err := a.findFile("config.json", folder); err == nil && ex != nil {
		_ = a.trashFile(ex.ID)
	}
	if _, err := a.uploadBytes(folder, "config.json", "application/json; charset=UTF-8", content); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{
		"ok":               "true",
		"photosFolderId":   firstRootID(roots),
		"photosFolderName": photoRootNames(roots),
		"photosFolders":    roots,
		"photosFolderSet":  len(roots) > 0,
	})
}

func firstRootID(roots []PhotosFolder) string {
	if len(roots) == 0 {
		return ""
	}
	return roots[0].ID
}

func firstRootName(roots []PhotosFolder) string {
	if len(roots) == 0 {
		return ""
	}
	return roots[0].Name
}

type batchExportItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	ExportName string `json:"exportName"`
}

func (a *App) batchExport(folderID string, items []batchExportItem) (map[string]any, error) {
	type doneItem struct {
		ID   string `json:"id"`
		Link string `json:"link"`
	}
	done := []doneItem{}
	fails := []string{}
	reUnsafe := regexp.MustCompile(`[<>:"/\\|?*]+`)
	reExt := regexp.MustCompile(`\.[^.]+$`)
	for _, it := range items {
		base := it.Name
		if base == "" {
			base = it.Path
		}
		if base == "" {
			base = "foto"
		}
		ext := reExt.FindString(base)
		if ext == "" {
			ext = ".jpg"
		}
		name := it.ExportName
		if name == "" {
			name = base
		}
		name = reUnsafe.ReplaceAllString(name, "_")
		name = sliceRunes(name, 120) + ext
		f, err := a.copyToFolder(it.ID, folderID, name)
		if err != nil {
			label := it.Name
			if label == "" {
				label = it.Path
			}
			fails = append(fails, label+": "+err.Error())
			continue
		}
		done = append(done, doneItem{ID: f.ID, Link: "https://drive.google.com/file/d/" + f.ID + "/view"})
	}
	return map[string]any{"done": done, "fails": fails}, nil
}

func (a *App) loadPedomanCache() *pedomanCache {
	if a.db == nil {
		return nil
	}
	ctx, cancel := ctxTimeout(60 * time.Second)
	defer cancel()
	rows, err := a.db.QueryContext(ctx,
		`SELECT kode_barang, nama_barang, merek_barang FROM pedoman_items`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	pc := &pedomanCache{Codes: map[string]bool{}}
	names := newSet()
	mereks := newSet()
	for rows.Next() {
		var k, n, m sql.NullString
		if err := rows.Scan(&k, &n, &m); err != nil {
			continue
		}
		if v := normKode(nullStr(k)); v != "" {
			pc.Codes[v] = true
		}
		if v := normStr(nullStr(n)); v != "" {
			names.Add(v)
		}
		if v := normStr(nullStr(m)); v != "" {
			mereks.Add(v)
		}
		pc.Total++
	}
	if pc.Total == 0 {
		return nil
	}
	pc.Names = names.List()
	pc.Mereks = mereks.List()
	return pc
}

func (a *App) cleanProcess(w http.ResponseWriter, r *http.Request) {
	parts, _, ok := multipartParts(r)
	if !ok {
		sendErr(w, 400, "Missing boundary")
		return
	}
	var filePart *mpPart
	for i := range parts {
		if parts[i].filename != "" {
			filePart = &parts[i]
			break
		}
	}
	if filePart == nil {
		sendErr(w, 400, "File tidak ditemukan")
		return
	}
	var rawOpts map[string]any
	for i := range parts {
		if parts[i].name == "opts" {
			_ = json.Unmarshal(parts[i].data, &rawOpts)
			break
		}
	}
	opts := cleanOpts{}
	if rawOpts != nil {
		opts.CodeCols, _ = rawOpts["codeCols"].(string)
		opts.DescCols, _ = rawOpts["descCols"].(string)
		opts.RemoveBlank, _ = rawOpts["removeBlank"].(bool)
		opts.RemoveEmpty, _ = rawOpts["removeEmpty"].(bool)
		opts.KeepSnj, _ = rawOpts["keepSnj"].(bool)
		opts.FilterByPedoman, _ = rawOpts["filterByPedoman"].(bool)
	}
	if opts.FilterByPedoman {
		ped := a.loadPedomanCache()
		if ped == nil {
			sendErr(w, 400, "Pedoman (whitelist) belum di-upload. Upload dulu di section Pedoman SNJ.")
			return
		}
		opts.Pedoman = ped
	}
	res, err := buildCleanedExcel(filePart.data, opts)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	base := strings.TrimSuffix(filePart.filename, filepathExt(filePart.filename))
	outName := base + " - bersih.xlsx"
	outID := ""
	if len(res.OutList) > 0 {
		a.cleanOutsMu.Lock()
		a.cleanSeq++
		outID = itoa(int(a.cleanSeq)) + "-" + itoa(int(time.Now().UnixMilli()))
		a.cleanOuts[outID] = &cleanOutRec{list: res.OutList, inRows: res.InRows, outRow: res.OutRows}
		if len(a.cleanOuts) > 50 {
			keys := sortedKeys(a.cleanOuts)
			for _, k := range keys[:len(keys)-50] {
				delete(a.cleanOuts, k)
			}
		}
		a.cleanOutsMu.Unlock()
	}
	h := w.Header()
	h.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	h.Set("Content-Disposition", `attachment; filename="`+outName+`"`)
	h.Set("X-Clean-Rows-In", itoa(res.InRows))
	h.Set("X-Clean-Rows-Out", itoa(res.OutRows))
	h.Set("X-Clean-Dash-In", itoa(res.DashIn))
	h.Set("X-Clean-Dash-Out", itoa(res.DashOut))
	h.Set("X-Clean-Cols-Out", itoa(res.OutCols))
	h.Set("X-Clean-Sheets", strings.Join(res.Sheets, ", "))
	h.Set("X-Clean-Name", outName)
	h.Set("X-Clean-Out-Count", itoa(len(res.OutList)))
	h.Set("X-Clean-Out-Id", outID)
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	_, _ = w.Write(res.Buffer)
}

func filepathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
