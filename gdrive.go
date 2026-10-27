package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	driveScope = "https://www.googleapis.com/auth/drive"



	driveRedirect  = "http://localhost:8877/api/gdrive/callback"
	callbackPath   = "/api/gdrive/callback"
	indexFileName  = "index.json"
)




func driveRedirectFor(r *http.Request) string {
	if v := strings.TrimSpace(secretValue("CTF_OAUTH_REDIRECT")); v != "" {
		return v
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "https" || p == "http" {
		scheme = p
	}
	return scheme + "://" + r.Host + callbackPath
}

type driveToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expiry       int64  `json:"expiry"`
	Email        string `json:"email"`
}

type clientSecret struct {
	Installed struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
	Web struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"web"`
}




var (
	tokenMu    sync.Mutex
	tokenCache *driveToken

	aboutUserMu    sync.Mutex
	aboutUserEmail string
	aboutUserAt    time.Time
)

const aboutUserTTL = 10 * time.Minute

func invalidateAboutUser() {
	aboutUserMu.Lock()
	aboutUserEmail = ""
	aboutUserAt = time.Time{}
	aboutUserMu.Unlock()
}

func (cs *clientSecret) CID() string {
	if cs.Web.ClientID != "" {
		return cs.Web.ClientID
	}
	return cs.Installed.ClientID
}

func (cs *clientSecret) CSecret() string {
	if cs.Web.ClientID != "" {
		return cs.Web.ClientSecret
	}
	return cs.Installed.ClientSecret
}

func (a *App) loadClientSecret() *clientSecret {
	if v := os.Getenv("CTF_CLIENT_SECRET"); v != "" {
		var cs clientSecret
		if json.Unmarshal([]byte(v), &cs) == nil && (cs.Web.ClientID != "" || cs.Installed.ClientID != "") {
			return &cs
		}
	}
	b := readJSONSafe(filepath.Join(a.root, "client_secret.json"))
	if b == nil {
		return nil
	}
	var cs clientSecret
	if json.Unmarshal(b, &cs) != nil || (cs.Web.ClientID == "" && cs.Installed.ClientID == "") {
		return nil
	}
	return &cs
}

func (a *App) loadToken() *driveToken {
	if v := os.Getenv("CTF_DRIVE_TOKEN"); v != "" {
		var t driveToken
		if json.Unmarshal([]byte(v), &t) == nil {
			return &t
		}
	}
	b := readJSONSafe(a.tokenPath())
	if b == nil {
		return nil
	}
	var t driveToken
	if json.Unmarshal(b, &t) != nil {
		return nil
	}
	return &t
}

func (a *App) saveToken(t *driveToken) {
	tokenMu.Lock()
	tokenCache = t
	tokenMu.Unlock()
	invalidateAboutUser()
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(a.dataDir, 0o755)
	_ = os.WriteFile(a.tokenPath(), b, 0o600)
}

func (a *App) gdriveAccessToken() (string, error) {
	t := a.loadToken()
	if t == nil {
		return "", nil
	}
	tokenMu.Lock()
	if tokenCache != nil && tokenCache.AccessToken == t.AccessToken {
		t = tokenCache
	}
	tokenMu.Unlock()
	if time.Now().UnixMilli() < t.Expiry {
		return t.AccessToken, nil
	}
	cs := a.loadClientSecret()
	if cs == nil {
		return "", nil
	}
	form := url.Values{}
	form.Set("client_id", cs.CID())
	form.Set("client_secret", cs.CSecret())
	form.Set("refresh_token", t.RefreshToken)
	form.Set("grant_type", "refresh_token")
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var d struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return "", err
	}
	if d.AccessToken == "" {
		return "", nil
	}
	t.AccessToken = d.AccessToken
	exp := d.ExpiresIn
	if exp == 0 {
		exp = 3600
	}
	t.Expiry = time.Now().UnixMilli() + (exp-60)*1000
	a.saveToken(t)
	return t.AccessToken, nil
}

func (a *App) gdriveAuthURL(redirect string) (string, error) {
	cs := a.loadClientSecret()
	if cs == nil {
		return "", nil
	}
	verifier := randomB64(32)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	a.mu.Lock()
	a.oauth = &oauthState{verifier: verifier, state: randomHex(8), redirect: redirect}
	st := a.oauth
	a.mu.Unlock()
	p := url.Values{}
	p.Set("client_id", cs.CID())
	p.Set("redirect_uri", redirect)
	p.Set("response_type", "code")
	p.Set("scope", driveScope)
	p.Set("access_type", "offline")
	p.Set("prompt", "consent")
	p.Set("state", st.state)
	p.Set("code_challenge", challenge)
	p.Set("code_challenge_method", "S256")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + p.Encode(), nil
}

func (a *App) gdriveExchange(code string) error {
	cs := a.loadClientSecret()
	if cs == nil {
		return fmt.Errorf("client_secret.json belum ada")
	}
	a.mu.RLock()
	st := a.oauth
	a.mu.RUnlock()
	form := url.Values{}
	form.Set("client_id", cs.CID())
	form.Set("client_secret", cs.CSecret())
	form.Set("code", code)
	if st != nil {
		form.Set("code_verifier", st.verifier)
	}
	redirect := driveRedirect
	if st != nil && st.redirect != "" {
		redirect = st.redirect
	}
	form.Set("redirect_uri", redirect)
	form.Set("grant_type", "authorization_code")
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var d struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int64  `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return err
	}
	if d.AccessToken == "" || d.RefreshToken == "" {
		if d.ErrorDescription != "" {
			return fmt.Errorf("%s", d.ErrorDescription)
		}
		if d.Error != "" {
			return fmt.Errorf("%s", d.Error)
		}
		return fmt.Errorf("Autentikasi gagal")
	}
	var email string
	if me, err := driveUserInfo(d.AccessToken); err == nil {
		email = me
	}
	exp := d.ExpiresIn
	if exp == 0 {
		exp = 3600
	}
	a.saveToken(&driveToken{AccessToken: d.AccessToken, RefreshToken: d.RefreshToken,
		Expiry: time.Now().UnixMilli() + (exp-60)*1000, Email: email})
	a.mu.Lock()
	a.oauth = nil
	a.mu.Unlock()
	return nil
}

