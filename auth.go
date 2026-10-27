package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	roleAdmin = "admin"
	roleSuper = "superadmin"
	roleUser  = "user"

	modCTF = "ctf"
	modMC  = "mc"
	modMMD = "mmd"

	sessCookieName = "snj_session"


	seenThrottle = 5 * time.Minute


	activeMaxDays = 7
	idleMaxDays   = 30
)

var (
	sessAgeOnce sync.Once
	sessAgeSecs int
)




func sessMaxAge() int {
	sessAgeOnce.Do(func() {
		hours := 12
		raw := strings.TrimSpace(os.Getenv("CTF_SESSION_HOURS"))
		if raw == "" {
			raw = strings.TrimSpace(dotEnvValue("CTF_SESSION_HOURS"))
		}
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= 720 {
			hours = n
		}
		sessAgeSecs = hours * 3600
	})
	return sessAgeSecs
}

const authSchemaSQL = `
CREATE TABLE IF NOT EXISTS accounts (
  id SERIAL PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'admin',
  created_at TIMESTAMP DEFAULT NOW(),
  last_login TIMESTAMP
);
CREATE TABLE IF NOT EXISTS auth_sessions (
  token TEXT PRIMARY KEY,
  account_id INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP NOT NULL,
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions(expires_at);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS modules TEXT NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS last_seen TIMESTAMP;
CREATE TABLE IF NOT EXISTS login_events (
  id SERIAL PRIMARY KEY,
  account_id INTEGER NOT NULL,
  at TIMESTAMP DEFAULT NOW(),
  ip TEXT,
  ua TEXT,
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_login_events_acc_at ON login_events(account_id, at);
`

