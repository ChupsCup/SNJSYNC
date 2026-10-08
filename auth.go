package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	roleAdmin = "admin"
	roleSuper = "superadmin"

	sessCookieName = "snj_session"
	sessMaxAge     = 12 * 3600
)

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
`

type userSession struct {
	UserID   int    `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (u *userSession) isSuper() bool { return u != nil && u.Role == roleSuper }

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

func (a *App) createSession(w http.ResponseWriter, accountID int) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	exp := time.Now().Add(sessMaxAge * time.Second)
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
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
		MaxAge:   sessMaxAge,
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
	err = a.dbHandle().QueryRow(
		`SELECT acc.id, acc.username, acc.role
		   FROM auth_sessions s
		   JOIN accounts acc ON acc.id = s.account_id
		  WHERE s.token = $1 AND s.expires_at > NOW()`,
		ck.Value,
	).Scan(&s.UserID, &s.Username, &s.Role)
	if err != nil {
		return nil
	}
	return &s
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

var publicAPI = map[string]bool{
	"/api/search":      true,
	"/api/stats":       true,
	"/api/photo":       true,
	"/api/clean/out":   true,
	"/api/mc/dbstatus": true,

	"/api/user/summary":      true,
	"/api/user/masterlists":  true,
	"/api/user/compares":     true,
	"/api/user/master-items": true,
	"/api/user/export-ml":    true,
	"/api/user/mmd-items":    true,
	"/api/user/mmd-meta":     true,
	"/api/user/mmd-revisi":   true,
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

func (a *App) authLoginAPI(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		sendErr(w, 400, "Username dan password wajib diisi.")
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
		sendErr(w, 401, "Username atau password salah.")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		sendErr(w, 401, "Username atau password salah.")
		return
	}
	purgeExpiredSessions(a.dbHandle())
	if err := a.createSession(w, id); err != nil {
		sendErr(w, 500, "Gagal membuat sesi: "+err.Error())
		return
	}
	a.dbHandle().Exec(`UPDATE accounts SET last_login = NOW() WHERE id=$1`, id)

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
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
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
	if len(newPw) < 6 {
		sendErr(w, 400, "Password baru minimal 6 karakter.")
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
	if err := a.createSession(w, s.UserID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}