func driveUserInfo(access string) (string, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var me struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", err
	}
	return me.Email, nil
}

type driveResp struct {
	body []byte
	ct   string
}

func (a *App) gdriveAPI(p, params string) (*driveResp, error) {
	token, err := a.gdriveAccessToken()
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("Belum terhubung ke Google.")
	}
	u := "https://www.googleapis.com/drive/v3/" + p
	if params != "" {
		u += "?" + params
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		txt := string(body)
		if len(txt) > 300 {
			txt = txt[:300]
		}
		return nil, fmt.Errorf("Drive HTTP %d: %s", resp.StatusCode, txt)
	}
	return &driveResp{body: body, ct: resp.Header.Get("Content-Type")}, nil
}

func quoteDriveQuery(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

type driveFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
	Size     string `json:"size"`
}

func (a *App) findFolder(name, parentID string) (*driveFile, error) {
	q := "mimeType='application/vnd.google-apps.folder' and name='" + quoteDriveQuery(name) + "'"
	if parentID != "" {
		q += " and '" + parentID + "' in parents"
	} else {
		q += " and 'root' in parents"
	}
	p := url.Values{}
	p.Set("q", q)
	p.Set("fields", "files(id,name)")
	p.Set("pageSize", "50")
	r, err := a.gdriveAPI("files", p.Encode())
	if err != nil {
		return nil, err
	}
	var d struct {
		Files []driveFile `json:"files"`
	}
	if err := json.Unmarshal(r.body, &d); err != nil {
		return nil, err
	}
	if len(d.Files) == 0 {
		return nil, nil
	}
	return &d.Files[0], nil
}

