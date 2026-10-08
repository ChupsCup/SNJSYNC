package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	minPassLen = 6
	maxPassLen = 72
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

type accountItem struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
	LastLogin string `json:"lastLogin"`
	SesiAktif int    `json:"sesiAktif"`
	IsYou     bool   `json:"isYou"`
}

func roleLabel(role string) string {
	if role == roleSuper {
		return "Super Admin"
	}
	return "Admin"
}

func validateUsername(u string) error {
	if !usernameRe.MatchString(u) {
		return fmt.Errorf("Username 3-32 karakter, hanya huruf, angka, titik, _ atau -")
	}
	return nil
}

func validatePassword(p string) error {
	if len(p) < minPassLen {
		return fmt.Errorf("Password minimal %d karakter.", minPassLen)
	}
	if len(p) > maxPassLen {
		return fmt.Errorf("Password maksimal %d karakter.", maxPassLen)
	}
	return nil
}

func (a *App) usernameTaken(username string, excludeID int) (bool, error) {
	var n int
	err := a.dbHandle().QueryRow(
		`SELECT COUNT(*) FROM accounts WHERE lower(username)=lower($1) AND id<>$2`,
		username, excludeID,
	).Scan(&n)
	return n > 0, err
}

func (a *App) superCount() (int, error) {
	var n int
	err := a.dbHandle().QueryRow(
		`SELECT COUNT(*) FROM accounts WHERE role=$1`, roleSuper).Scan(&n)
	return n, err
}

