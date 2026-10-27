package main

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type mcAlias struct {
	ID int    `json:"id"`
	A  string `json:"a"`
	B  string `json:"b"`
}

var reAliasTok = regexp.MustCompile(`[A-Z0-9]+`)

func normAliasText(s string) string {
	return strings.Join(reAliasTok.FindAllString(strings.ToUpper(s), -1), " ")
}

func aliasHasLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			return true
		}
	}
	return false
}

type aliasRule struct {
	re   *regexp.Regexp
	to   string
	toks int
}

type aliasRules struct {
	rules []aliasRule
}

func newAliasRules(list []mcAlias) *aliasRules {
	ar := &aliasRules{}
	for _, al := range list {
		aToks := strings.Fields(normAliasText(al.A))
		bToks := strings.Fields(normAliasText(al.B))
		if len(aToks) == 0 || len(bToks) == 0 {
			continue
		}
		quote := make([]string, 0, len(bToks))
		for _, t := range bToks {
			quote = append(quote, regexp.QuoteMeta(t))
		}
		re, err := regexp.Compile(`(?i)\b` + strings.Join(quote, `[\s\-_.,/]*`) + `\b`)
		if err != nil {
			continue
		}
		ar.rules = append(ar.rules, aliasRule{re: re, to: strings.Join(aToks, " "), toks: len(bToks)})
	}
	sort.SliceStable(ar.rules, func(i, j int) bool { return ar.rules[i].toks > ar.rules[j].toks })
	return ar
}

func (ar *aliasRules) apply(s string) string {
	if ar == nil || len(ar.rules) == 0 || s == "" {
		return s
	}
	for _, r := range ar.rules {
		s = r.re.ReplaceAllLiteralString(s, r.to)
	}
	return s
}

func (a *App) loadMcAliases() []mcAlias {
	db := a.dbHandle()
	if db == nil {
		return nil
	}
	rows, err := db.Query(`SELECT id, left_text, right_text FROM mc_aliases ORDER BY id`)
	if err != nil {
		logf("MC alias load: %v", err)
		return nil
	}
	defer rows.Close()
	out := []mcAlias{}
	for rows.Next() {
		var al mcAlias
		if err := rows.Scan(&al.ID, &al.A, &al.B); err == nil {
			out = append(out, al)
		}
	}
	return out
}

func (a *App) mcAliasList(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	rows, err := db.Query(`SELECT id, left_text, right_text FROM mc_aliases ORDER BY id`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	list := []mcAlias{}
	for rows.Next() {
		var al mcAlias
		if err := rows.Scan(&al.ID, &al.A, &al.B); err == nil {
			list = append(list, al)
		}
	}
	sendJSON(w, 200, map[string]any{"aliases": list})
}

func (a *App) mcAliasAdd(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	var in struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	if err := readJSONBody(r, &in); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	na, nb := normAliasText(in.A), normAliasText(in.B)
	if na == "" || nb == "" {
		sendErr(w, 400, "Isi dua kata: kiri = kanan")
		return
	}
	if na == nb {
		sendErr(w, 400, "Kata kiri dan kanan sama")
		return
	}
	if len(na) > 120 || len(nb) > 120 {
		sendErr(w, 400, "Maksimal 120 karakter per sisi")
		return
	}
	if len(strings.Fields(na)) > 8 || len(strings.Fields(nb)) > 8 {
		sendErr(w, 400, "Maksimal 8 kata per sisi")
		return
	}
	if !aliasHasLetter(na) || !aliasHasLetter(nb) {
		sendErr(w, 400, "Tiap sisi harus ada hurufnya")
		return
	}
	var dup int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_aliases WHERE (left_text=$1 AND right_text=$2) OR (left_text=$2 AND right_text=$1)`, na, nb).Scan(&dup); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	if dup > 0 {
		sendErr(w, 409, "Alias sudah ada: "+na+" = "+nb)
		return
	}
	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM mc_aliases`).Scan(&total)
	if total >= 500 {
		sendErr(w, 400, "Maksimal 500 alias")
		return
	}
	var id int
	if err := db.QueryRow(`INSERT INTO mc_aliases (left_text, right_text) VALUES ($1,$2) RETURNING id`, na, nb).Scan(&id); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	logf("[MC] alias +%d: %s = %s", id, na, nb)
	sendJSON(w, 200, map[string]any{"ok": true, "id": id, "a": na, "b": nb})
}

func (a *App) mcAliasDelete(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	var in struct {
		ID int `json:"id"`
	}
	if err := readJSONBody(r, &in); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	if in.ID <= 0 {
		sendErr(w, 400, "ID tidak valid")
		return
	}
	if _, err := db.Exec(`DELETE FROM mc_aliases WHERE id=$1`, in.ID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	logf("[MC] alias -%d", in.ID)
	sendJSON(w, 200, map[string]any{"ok": true})
}

