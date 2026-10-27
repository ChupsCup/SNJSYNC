package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	minPassLen = 8
	maxPassLen = 72
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

type accountItem struct {
	ID           int      `json:"id"`
	Username     string   `json:"username"`
	Role         string   `json:"role"`
	Modules      []string `json:"modules"`
	ModulesLabel string   `json:"modulesLabel"`
	CreatedAt    string   `json:"createdAt"`
	LastLogin    string   `json:"lastLogin"`
	LastSeen     string   `json:"lastSeen"`
	SesiAktif    int      `json:"sesiAktif"`
	LoginTotal   int      `json:"loginTotal"`
	Login30      int      `json:"login30"`
	Status       string   `json:"status"`
	IdleDays     int      `json:"idleDays"`
	IsYou        bool     `json:"isYou"`
}



func activityStatus(lastSeen, lastLogin sql.NullTime, now time.Time) (string, int) {
	best := lastSeen
	if !best.Valid {
		best = lastLogin
	}
	if !best.Valid {
		return "belum", -1
	}
	days := int(now.Sub(best.Time).Hours() / 24)
	if days < 0 {
		days = 0
	}
	switch {
	case days <= activeMaxDays:
		return "aktif", days
	case days <= idleMaxDays:
		return "jarang", days
	default:
		return "mati", days
	}
}

func roleLabel(role string) string {
	switch role {
	case roleSuper:
		return "Super Admin"
	case roleUser:
		return "User"
	default:
		return "Admin"
	}
}

func moduleLabel(mod string) string {
	switch mod {
	case modCTF:
		return "Cari Foto"
	case modMC:
		return "Master Item"
	case modMMD:
		return "Masterlist"
	default:
		return mod
	}
}

func modulesLabel(mods []string) string {
	if len(mods) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(mods))
	for _, m := range mods {
		parts = append(parts, moduleLabel(m))
	}
	return strings.Join(parts, ", ")
}


func validRole(role string) bool {
	return role == roleAdmin || role == roleSuper || role == roleUser
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
		SELECT acc.id, acc.username, acc.role, COALESCE(acc.modules,''),
		       COALESCE(to_char(acc.created_at,'DD/MM/YYYY HH24:MI'),''),
		       COALESCE(to_char(acc.last_login,'DD/MM/YYYY HH24:MI'),''),
		       COALESCE(to_char(acc.last_seen,'DD/MM/YYYY HH24:MI'),''),
		       acc.last_login, acc.last_seen,
		       (SELECT COUNT(*) FROM auth_sessions sx
		         WHERE sx.account_id = acc.id AND sx.expires_at > NOW()),
		       (SELECT COUNT(*) FROM login_events le WHERE le.account_id = acc.id),
		       (SELECT COUNT(*) FROM login_events le
		         WHERE le.account_id = acc.id AND le.at > NOW() - INTERVAL '30 days')
		  FROM accounts acc
		 ORDER BY CASE acc.role WHEN 'superadmin' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
		          lower(acc.username)`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	now := time.Now()
	list := []accountItem{}
	for rows.Next() {
		var it accountItem
		var mods string
		var ll, ls sql.NullTime
		if err := rows.Scan(&it.ID, &it.Username, &it.Role, &mods,
			&it.CreatedAt, &it.LastLogin, &it.LastSeen, &ll, &ls,
			&it.SesiAktif, &it.LoginTotal, &it.Login30); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		it.Modules = parseModulesCSV(mods)
		it.ModulesLabel = modulesLabel(it.Modules)
		it.Status, it.IdleDays = activityStatus(ls, ll, now)
		it.IsYou = it.ID == s.UserID
		list = append(list, it)
	}
	sendJSON(w, 200, map[string]any{"accounts": list, "isYou": s.UserID})
}


func (a *App) accountsActivityAPI(w http.ResponseWriter, r *http.Request) {
	if a.requireSuper(w, r) == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("id")))
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

	rows, err := a.dbHandle().Query(`
		SELECT to_char(d.day,'YYYY-MM-DD'), COALESCE(c.n,0)
		  FROM generate_series(CURRENT_DATE - INTERVAL '29 days', CURRENT_DATE, '1 day') AS d(day)
		  LEFT JOIN (
		    SELECT date_trunc('day', at)::date AS day, COUNT(*) AS n
		      FROM login_events
		     WHERE account_id = $1 AND at > NOW() - INTERVAL '30 days'
		     GROUP BY 1
		  ) c ON c.day = d.day
		 ORDER BY d.day`, id)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type dayCount struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	days := []dayCount{}
	total30 := 0
	for rows.Next() {
		var d dayCount
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		total30 += d.Count
		days = append(days, d)
	}

	var total int
	a.dbHandle().QueryRow(`SELECT COUNT(*) FROM login_events WHERE account_id=$1`, id).Scan(&total)

	sendJSON(w, 200, map[string]any{
		"id":       id,
		"username": username,
		"days":     days,
		"total30":  total30,
		"total":    total,
	})
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
	if !validRole(role) {
		sendErr(w, 400, "Role tidak dikenal.")
		return
	}
	mods := parseModulesCSV(r.FormValue("modules"))
	if role != roleUser {
		mods = nil
	}
	if role == roleUser && len(mods) == 0 {
		sendErr(w, 400, "Pilih minimal satu modul untuk akun User.")
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
		`INSERT INTO accounts (username, password_hash, role, modules) VALUES ($1,$2,$3,$4) RETURNING id`,
		username, hash, role, modulesCSV(mods)).Scan(&id); err != nil {

		sendErr(w, 409, "Gagal membuat akun: "+err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{
		"ok": true, "id": id, "username": username, "role": role,
		"modules": mods,
		"pesan":   "Akun " + roleLabel(role) + " " + username + " dibuat.",
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

	var curUser, curRole, curModsRaw string
	if err := a.dbHandle().QueryRow(
		`SELECT username, role, COALESCE(modules,'') FROM accounts WHERE id=$1`, id).
		Scan(&curUser, &curRole, &curModsRaw); err != nil {
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


	effRole := curRole
	if newRole != "" {
		effRole = newRole
	}

	if newRole != "" && newRole != curRole {
		if !validRole(newRole) {
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


	if effRole == roleUser {
		mods := parseModulesCSV(r.FormValue("modules"))
		if len(mods) == 0 {
			sendErr(w, 400, "Pilih minimal satu modul untuk akun User.")
			return
		}
		if modulesCSV(mods) != curModsRaw {
			if _, err := a.dbHandle().Exec(
				`UPDATE accounts SET modules=$1 WHERE id=$2`, modulesCSV(mods), id); err != nil {
				sendErr(w, 500, err.Error())
				return
			}
			changed = append(changed, "modul")
		}
	} else if curModsRaw != "" {
		if _, err := a.dbHandle().Exec(
			`UPDATE accounts SET modules='' WHERE id=$1`, id); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		changed = append(changed, "modul")
	}

	if len(changed) == 0 {
		sendErr(w, 400, "Tidak ada perubahan.")
		return
	}

	a.dbHandle().Exec(`DELETE FROM auth_sessions WHERE account_id=$1`, id)
	if id == me.UserID {
		if err := a.createSession(w, r, id); err != nil {
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

		if err := a.createSession(w, r, id); err != nil {
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