func (a *App) findFile(name, parentID string) (*driveFile, error) {
	q := "name='" + quoteDriveQuery(name) + "'"
	if parentID != "" {
		q += " and '" + parentID + "' in parents"
	} else {
		q += " and 'root' in parents"
	}
	q += " and trashed=false"
	p := url.Values{}
	p.Set("q", q)
	p.Set("fields", "files(id,name,size,mimeType)")
	p.Set("pageSize", "50")
	r, err := a.gdriveAPI("files", p.Encode())
	if err != nil {
		return nil, err
	}
	var d struct {
		Files []driveFile `json:"files"`
	}
	if err := json.Unmarshal(r.body, &d); err != nil {
		return nil, err
	}
	if len(d.Files) == 0 {
		return nil, nil
	}
	return &d.Files[0], nil
}

func (a *App) ensureDataFolder() (string, error) {
	a.mu.RLock()
	cfg := a.cfg
	a.mu.RUnlock()
	if cfg.DataFolderID != "" {
		p := url.Values{}
		p.Set("fields", "id,name")
		if _, err := a.gdriveAPI("files/"+cfg.DataFolderID, p.Encode()); err == nil {
			return cfg.DataFolderID, nil
		}
		a.mu.Lock()
		a.cfg.DataFolderID = ""
		a.mu.Unlock()
	}
	name := cfg.FolderName
	if name == "" {
		name = "CTF"
	}
	f, err := a.findFolder(name, "")
	if err != nil {
		return "", err
	}
	if f == nil {
		token, terr := a.gdriveAccessToken()
		if terr != nil || token == "" {
			return "", fmt.Errorf("Belum terhubung ke Google.")
		}
		body, _ := json.Marshal(map[string]string{"name": name, "mimeType": "application/vnd.google-apps.folder"})
		req, _ := http.NewRequest("POST", "https://www.googleapis.com/drive/v3/files", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		var nf driveFile
		if err := json.NewDecoder(resp.Body).Decode(&nf); err != nil {
			return "", err
		}
		f = &nf
	}
	a.mu.Lock()
	a.cfg.DataFolderID = f.ID
	a.mu.Unlock()
	a.saveConfig()
	return f.ID, nil
}

type refreshResult struct {
	Cached bool   `json:"cached"`
	Total  int    `json:"total"`
	Error  string `json:"error,omitempty"`
}

func (a *App) refreshIndex(force bool) (*refreshResult, error) {
	folderID, err := a.ensureDataFolder()
	if err != nil {
		return &refreshResult{Error: err.Error()}, err
	}
	a.mu.RLock()
	meta, fetched, cur := a.idxMeta, a.idxFetched, len(a.idx)
	a.mu.RUnlock()
	f, err := a.findFile(indexFileName, folderID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return &refreshResult{Total: cur}, nil
	}
	if !force && meta == f.ID && time.Now().UnixMilli()-fetched < 60000 {
		return &refreshResult{Cached: true, Total: cur}, nil
	}
	r, err := a.gdriveAPI("files/"+f.ID+"?alt=media", "")
	if err != nil {
		return nil, err
	}
	var arr []PhotoEntry
	if err := json.Unmarshal(stripBOM(r.body), &arr); err != nil {
		e := fmt.Errorf("index.json isi bukan array.")
		return &refreshResult{Total: cur, Error: e.Error()}, e
	}
	a.mu.Lock()
	a.idx = arr
	a.idxMeta = f.ID
	a.idxFetched = time.Now().UnixMilli()
	a.buildPhotoUniverse()
	a.mu.Unlock()
	if b, err := json.Marshal(arr); err == nil {
		_ = os.WriteFile(a.indexPath(), b, 0o644)
	}
	a.resetSearchIndex()
	return &refreshResult{Cached: false, Total: len(arr)}, nil
}

const dirConcurrency = 10

type dirNode struct {
	id   string
	path string
}

func (a *App) driveChildren(folderID string) ([]driveFile, error) {
	p := url.Values{}
	p.Set("q", "'"+folderID+"' in parents and trashed=false")
	p.Set("fields", "nextPageToken,files(id,name,mimeType,size)")
	p.Set("orderBy", "name")
	p.Set("pageSize", "1000")
	req := p.Encode()
	var out []driveFile
	for {
		r, err := a.gdriveAPI("files", req)
		if err != nil {
			return nil, err
		}
		var d struct {
			Files         []driveFile `json:"files"`
			NextPageToken string      `json:"nextPageToken"`
		}
		if err := json.Unmarshal(r.body, &d); err != nil {
			return nil, err
		}
		out = append(out, d.Files...)
		if d.NextPageToken == "" {
			break
		}
		req += "&pageToken=" + url.QueryEscape(d.NextPageToken)
	}
	return out, nil
}

func (a *App) listDriveFolder(rootID string) ([]PhotoEntry, error) {
	var out []PhotoEntry
	frontier := []dirNode{{id: rootID}}
	seen := map[string]bool{rootID: true}
	for len(frontier) > 0 {
		var next []dirNode
		for i := 0; i < len(frontier); i += dirConcurrency {
			end := i + dirConcurrency
			if end > len(frontier) {
				end = len(frontier)
			}
			type res struct {
				subs   []dirNode
				kids   []driveFile
				parent string
			}
			results := make([]res, 0, end-i)
			for _, f := range frontier[i:end] {
				kids, err := a.driveChildren(f.id)
				if err != nil {
					return nil, err
				}
				var subs []dirNode
				for _, k := range kids {
					if k.MimeType == "application/vnd.google-apps.folder" {
						pth := k.Name
						if f.path != "" {
							pth = f.path + "/" + k.Name
						}
						if !seen[k.ID] {
							seen[k.ID] = true
							subs = append(subs, dirNode{id: k.ID, path: pth})
						}
					}
				}
				results = append(results, res{subs: subs, kids: kids, parent: f.path})
			}
			for _, r := range results {
				next = append(next, r.subs...)
				for _, k := range r.kids {
					if strings.HasPrefix(k.MimeType, "image/") {
						rel := k.Name
						if r.parent != "" {
							rel = r.parent + "/" + k.Name
						}
						sz, _ := strconv.ParseInt(k.Size, 10, 64)
						out = append(out, PhotoEntry{ID: k.ID, Name: k.Name, Dir: r.parent, Path: rel, Size: sz})
					}
				}
			}
		}
		frontier = next
	}
	return out, nil
}

func (a *App) listDriveRoots(roots []PhotosFolder) ([]PhotoEntry, error) {
	gabung := len(roots) > 1
	out := make([]PhotoEntry, 0, 1024)
	seen := map[string]bool{}
	for _, r := range roots {
		list, err := a.listDriveFolder(r.ID)
		if err != nil {
			return nil, err
		}
		pfx := r.Name
		for _, e := range list {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			if gabung && pfx != "" {
				if e.Dir == "" {
					e.Dir = pfx
				} else {
					e.Dir = pfx + "/" + e.Dir
				}
				e.Path = pfx + "/" + e.Path
			}
			out = append(out, e)
		}
	}
	return out, nil
}

func (a *App) refreshPhotoList(force bool) (*refreshResult, error) {
	a.mu.RLock()
	roots := a.cfg.PhotoRoots()
	key := photoRootKey(roots)
	meta := a.photoMeta
	at := a.universeAt
	cur := len(a.photoList)
	a.mu.RUnlock()
	if key == "" {
		e := fmt.Errorf("Folder foto belum dipilih. Buka Settings lalu pilih folder foto dulu.")
		return &refreshResult{Total: cur, Error: e.Error()}, e
	}
	if !force && meta == key && time.Now().UnixMilli()-at < 60000 {
		return &refreshResult{Cached: true, Total: cur}, nil
	}
	if b := readJSONSafe(a.photoListPath()); b != nil && !force {
		var cached struct {
			Root  string       `json:"root"`
			Saved int64        `json:"saved"`
			Files []PhotoEntry `json:"files"`
		}
		if json.Unmarshal(b, &cached) == nil && cached.Root == key && len(cached.Files) > 0 {
			a.mu.Lock()
			a.photoList = cached.Files
			a.photoMeta = key
			a.buildPhotoUniverse()
			a.universeAt = time.Now().UnixMilli()
			tot := len(a.photoList)
			a.mu.Unlock()
			a.resetSearchIndex()
			return &refreshResult{Cached: true, Total: tot}, nil
		}
	}
	list, err := a.listDriveRoots(roots)
	if err != nil {
		return &refreshResult{Total: cur, Error: err.Error()}, err
	}
	a.mu.Lock()
	a.photoList = list
	a.photoMeta = key
	a.buildPhotoUniverse()
	a.universeAt = time.Now().UnixMilli()
	a.mu.Unlock()
	if b, err := json.Marshal(map[string]any{"root": key, "saved": time.Now().UnixMilli(), "files": list}); err == nil {
		_ = os.MkdirAll(a.dataDir, 0o755)
		_ = os.WriteFile(a.photoListPath(), b, 0o644)
	}
	a.resetSearchIndex()
	return &refreshResult{Cached: false, Total: len(list)}, nil
}

func (a *App) driveDownloadURL(fileID string) (*http.Response, error) {
	token, err := a.gdriveAccessToken()
	if err != nil || token == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Belum terhubung ke Google.")
	}
	urls := []string{
		"https://www.googleapis.com/drive/v3/files/" + url.PathEscape(fileID) +
			"?alt=media&supportsAllDrives=true",
		"https://drive.google.com/uc?export=download&confirm=t&id=" + url.QueryEscape(fileID),
	}
	var lastErr error
	for _, u := range urls {
		for attempt := 0; attempt < 3; attempt++ {
			req, _ := http.NewRequest("GET", u, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				lastErr = err
				break
			}
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				if bodyBukanHTML(resp) {
					return resp, nil
				}
				lastErr = fmt.Errorf("Drive mengembalikan halaman login, bukan file")
				break
			}
			code := resp.StatusCode
			resp.Body.Close()
			if code == 503 || code == 429 {
				lastErr = fmt.Errorf("Download gagal: HTTP %d", code)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			lastErr = fmt.Errorf("Download gagal: HTTP %d", code)
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("Download gagal")
	}
	return nil, lastErr
}

func bodyBukanHTML(resp *http.Response) bool {
	head := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") || looksLikeHTML(head) {
		resp.Body.Close()
		return false
	}
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), resp.Body), resp.Body}
	return true
}

