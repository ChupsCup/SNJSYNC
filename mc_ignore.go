package main

import (
	"net/http"
	"strings"
)

type mcIgnore struct {
	ID   int    `json:"id"`
	Word string `json:"word"`
}

type ignoreWords struct {
	set *strSet
}

func normIgnoreWord(s string) string {
	toks := strings.Fields(normAliasText(s))
	if len(toks) != 1 {
		return ""
	}
	if len(toks[0]) < 2 || len(toks[0]) > 30 {
		return ""
	}
	return toks[0]
}

func newIgnoreWords(list []mcIgnore) *ignoreWords {
	iw := &ignoreWords{set: newSet()}
	for _, it := range list {
		w := normIgnoreWord(it.Word)
		if w == "" {
			continue
		}
		added := false
		for _, t := range tokenize2(w) {
			iw.set.Add(t)
			added = true
		}
		if !added {
			iw.set.Add(strings.ToLower(w))
		}
	}
	return iw
}

func (iw *ignoreWords) Has(t string) bool {
	if iw == nil || iw.set == nil {
		return false
	}
	return iw.set.Has(t)
}

func (iw *ignoreWords) filterSet(s *strSet) *strSet {
	if iw == nil || iw.set == nil || iw.set.Len() == 0 || s == nil {
		return s
	}
	drop := []string{}
	for _, t := range s.List() {
		if iw.Has(t) {
			drop = append(drop, t)
		}
	}
	for _, t := range drop {
		s.Delete(t)
	}
	return s
}

func (a *App) loadMcIgnores() []mcIgnore {
	db := a.dbHandle()
	if db == nil {
		return nil
	}
	rows, err := db.Query(`SELECT id, word FROM mc_ignore_words ORDER BY id`)
	if err != nil {
		logf("MC ignore load: %v", err)
		return nil
	}
	defer rows.Close()
	out := []mcIgnore{}
	for rows.Next() {
		var it mcIgnore
		if err := rows.Scan(&it.ID, &it.Word); err == nil {
			out = append(out, it)
		}
	}
	return out
}

func (a *App) mcIgnoreList(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	rows, err := db.Query(`SELECT id, word FROM mc_ignore_words ORDER BY id`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	list := []mcIgnore{}
	for rows.Next() {
		var it mcIgnore
		if err := rows.Scan(&it.ID, &it.Word); err == nil {
			list = append(list, it)
		}
	}
	sendJSON(w, 200, map[string]any{"words": list})
}

func (a *App) mcIgnoreAdd(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	var in struct {
		Word string `json:"word"`
	}
	if err := readJSONBody(r, &in); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	raw := strings.TrimSpace(in.Word)
	if raw == "" {
		sendErr(w, 400, "Isi satu kata")
		return
	}
	if len(strings.Fields(raw)) != 1 {
		sendErr(w, 400, "Satu kata saja, tanpa spasi")
		return
	}
	w2 := normIgnoreWord(raw)
	if w2 == "" {
		sendErr(w, 400, "Kata harus 2-30 huruf/angka")
		return
	}
	if !aliasHasLetter(w2) {
		sendErr(w, 400, "Kata harus ada hurufnya")
		return
	}
	var dup int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mc_ignore_words WHERE word=$1`, w2).Scan(&dup); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	if dup > 0 {
		sendErr(w, 409, "Kata sudah ada: "+w2)
		return
	}
	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM mc_ignore_words`).Scan(&total)
	if total >= 500 {
		sendErr(w, 400, "Maksimal 500 kata")
		return
	}
	var id int
	if err := db.QueryRow(`INSERT INTO mc_ignore_words (word) VALUES ($1) RETURNING id`, w2).Scan(&id); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	logf("[MC] ignore +%d: %s", id, w2)
	sendJSON(w, 200, map[string]any{"ok": true, "id": id, "word": w2})
}

func (a *App) mcIgnoreDelete(w http.ResponseWriter, r *http.Request) {
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
	if _, err := db.Exec(`DELETE FROM mc_ignore_words WHERE id=$1`, in.ID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	logf("[MC] ignore -%d", in.ID)
	sendJSON(w, 200, map[string]any{"ok": true})
}
