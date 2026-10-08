package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultPort = 8877

func listenPort() int {
	if v := os.Getenv("CTF_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultPort
}

func sendJSON(w http.ResponseWriter, code int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"JSON error"}`, 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func sendErr(w http.ResponseWriter, code int, msg string) {
	sendJSON(w, code, map[string]string{"error": msg})
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 256<<20))
}

func readJSONBody(r *http.Request, v any) error {
	b, err := readBody(r)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}

func qInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func qBool(r *http.Request, key string) bool {
	switch strings.ToLower(r.URL.Query().Get(key)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *App) isAdmin(r *http.Request) bool {
	if s := a.sessionFromReq(r); s != nil {
		return true
	}
	if isLocalhost(r) {
		return true
	}
	pw := secretValue("CTF_ADMIN_PASSWORD")
	if pw == "" {

		return false
	}
	got := r.Header.Get("X-Admin-Password")
	if got == "" {
		got = r.URL.Query().Get("admin")
	}
	if got == "" {
		if ck, err := r.Cookie("ctf_admin"); err == nil {
			got = ck.Value
		}
	}
	return got != "" && hmac.Equal([]byte(got), []byte(pw))
}

var staticFiles = map[string]string{

	"/":                  "user.html",
	"/user":              "user.html",
	"/user.html":         "user.html",
	"/user.js":           "user.js",
	"/dashboard":         "dashboard.html",
	"/dashboard.html":    "dashboard.html",
	"/login":             "login.html",
	"/login.html":        "login.html",
	"/ctf":               "index.html",
	"/ctf.html":          "index.html",
	"/index.html":        "index.html",
	"/batch":             "batch.html",
	"/batch.html":        "batch.html",
	"/mc":                "mc.html",
	"/mc.html":           "mc.html",
	"/clean":             "clean.html",
	"/clean.html":        "clean.html",
	"/accounts":          "accounts.html",
	"/accounts.html":     "accounts.html",
	"/accounts.js":       "accounts.js",
	"/app.js":            "app.js",
	"/account.js":        "account.js",
	"/batch.js":          "batch.js",
	"/mc.js":             "mc.js",
	"/bg-dashboard.jpeg": "bg-dashboard.jpeg",
}

func (a *App) serveStatic(w http.ResponseWriter, r *http.Request) {
	name, ok := staticFiles[r.URL.Path]
	if !ok {
		sendErr(w, 404, "Not found")
		return
	}
	full := filepath.Join(a.root, name)
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		sendErr(w, 404, "Not found")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", fmt.Sprintf(`"%d-%d"`, st.ModTime().Unix(), st.Size()))

	f, err := os.Open(full)
	if err != nil {
		sendErr(w, 404, "Not found")
		return
	}
	defer f.Close()
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if adminPages[r.URL.Path] && !a.requirePageLogin(w, r) {
			return
		}

		if superPages[r.URL.Path] && !a.requirePageSuper(w, r) {
			return
		}
		if _, ok := staticFiles[r.URL.Path]; ok {
			a.serveStatic(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			a.apiRouter(w, r)
			return
		}

		if r.URL.Path == "" || r.URL.Path == "/" {
			a.serveStatic(w, r)
			return
		}
		if filepath.Ext(r.URL.Path) == "" {
			if _, ok := staticFiles[r.URL.Path]; ok {
				a.serveStatic(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})
	return a.withSecurity(mux)
}

func (a *App) withSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.Contains(r.URL.Path, "..") {
			sendErr(w, 400, "Bad path")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) apiRouter(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path

	if !publicAPI[p] && !authAPI[p] {
		if a.requireAdmin(w, r) == nil {
			return
		}
	}

	switch {

	case p == "/api/mmd/save" && r.Method == http.MethodPost:
		a.mmdSaveAPI(w, r)

	case p == "/api/user/summary" && r.Method == http.MethodGet:
		a.userSummary(w, r)
	case p == "/api/user/masterlists" && r.Method == http.MethodGet:
		a.userMasterlists(w, r)
	case p == "/api/user/compares" && r.Method == http.MethodGet:
		a.userCompareList(w, r)
	case p == "/api/user/master-items" && r.Method == http.MethodGet:
		a.userMasterItems(w, r)
	case p == "/api/user/export-ml" && r.Method == http.MethodGet:
		a.userExportML(w, r)
	case p == "/api/user/mmd-items" && r.Method == http.MethodGet:
		a.userMMDItems(w, r)
	case p == "/api/user/mmd-meta" && r.Method == http.MethodGet:
		a.userMMDMeta(w, r)
	case p == "/api/user/mmd-revisi" && r.Method == http.MethodGet:
		a.userMMDRevisi(w, r)
	case p == "/api/user/export-mmd" && r.Method == http.MethodGet:
		a.userExportMMD(w, r)

	case p == "/api/auth/login" && r.Method == http.MethodPost:
		a.authLoginAPI(w, r)
	case p == "/api/auth/logout" && r.Method == http.MethodPost:
		a.authLogoutAPI(w, r)
	case p == "/api/auth/me" && r.Method == http.MethodGet:
		a.authMeAPI(w, r)
	case p == "/api/auth/change-password" && r.Method == http.MethodPost:
		a.authChangePasswordAPI(w, r)

	case p == "/api/auth/accounts" && r.Method == http.MethodGet:
		a.accountsListAPI(w, r)
	case p == "/api/auth/accounts/create" && r.Method == http.MethodPost:
		a.accountsCreateAPI(w, r)
	case p == "/api/auth/accounts/update" && r.Method == http.MethodPost:
		a.accountsUpdateAPI(w, r)
	case p == "/api/auth/accounts/delete" && r.Method == http.MethodPost:
		a.accountsDeleteAPI(w, r)
	case p == "/api/auth/accounts/logout-all" && r.Method == http.MethodPost:
		a.accountsLogoutAllAPI(w, r)

	case p == "/api/search" && r.Method == http.MethodGet:
		q := r.URL.Query().Get("q")

		if strings.TrimSpace(q) == "" {
			sendJSON(w, 200, searchResp{Results: []photoResult{}, Groups: []groupHit{}})
			return
		}
		if !a.waitReady() {
			sendJSON(w, 503, map[string]string{"error": "Data masih dimuat, coba lagi sebentar."})
			return
		}
		sendJSON(w, 200, a.search(q))

	case p == "/api/stats" && r.Method == http.MethodGet:
		a.waitReady()
		sendJSON(w, 200, a.stats())

	case p == "/api/index/refresh":
		a.gdriveRefreshIndex(w, r)
	case p == "/api/photo":
		a.servePhoto(w, r)

	case p == "/api/batch/check" && r.Method == http.MethodPost:
		a.batchCheckAPI(w, r)
	case p == "/api/batch/export" && r.Method == http.MethodPost:
		a.batchExportAPI(w, r)

	case p == "/api/gdrive/status":
		a.gdriveStatus(w, r)
	case p == "/api/gdrive/token":
		a.gdriveToken(w)
	case p == "/api/gdrive/revoke" && r.Method == http.MethodPost:
		a.gdriveRevoke(w, r)
	case p == "/api/gdrive/secret" && r.Method == http.MethodPost:
		a.gdriveSecret(w, r)
	case p == "/api/gdrive/auth":
		a.gdriveAuth(w, r)
	case p == "/api/gdrive/callback":
		a.gdriveCallback(w, r)
	case p == "/api/gdrive/folders":
		a.gdriveList(w, r)
	case p == "/api/gdrive/setphotos":
		a.setPhotos(w, r)

	case p == "/api/clean/out" && r.Method == http.MethodGet:
		a.cleanerOut(w, r)
	case p == "/api/clean/process" && r.Method == http.MethodPost:
		a.cleanProcess(w, r)

	case p == "/api/pedoman/upload" && r.Method == http.MethodPost:
		a.mcUploadPedoman(w, r)
	case p == "/api/pedoman/list" && r.Method == http.MethodGet:
		a.pedomanList(w, r)
	case strings.HasPrefix(p, "/api/pedoman/delete/") && r.Method == http.MethodPost:
		a.pedomanDelete(w, r, strings.TrimPrefix(p, "/api/pedoman/delete/"))

	case p == "/api/mc/upload-ml" && r.Method == http.MethodPost:
		a.mcUploadML(w, r)
	case p == "/api/mc/upload-pl" && r.Method == http.MethodPost:
		a.mcUploadPL(w, r)
	case strings.HasPrefix(p, "/api/mc/ml-status/") && r.Method == http.MethodGet:
		a.uploadStatus(w, r, strings.TrimPrefix(p, "/api/mc/ml-status/"), "masterlist_uploads")
	case strings.HasPrefix(p, "/api/mc/pl-status/") && r.Method == http.MethodGet:
		a.uploadStatus(w, r, strings.TrimPrefix(p, "/api/mc/pl-status/"), "packing_uploads")
	case p == "/api/mc/dbstatus" && r.Method == http.MethodGet:
		st := pgStatus()
		sendJSON(w, 200, map[string]any{
			"status":    st,
			"ready":     st == "ready",
			"message":   pgInfoMsg(st),
			"container": pgContainer,
		})
	case p == "/api/mc/pl-uploads" && r.Method == http.MethodGet:
		if a.requireDB(w) == nil {
			return
		}
		uploads := a.queryUploads(`SELECT id, filename, uploaded_at, 0 AS approved, total_rows, status FROM packing_uploads ORDER BY uploaded_at DESC`)
		if uploads == nil {
			uploads = []uploadRow{}
		}
		sendJSON(w, 200, map[string]any{"uploads": uploads})
	case p == "/api/mc/masterlists" && r.Method == http.MethodGet:
		if a.requireDB(w) == nil {
			return
		}
		uploads := a.queryUploads(`SELECT id, filename, sheet_name, uploaded_at, approved, total_rows, status FROM masterlist_uploads ORDER BY uploaded_at DESC`)
		if uploads == nil {
			uploads = []uploadRow{}
		}
		sendJSON(w, 200, map[string]any{"uploads": uploads})
	case p == "/api/mc/approve-ml" && r.Method == http.MethodPost:
		a.mcApproveML(w, r)
	case p == "/api/mc/save-master-items" && r.Method == http.MethodPost:
		a.mcSaveMasterItems(w, r)
	case p == "/api/mc/approve-result" && r.Method == http.MethodPost:
		a.mcApproveResult(w, r)
	case p == "/api/mc/confirm-review" && r.Method == http.MethodPost:
		a.mcConfirmReview(w, r)
	case p == "/api/mc/reject-review" && r.Method == http.MethodPost:
		a.mcRejectReview(w, r)
	case p == "/api/mc/delete-ml" && r.Method == http.MethodPost:
		a.mcDelete(w, r, "ml")
	case p == "/api/mc/delete-pl" && r.Method == http.MethodPost:
		a.mcDelete(w, r, "pl")
	case p == "/api/mc/clear-all" && r.Method == http.MethodPost:
		a.mcClearAll(w, r)
	case p == "/api/mc/compare" && r.Method == http.MethodPost:
		var payload comparePayload
		if err := readJSONBody(r, &payload); err != nil {
			sendErr(w, 400, err.Error())
			return
		}
		resp, err := a.runCompare(payload, nil)
		if err != nil {
			sendErr(w, 400, err.Error())
			return
		}
		sendJSON(w, 200, resp)
	case p == "/api/mc/results" && r.Method == http.MethodGet:
		a.mcResultsList(w, r)
	case p == "/api/mc/export" && r.Method == http.MethodPost:
		a.mcExport(w, r)
	case strings.HasPrefix(p, "/api/mc/results/"):
		a.mcResultByID(w, r, strings.TrimPrefix(p, "/api/mc/results/"))

	default:
		sendErr(w, 404, "Not found: "+p)
	}
}

func (a *App) uploadStatus(w http.ResponseWriter, r *http.Request, idStr, table string) {
	if a.db == nil {
		sendJSON(w, 200, map[string]string{"status": "unknown"})
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || id <= 0 {
		sendErr(w, 404, "Not found")
		return
	}
	ctx, cancel := ctxTimeout(15 * time.Second)
	defer cancel()
	var out struct {
		ID        int    `json:"id"`
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		TotalRows int    `json:"total_rows"`
	}
	if err := a.db.QueryRowContext(ctx,
		`SELECT id, filename, status, total_rows FROM `+table+` WHERE id=$1`, id).
		Scan(&out.ID, &out.Filename, &out.Status, &out.TotalRows); err != nil {
		sendErr(w, 404, "Not found")
		return
	}
	sendJSON(w, 200, out)
}

func freePort(p int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func main() {
	root := exeDir()
	if v := os.Getenv("CTF_ROOT"); v != "" {
		root = v
	}
	a := newApp(root)

	go func() {
		a.db = openDB()
		applyAuthSchema(a.db)
		applyMMDSchema(a.db)
		backfillDedupMMD(a.db)
		a.loadCache()
		n := len(a.getSearchIndex().entries)
		close(a.ready)
		logf("Data siap: %d foto, search index %d entri", len(a.snapshotUniverse()), n)
	}()

	if os.Getenv("CTF_NO_PG_AUTOSTART") != "1" {
		go a.warmPostgres()
	}

	port := listenPort()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           a.handler(),
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Gagal dengar di %s: %v", addr, err)
	}
	logf("SNJSYNC (Go) siap di http://localhost:%d", port)
	logf("Root: %s", root)

	if os.Getenv("CTF_NO_BROWSER") != "1" {
		go openURL(fmt.Sprintf("http://localhost:%d/?r=%d", port, time.Now().UnixNano()))
	}
	if err := srv.Serve(ln); err != nil {
		log.Fatal(err)
	}
}

func openURL(u string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