func (a *App) accountsListAPI(w http.ResponseWriter, r *http.Request) {
	s := a.requireSuper(w, r)
	if s == nil {
		return
	}
	rows, err := a.dbHandle().Query(`
		SELECT acc.id, acc.username, acc.role,
		       COALESCE(to_char(acc.created_at,'DD/MM/YYYY HH24:MI'),''),
		       COALESCE(to_char(acc.last_login,'DD/MM/YYYY HH24:MI'),''),
		       (SELECT COUNT(*) FROM auth_sessions sx
		         WHERE sx.account_id = acc.id AND sx.expires_at > NOW())
		  FROM accounts acc
		 ORDER BY CASE acc.role WHEN 'superadmin' THEN 0 ELSE 1 END,
		          lower(acc.username)`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	list := []accountItem{}
	for rows.Next() {
		var it accountItem
		if err := rows.Scan(&it.ID, &it.Username, &it.Role,
			&it.CreatedAt, &it.LastLogin, &it.SesiAktif); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		it.IsYou = it.ID == s.UserID
		list = append(list, it)
	}
	sendJSON(w, 200, map[string]any{"accounts": list, "isYou": s.UserID})
}

func (a *App) accountsCreateAPI(w http.ResponseWriter, r *http.Request) {
	if a.requireSuper(w, r) == nil {
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	role := strings.TrimSpace(r.FormValue("role"))
	if role == "" {
		role = roleAdmin
	}
	if role != roleAdmin && role != roleSuper {
		sendErr(w, 400, "Role tidak dikenal.")
		return
	}
	if err := validateUsername(username); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	if err := validatePassword(password); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	taken, err := a.usernameTaken(username, 0)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	if taken {
		sendErr(w, 409, "Username sudah dipakai.")
		return
	}
	hash, err := bcryptHash(password)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	var id int
	if err := a.dbHandle().QueryRow(
		`INSERT INTO accounts (username, password_hash, role) VALUES ($1,$2,$3) RETURNING id`,
		username, hash, role).Scan(&id); err != nil {

		sendErr(w, 409, "Gagal membuat akun: "+err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{
		"ok": true, "id": id, "username": username, "role": role,
		"pesan": "Akun " + roleLabel(role) + " " + username + " dibuat.",
	})
}

func (a *App) accountsUpdateAPI(w http.ResponseWriter, r *http.Request) {
	me := a.requireSuper(w, r)
	if me == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("id")))
	if err != nil || id <= 0 {
		sendErr(w, 400, "ID akun tidak valid.")
		return
	}

	var curUser, curRole string
	if err := a.dbHandle().QueryRow(
		`SELECT username, role FROM accounts WHERE id=$1`, id).
		Scan(&curUser, &curRole); err != nil {
		sendErr(w, 404, "Akun tidak ditemukan.")
		return
	}

	newUser := strings.TrimSpace(r.FormValue("username"))
	newPass := r.FormValue("password")
	newRole := strings.TrimSpace(r.FormValue("role"))

	if newRole == "-" {
		newRole = ""
	}

	changed := []string{}

	if newUser != "" && newUser != curUser {
		if err := validateUsername(newUser); err != nil {
			sendErr(w, 400, err.Error())
			return
		}
		taken, err := a.usernameTaken(newUser, id)
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if taken {
			sendErr(w, 409, "Username sudah dipakai.")
			return
		}
		if _, err := a.dbHandle().Exec(
			`UPDATE accounts SET username=$1 WHERE id=$2`, newUser, id); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		changed = append(changed, "username")
	}

	if newPass != "" {
		if err := validatePassword(newPass); err != nil {
			sendErr(w, 400, err.Error())
			return
		}
		hash, err := bcryptHash(newPass)
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if _, err := a.dbHandle().Exec(
			`UPDATE accounts SET password_hash=$1 WHERE id=$2`, hash, id); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		changed = append(changed, "password")
	}

	if newRole != "" && newRole != curRole {
		if newRole != roleAdmin && newRole != roleSuper {
			sendErr(w, 400, "Role tidak dikenal.")
			return
		}

		if id == me.UserID {
			sendErr(w, 400, "Tidak bisa mengubah role akun sendiri.")
			return
		}

		if curRole == roleSuper {
			n, err := a.superCount()
			if err != nil {
				sendErr(w, 500, err.Error())
				return
			}
			if n <= 1 {
				sendErr(w, 400, "Harus ada minimal satu Super Admin.")
				return
			}
		}
		if _, err := a.dbHandle().Exec(
			`UPDATE accounts SET role=$1 WHERE id=$2`, newRole, id); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		changed = append(changed, "role")
	}

	if len(changed) == 0 {
		sendErr(w, 400, "Tidak ada perubahan.")
		return
	}

	a.dbHandle().Exec(`DELETE FROM auth_sessions WHERE account_id=$1`, id)
	if id == me.UserID {
		if err := a.createSession(w, id); err != nil {
			sendErr(w, 500, "Gagal memperbarui sesi: "+err.Error())
			return
		}
	}
	sendJSON(w, 200, map[string]any{
		"ok": true, "changed": changed,
		"pesan": "Akun " + curUser + " diperbarui (" + strings.Join(changed, ", ") + ").",
	})
}

func (a *App) accountsLogoutAllAPI(w http.ResponseWriter, r *http.Request) {
	me := a.requireSuper(w, r)
	if me == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("id")))
	if err != nil || id <= 0 {
		sendErr(w, 400, "ID akun tidak valid.")
		return
	}
	var username string
	if err := a.dbHandle().QueryRow(
		`SELECT username FROM accounts WHERE id=$1`, id).Scan(&username); err != nil {
		sendErr(w, 404, "Akun tidak ditemukan.")
		return
	}
	res, err := a.dbHandle().Exec(`DELETE FROM auth_sessions WHERE account_id=$1`, id)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if id == me.UserID {

		if err := a.createSession(w, id); err != nil {
			sendErr(w, 500, "Gagal memperbarui sesi: "+err.Error())
			return
		}
	}
	pesan := fmt.Sprintf("Akun %s tidak memiliki sesi aktif.", username)
	if n > 0 {
		pesan = fmt.Sprintf("Akun %s dikeluarkan dari %d perangkat/browser.", username, n)
	}
	sendJSON(w, 200, map[string]any{"ok": true, "keluar": n, "pesan": pesan})
}

func (a *App) accountsDeleteAPI(w http.ResponseWriter, r *http.Request) {
	me := a.requireSuper(w, r)
	if me == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("id")))
	if err != nil || id <= 0 {
		sendErr(w, 400, "ID akun tidak valid.")
		return
	}
	if id == me.UserID {
		sendErr(w, 400, "Tidak bisa menghapus akun sendiri.")
		return
	}
	var username, role string
	if err := a.dbHandle().QueryRow(
		`SELECT username, role FROM accounts WHERE id=$1`, id).
		Scan(&username, &role); err != nil {
		sendErr(w, 404, "Akun tidak ditemukan.")
		return
	}
	if role == roleSuper {
		n, err := a.superCount()
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if n <= 1 {
			sendErr(w, 400, "Harus ada minimal satu Super Admin.")
			return
		}
	}

	if _, err := a.dbHandle().Exec(`DELETE FROM accounts WHERE id=$1`, id); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{
		"ok":    true,
		"pesan": "Akun " + username + " dihapus.",
	})
}