type userSession struct {
	UserID   int      `json:"userId"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Modules  []string `json:"modules"`
}

func (u *userSession) isSuper() bool { return u != nil && u.Role == roleSuper }


func (u *userSession) allAccess() bool {
	return u != nil && (u.Role == roleAdmin || u.Role == roleSuper)
}


func (u *userSession) canModule(mod string) bool {
	if u == nil {
		return false
	}
	if u.allAccess() {
		return true
	}
	for _, m := range u.Modules {
		if m == mod {
			return true
		}
	}
	return false
}


func parseModulesCSV(s string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		switch p {
		case modCTF, modMC, modMMD:
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func modulesCSV(mods []string) string { return strings.Join(mods, ",") }

func applyAuthSchema(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec(authSchemaSQL); err != nil {
		for _, stmt := range splitStatements(authSchemaSQL) {
			if _, err := db.Exec(stmt); err != nil {
				log.Println("auth schema:", err)
			}
		}
	}
	seedDefaultAccounts(db)
	purgeExpiredSessions(db)
}

func purgeExpiredSessions(db *sql.DB) {
	if db == nil {
		return
	}
	res, err := db.Exec(`DELETE FROM auth_sessions WHERE expires_at < NOW()`)
	if err != nil {
		log.Println("auth purge sesi:", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("auth: %d sesi kedaluwarsa dibersihkan", n)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func bcryptHash(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func seedDefaultAccounts(db *sql.DB) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&n); err != nil || n > 0 {
		return
	}
	defaults := []struct {
		user, pass, role, envKey string
	}{
		{envOr("CTF_ADMIN_USER", "admin"), secretValue("CTF_ADMIN_PASSWORD"), roleAdmin, "CTF_ADMIN_PASSWORD"},
		{envOr("CTF_SUPER_USER", "superadmin"), secretValue("CTF_SUPER_PASSWORD"), roleSuper, "CTF_SUPER_PASSWORD"},
	}
	for _, d := range defaults {
		if d.pass == "" {
			log.Printf("auth: akun %s (%s) TIDAK dibuat - set env %s dulu, lalu restart server",
				d.user, d.role, d.envKey)
			continue
		}
		hash, err := bcryptHash(d.pass)
		if err != nil {
			log.Println("auth seed hash:", err)
			continue
		}
		if _, err := db.Exec(
			`INSERT INTO accounts (username, password_hash, role) VALUES ($1,$2,$3)
			 ON CONFLICT (username) DO NOTHING`,
			d.user, hash, d.role,
		); err != nil {
			log.Println("auth seed:", err)
		}
	}
	log.Println("auth: akun dibuat dari env - ganti password setelah login pertama")
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}




func reqIsHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (a *App) createSession(w http.ResponseWriter, r *http.Request, accountID int) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(time.Duration(sessMaxAge()) * time.Second)
	if _, err := a.dbHandle().Exec(
		`INSERT INTO auth_sessions (token, account_id, expires_at) VALUES ($1,$2,$3)`,
		token, accountID, exp,
	); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   reqIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
		MaxAge:   sessMaxAge(),
	})
	return nil
}

func (a *App) sessionFromReq(r *http.Request) *userSession {
	if a == nil || a.dbHandle() == nil {
		return nil
	}
	ck, err := r.Cookie(sessCookieName)
	if err != nil || ck.Value == "" {
		return nil
	}
	var s userSession
	var mods string
	err = a.dbHandle().QueryRow(
		`SELECT acc.id, acc.username, acc.role, COALESCE(acc.modules,'')
		   FROM auth_sessions s
		   JOIN accounts acc ON acc.id = s.account_id
		  WHERE s.token = $1 AND s.expires_at > NOW()`,
		ck.Value,
	).Scan(&s.UserID, &s.Username, &s.Role, &mods)
	if err != nil {
		return nil
	}
	s.Modules = parseModulesCSV(mods)
	a.touchSeen(s.UserID)
	return &s
}



func (a *App) touchSeen(id int) {
	if a == nil || id <= 0 {
		return
	}
	now := time.Now()
	a.seenMu.Lock()
	if a.seenAt == nil {
		a.seenAt = map[int]time.Time{}
	}
	if t, ok := a.seenAt[id]; ok && now.Sub(t) < seenThrottle {
		a.seenMu.Unlock()
		return
	}
	a.seenAt[id] = now
	a.seenMu.Unlock()
	go func() {
		if db := a.dbHandle(); db != nil {
			db.Exec(`UPDATE accounts SET last_seen = NOW() WHERE id = $1`, id)
		}
	}()
}

var (
	trustedProxyOnce sync.Once
	trustedProxyNets []*net.IPNet
)




func loadTrustedProxies() {
	trustedProxyOnce.Do(func() {
		for _, part := range strings.Split(os.Getenv("CTF_TRUSTED_PROXY"), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if !strings.Contains(part, "/") {
				ip := net.ParseIP(part)
				if ip == nil {
					continue
				}
				if v4 := ip.To4(); v4 != nil {
					part = v4.String() + "/32"
				} else {
					part = ip.String() + "/128"
				}
			}
			if _, n, err := net.ParseCIDR(part); err == nil {
				trustedProxyNets = append(trustedProxyNets, n)
			}
		}
	})
}

func remoteIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	return net.ParseIP(host)
}

func isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}
	loadTrustedProxies()
	for _, n := range trustedProxyNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}




func clientIP(r *http.Request) string {
	if isTrustedProxy(remoteIP(r)) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i > 0 {
				xff = xff[:i]
			}
			if ip := strings.TrimSpace(xff); ip != "" {
				return ip
			}
		}
	}
	if ip := remoteIP(r); ip != nil {
		return ip.String()
	}
	return r.RemoteAddr
}

func (a *App) requireUser(w http.ResponseWriter, r *http.Request) *userSession {
	if s := a.sessionFromReq(r); s != nil {
		return s
	}
	sendJSON(w, 401, map[string]string{"error": "Login dulu."})
	return nil
}

func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request) *userSession {
	s := a.requireUser(w, r)
	if s == nil {
		return nil
	}
	if s.Role != roleAdmin && s.Role != roleSuper {
		sendJSON(w, 403, map[string]string{"error": "Akses admin diperlukan."})
		return nil
	}
	return s
}


func (a *App) requireModule(w http.ResponseWriter, r *http.Request, mod string) *userSession {
	s := a.requireUser(w, r)
	if s == nil {
		return nil
	}
	if !s.canModule(mod) {
		sendJSON(w, 403, map[string]string{"error": "Anda tidak punya akses ke modul ini."})
		return nil
	}
	return s
}

func (a *App) requireSuper(w http.ResponseWriter, r *http.Request) *userSession {
	s := a.requireAdmin(w, r)
	if s == nil {
		return nil
	}
	if !s.isSuper() {
		sendJSON(w, 403, map[string]string{"error": "Akses super admin diperlukan."})
		return nil
	}
	return s
}


var loginPages = map[string]bool{
	"/":          true,
	"/user":      true,
	"/user.html": true,
}

var adminPages = map[string]bool{
	"/dashboard":      true,
	"/dashboard.html": true,
	"/ctf":            true,
	"/ctf.html":       true,
	"/index.html":     true,
	"/batch":          true,
	"/batch.html":     true,
	"/mc":             true,
	"/mc.html":        true,
	"/clean":          true,
	"/clean.html":     true,
}





var publicAPI = map[string]bool{}



var moduleAPI = map[string]string{
	"/api/search":            modCTF,
	"/api/photo":             modCTF,
	"/api/user/masterlists":  modMMD,
	"/api/user/compares":     modMC,
	"/api/user/master-items": modMC,
	"/api/user/export-ml":    modMC,
	"/api/user/mmd-items":    modMMD,
	"/api/user/mmd-meta":     modMMD,
	"/api/user/mmd-revisi":   modMMD,
	"/api/user/export-mmd":   modMMD,
}


var loginOnlyAPI = map[string]bool{
	"/api/user/summary": true,
}

var authAPI = map[string]bool{
	"/api/auth/login":           true,
	"/api/auth/logout":          true,
	"/api/auth/me":              true,
	"/api/auth/change-password": true,
}

var superPages = map[string]bool{
	"/accounts":      true,
	"/accounts.html": true,
}

func (a *App) requirePageLogin(w http.ResponseWriter, r *http.Request) bool {
	if a.sessionFromReq(r) != nil {
		return true
	}
	http.Redirect(w, r, "/login", http.StatusFound)
	return false
}



func (a *App) requirePageAdmin(w http.ResponseWriter, r *http.Request) bool {
	s := a.sessionFromReq(r)
	if s == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return false
	}
	if s.Role != roleAdmin && s.Role != roleSuper {
		http.Redirect(w, r, "/", http.StatusFound)
		return false
	}
	return true
}

func (a *App) requirePageSuper(w http.ResponseWriter, r *http.Request) bool {
	s := a.sessionFromReq(r)
	if s == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return false
	}
	if !s.isSuper() {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return false
	}
	return true
}


const (
	loginMaxFails = 5
	loginLockDur  = 15 * time.Minute
	loginFailDur  = 15 * time.Minute
	loginFailsCap = 5000
)

type loginFailRec struct {
	n        int
	lastFail time.Time
	lockedTo time.Time
}

var (
	loginMu    sync.Mutex
	loginFails = map[string]*loginFailRec{}
)


func loginKey(ip, username string) string {
	return ip + "|" + strings.ToLower(strings.TrimSpace(username))
}


func loginLockedUntil(k string) time.Time {
	loginMu.Lock()
	defer loginMu.Unlock()
	rec := loginFails[k]
	if rec == nil {
		return time.Time{}
	}
	if time.Since(rec.lastFail) > loginFailDur {
		delete(loginFails, k)
		return time.Time{}
	}
	return rec.lockedTo
}

func loginRecordFail(k string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	rec := loginFails[k]
	if rec == nil {
		if len(loginFails) >= loginFailsCap {
			for kk, v := range loginFails {
				if time.Since(v.lastFail) > loginFailDur {
					delete(loginFails, kk)
				}
			}
			if len(loginFails) >= loginFailsCap {
				return
			}
		}
		rec = &loginFailRec{}
		loginFails[k] = rec
	}
	rec.n++
	rec.lastFail = time.Now()
	if rec.n >= loginMaxFails {
		rec.lockedTo = time.Now().Add(loginLockDur)
	}
}

func loginReset(k string) {
	loginMu.Lock()
	delete(loginFails, k)
	loginMu.Unlock()
}

func (a *App) authLoginAPI(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		sendErr(w, 400, "Username dan password wajib diisi.")
		return
	}

	lk := loginKey(clientIP(r), username)
	if until := loginLockedUntil(lk); until.After(time.Now()) {
		sendErr(w, 429, "Terlalu banyak percobaan gagal. Coba lagi pukul "+until.Format("15:04")+".")
		return
	}
	if a.dbHandle() == nil {
		sendErr(w, 503, "Database belum siap.")
		return
	}

	var id int
	var hash, role, stored string

	err := a.dbHandle().QueryRow(
		`SELECT id, username, password_hash, role FROM accounts WHERE username=$1`,
		username,
	).Scan(&id, &stored, &hash, &role)
	if err != nil {

		bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"),
			[]byte(password),
		)
		loginRecordFail(lk)
		sendErr(w, 401, "Username atau password salah.")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		loginRecordFail(lk)
		sendErr(w, 401, "Username atau password salah.")
		return
	}
	loginReset(lk)
	purgeExpiredSessions(a.dbHandle())
	if err := a.createSession(w, r, id); err != nil {
		sendErr(w, 500, "Gagal membuat sesi: "+err.Error())
		return
	}
	a.dbHandle().Exec(`UPDATE accounts SET last_login = NOW(), last_seen = NOW() WHERE id=$1`, id)
	a.dbHandle().Exec(
		`INSERT INTO login_events (account_id, at, ip, ua) VALUES ($1, NOW(), $2, $3)`,
		id, clientIP(r), r.UserAgent(),
	)
	a.seenMu.Lock()
	if a.seenAt == nil {
		a.seenAt = map[int]time.Time{}
	}
	a.seenAt[id] = time.Now()
	a.seenMu.Unlock()

	sendJSON(w, 200, map[string]any{
		"ok":       true,
		"username": stored,
		"role":     role,
	})
}

func (a *App) authLogoutAPI(w http.ResponseWriter, r *http.Request) {
	if ck, err := r.Cookie(sessCookieName); err == nil && ck.Value != "" && a.dbHandle() != nil {
		a.dbHandle().Exec(`DELETE FROM auth_sessions WHERE token=$1`, ck.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessCookieName, Value: "", Path: "/",
		HttpOnly: true, Secure: reqIsHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	sendJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) authMeAPI(w http.ResponseWriter, r *http.Request) {
	if s := a.sessionFromReq(r); s != nil {
		sendJSON(w, 200, map[string]any{"loggedIn": true, "session": s})
		return
	}
	sendJSON(w, 200, map[string]any{"loggedIn": false})
}

func (a *App) authChangePasswordAPI(w http.ResponseWriter, r *http.Request) {
	s := a.requireUser(w, r)
	if s == nil {
		return
	}
	oldPw := r.FormValue("oldPassword")
	newPw := r.FormValue("newPassword")
	if err := validatePassword(newPw); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	var hash string
	if err := a.dbHandle().QueryRow(`SELECT password_hash FROM accounts WHERE id=$1`, s.UserID).Scan(&hash); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPw)) != nil {
		sendErr(w, 401, "Password lama salah.")
		return
	}
	newHash, err := bcryptHash(newPw)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	if _, err := a.dbHandle().Exec(`UPDATE accounts SET password_hash=$1 WHERE id=$2`, newHash, s.UserID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}

	a.dbHandle().Exec(`DELETE FROM auth_sessions WHERE account_id=$1`, s.UserID)
	if err := a.createSession(w, r, s.UserID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}