func looksLikeHTML(b []byte) bool {
	s := strings.ToLower(strings.TrimSpace(string(b)))
	return strings.HasPrefix(s, "<!doctype") || strings.HasPrefix(s, "<html") ||
		strings.HasPrefix(s, "<head")
}

func (a *App) copyToFolder(fileID, folderID, name string) (*driveFile, error) {
	dl, err := a.driveDownloadURL(fileID)
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(dl.Body)
	dl.Body.Close()
	if err != nil {
		return nil, err
	}
	return a.uploadBytes(folderID, name, "application/octet-stream", buf)
}

func (a *App) uploadBytes(folderID, name, contentType string, content []byte) (*driveFile, error) {
	token, err := a.gdriveAccessToken()
	if err != nil || token == "" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Belum terhubung ke Google.")
	}
	metab, _ := json.Marshal(map[string]any{"name": name, "parents": []string{folderID}})
	req, _ := http.NewRequest("POST", "https://www.googleapis.com/upload/drive/v3/files?uploadType=resumable", bytes.NewReader(metab))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	init, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer init.Body.Close()
	if init.StatusCode < 200 || init.StatusCode > 299 {
		return nil, fmt.Errorf("Init upload gagal: HTTP %d", init.StatusCode)
	}
	loc := init.Header.Get("Location")
	if loc == "" {
		return nil, fmt.Errorf("Init upload gagal: tidak ada location")
	}
	put, err := http.NewRequest("PUT", loc, bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	put.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(put)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("Upload gagal: HTTP %d", resp.StatusCode)
	}
	var f driveFile
	if err := json.NewDecoder(resp.Body).Decode(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

func (a *App) trashFile(fileID string) error {
	token, err := a.gdriveAccessToken()
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("POST", "https://www.googleapis.com/drive/v3/files/"+fileID+"/trash", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func randomB64(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func logf(format string, args ...any) { log.Printf(format, args...) }
