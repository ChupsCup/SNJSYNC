package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const userPageSize = 60

func atoiSafe(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
		return n
	}
	return def
}

type mmdSaveResult struct {
	OK         bool   `json:"ok"`
	UploadID   int    `json:"uploadId"`
	TotalFile  int    `json:"totalFile"`
	Baru       int    `json:"baru"`
	Duplikat   int    `json:"duplikat"`
	Terisi     int    `json:"terisi"`
	Potongan   int    `json:"potongan"`
	Diperbarui int    `json:"diperbarui"`
	Revisi     int    `json:"revisi"`
	Total      int    `json:"total"`
	Pesan      string `json:"pesan"`
}

func (a *App) mmdSaveAPI(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	parts, _, ok := multipartParts(r)
	if !ok {
		sendErr(w, 400, "Upload gagal: multipart tidak valid.")
		return
	}

	var fp *mpPart
	var fpMentah *mpPart
	for i := range parts {
		if parts[i].filename == "" {
			continue
		}
		switch parts[i].name {
		case "mentah":
			fpMentah = &parts[i]
		default:
			if fp == nil {
				fp = &parts[i]
			}
		}
	}
	if fp == nil {
		sendErr(w, 400, "File bersih belum diunggah. Jalankan \"Proses Cleaning\" dulu, lalu \"Simpan MMD\".")
		return
	}

	rows, err := parseMMDBersih(fp.data)
	if err != nil {
		sendErr(w, 400, "Gagal baca file bersih: "+err.Error())
		return
	}

	peta := map[string]mmdPotongan{}
	if fpMentah != nil {
		if p, err := parseMMDMentah(fpMentah.data); err == nil {
			peta = p
		}
	}

	ctx, cancel := ctxTimeout(5 * time.Minute)
	defer cancel()

	var uploadID int
	if err := db.QueryRowContext(ctx,
		`INSERT INTO mmd_uploads (filename, total_rows) VALUES ($1,$2) RETURNING id`,
		fp.filename, len(rows)).Scan(&uploadID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}

	lamaDesc := map[string]string{}
	if rs, err := db.QueryContext(ctx,
		`SELECT dedup_key, deskripsi FROM mmd_items WHERE dedup_key IS NOT NULL`); err == nil {
		for rs.Next() {
			var k, d string
			if rs.Scan(&k, &d) == nil {
				lamaDesc[k] = d
			}
		}
		rs.Close()
	}
	var daftarRevisi []revisiBaris

	baru, duplikat, diperbarui, terisiPotongan := 0, 0, 0, 0
	for _, b := range rows {
		p := peta[b.ItemCode]
		if p.Utama == "" && p.Spesifik == "" {

			utama, spes := mmdPecahDeskripsi(b.Deskripsi)
			p = mmdPotongan{Utama: utama, Spesifik: b.Deskripsi}
			_ = spes
		} else if p.Spesifik == "" {
			p.Spesifik = b.Deskripsi
		}
		if p.Utama != "" {
			terisiPotongan++
		}

		aktif := mmdAktif(b.NonAktif)

		key := cutKey(b.ItemCode)
		if key == "" {
			continue
		}

		oldDesc, sudahAda := lamaDesc[key]

		var id int
		var diinsert bool
		err := db.QueryRowContext(ctx,
			`INSERT INTO mmd_items
			   (item_code, deskripsi, kategori, status_aktif,
			    potongan_utama, potongan_spesifik, dedup_key,
			    upload_id, nama_upload, bersih)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,TRUE)
			 ON CONFLICT (dedup_key) DO UPDATE SET
			   deskripsi = EXCLUDED.deskripsi,
			   kategori = EXCLUDED.kategori,
			   status_aktif = EXCLUDED.status_aktif,
			   potongan_utama = EXCLUDED.potongan_utama,
			   potongan_spesifik = EXCLUDED.potongan_spesifik,
			   upload_id = EXCLUDED.upload_id,
			   nama_upload = EXCLUDED.nama_upload,
			   -- tanggal_masuk TIDAK diubah: itu tanggal asli baris
			   -- masuk (hari-1), bukan waktu baris terakhir disentuh.
			   diubah_pada = CASE
			     WHEN mmd_items.deskripsi IS DISTINCT FROM EXCLUDED.deskripsi
			     THEN now() ELSE mmd_items.diubah_pada END,
			   bersih = TRUE
			 RETURNING id, (xmax = 0) AS diinsert`,
			b.ItemCode, b.Deskripsi, b.Kategori, aktif,
			p.Utama, p.Spesifik, key, uploadID, fp.filename,
		).Scan(&id, &diinsert)
		if err != nil {
			duplikat++
			continue
		}
		if diinsert {
			baru++
		} else {
			diperbarui++
		}

		if sudahAda && bedaDeskripsi(oldDesc, b.Deskripsi) {
			daftarRevisi = append(daftarRevisi, revisiBaris{
				key: key, code: b.ItemCode, lama: oldDesc, baru: b.Deskripsi,
			})
		}
	}

	if err := simpanRevisiMMD(ctx, db, daftarRevisi, fp.filename); err != nil {
		logf("MMD riwayat revisi: %v", err)
	}

	var totalDB int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mmd_items WHERE bersih`).Scan(&totalDB)

	db.QueryRowContext(ctx,
		`UPDATE mmd_uploads SET baru=$2, diperbarui=$3 WHERE id=$1`,
		uploadID, baru, diperbarui)

	pesan := fmt.Sprintf("%d item baru disimpan, %d data diperbarui, %d revisi deskripsi, %d gagal. Total di database: %d.",
		baru, diperbarui, len(daftarRevisi), duplikat, totalDB)
	if fpMentah == nil {
		pesan += " (file mentah tidak dikirim, potongan diambil dari deskripsi)"
	}
	sendJSON(w, 200, mmdSaveResult{
		OK: true, UploadID: uploadID, TotalFile: len(rows),
		Baru: baru, Duplikat: duplikat, Diperbarui: diperbarui,
		Revisi: len(daftarRevisi),
		Terisi: len(rows), Potongan: terisiPotongan,
		Total: totalDB,
		Pesan: pesan,
	})
}

func (a *App) userMasterlists(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	rows, err := db.Query(`SELECT id, filename, uploaded_at, total_rows
	                          FROM masterlist_uploads
	                         WHERE status='done'
	                         ORDER BY uploaded_at DESC LIMIT 400`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type mlDate struct {
		ID         int    `json:"id"`
		Filename   string `json:"filename"`
		UploadedAt string `json:"uploadedAt"`
		TotalRows  int    `json:"totalRows"`
	}
	out := []mlDate{}
	for rows.Next() {
		var m mlDate
		var at time.Time
		if rows.Scan(&m.ID, &m.Filename, &at, &m.TotalRows) != nil {
			continue
		}
		m.UploadedAt = at.Format("2006-01-02")
		out = append(out, m)
	}
	sendJSON(w, 200, map[string]any{"masterlists": out})
}

type plCompareRow struct {
	NoContainer string `json:"noContainer"`
	Brand       string `json:"brand"`
	NamaItem    string `json:"namaItem"`
	ETA         string `json:"eta"`
	Importir    string `json:"importir"`
	Sheet       string `json:"sheet"`
	Flag        string `json:"flag"`
	MasterCode  string `json:"masterCode"`
	MasterDesc  string `json:"masterDesc"`

	Note string `json:"note"`
}

func latestCompareResultID(db *sql.DB, id int) int {
	if id > 0 {
		return id
	}
	var out int
	_ = db.QueryRow(
		`SELECT id FROM compare_results
		  WHERE results_json IS NOT NULL AND results_json <> '[]' AND total > 0
		  ORDER BY id DESC LIMIT 1`).Scan(&out)
	return out
}

func scanCompareRows(db *sql.DB, id int) ([]plCompareRow, error) {
	var raw string
	if err := db.QueryRow(`SELECT results_json FROM compare_results WHERE id=$1`, id).Scan(&raw); err != nil {
		return nil, err
	}
	var list []plCompareRow
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (a *App) userCompareList(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	rows, err := db.Query(
		`SELECT c.id, c.filename, c.created_at, c.total, c.exist_count, c.new_count,
		        c.review_count, c.approved, u.filename, u.uploaded_at
		   FROM compare_results c
		   JOIN packing_uploads u ON u.id = c.packing_upload_id
		  WHERE c.results_json IS NOT NULL AND c.results_json <> '[]' AND c.total > 0
		  ORDER BY c.id DESC LIMIT 100`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type cmp struct {
		ID         int    `json:"id"`
		Filename   string `json:"filename"`
		CreatedAt  string `json:"createdAt"`
		Total      int    `json:"total"`
		Existing   int    `json:"existing"`
		NewItem    int    `json:"newItem"`
		Review     int    `json:"review"`
		Approved   bool   `json:"approved"`
		PLFilename string `json:"plFilename"`
		PLUploaded string `json:"plUploaded"`
	}
	out := []cmp{}
	for rows.Next() {
		var c cmp
		var ca, ua time.Time
		if rows.Scan(&c.ID, &c.Filename, &ca, &c.Total, &c.Existing, &c.NewItem,
			&c.Review, &c.Approved, &c.PLFilename, &ua) != nil {
			continue
		}
		c.CreatedAt = ca.Format("2006-01-02")
		c.PLUploaded = ua.Format("2006-01-02")
		out = append(out, c)
	}
	sendJSON(w, 200, map[string]any{"compares": out})
}

func (a *App) userMasterItems(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	imp := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("importir")))
	sheet := strings.TrimSpace(r.URL.Query().Get("sheet"))
	page := atoiSafe(r.URL.Query().Get("page"), 1)
	limit := atoiSafe(r.URL.Query().Get("limit"), userPageSize)

	cid := latestCompareResultID(db, atoiSafe(r.URL.Query().Get("compareId"), 0))
	if cid == 0 {
		sendJSON(w, 200, map[string]any{"items": []plCompareRow{}, "total": 0,
			"page": page, "limit": limit, "compareId": 0,
			"existing": 0, "newItem": 0})
		return
	}
	list, err := scanCompareRows(db, cid)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}

	var sumExisting, sumNew int
	for _, it := range list {
		if it.Flag == "NEW ITEM" {
			sumNew++
		} else {
			sumExisting++
		}
	}

	lq := strings.ToLower(q)
	filtered := make([]plCompareRow, 0, len(list))
	for _, it := range list {

		if strings.HasSuffix(it.NoContainer, "TOTAL") && it.NamaItem == "" {
			continue
		}
		if status == "EXISTING" && it.Flag != "EXISTING" {
			continue
		}
		if status == "NEW" && it.Flag != "NEW ITEM" {
			continue
		}
		if imp != "" && it.Importir != imp {
			continue
		}
		if sheet != "" && it.Sheet != sheet {
			continue
		}
		if lq != "" && !plRowMatch(it, lq) {
			continue
		}
		filtered = append(filtered, it)
	}

	total := len(filtered)
	off := (page - 1) * limit
	if off > total {
		off = total
	}
	end := off + limit
	if end > total {
		end = total
	}
	page2 := filtered[off:end]

	sheets := newSet()
	imps := newSet()
	for _, it := range list {
		if it.Sheet != "" {
			sheets.Add(it.Sheet)
		}
		if it.Importir != "" {
			imps.Add(it.Importir)
		}
	}

	sendJSON(w, 200, map[string]any{
		"items": page2, "total": total, "page": page, "limit": limit,
		"compareId": cid, "existing": sumExisting, "newItem": sumNew,
		"sheets": sheets.List(), "importirs": imps.List(),
	})
}

func plRowMatch(it plCompareRow, lq string) bool {
	for _, f := range []string{it.NoContainer, it.Brand, it.NamaItem, it.Importir,
		it.Sheet, it.Flag, it.MasterCode, it.MasterDesc, it.Note} {
		if f != "" && strings.Contains(strings.ToLower(f), lq) {
			return true
		}
	}
	return false
}

func derefS(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (a *App) userMMDItems(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kategori := strings.TrimSpace(r.URL.Query().Get("kategori"))
	page := atoiSafe(r.URL.Query().Get("page"), 1)
	limit := atoiSafe(r.URL.Query().Get("limit"), userPageSize)
	onlyAktif := r.URL.Query().Get("aktif") == "1"

	where := []string{"bersih = TRUE"}
	args := []any{}
	if q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		i := len(args)
		where = append(where, fmt.Sprintf(
			`(lower(deskripsi) LIKE $%d OR lower(item_code) LIKE $%d
			  OR lower(potongan_utama) LIKE $%d OR lower(potongan_spesifik) LIKE $%d
			  OR lower(kategori) LIKE $%d)`,
			i, i, i, i, i))
	}
	if kategori != "" {
		args = append(args, strings.ToUpper(kategori))
		where = append(where, fmt.Sprintf("kategori = $%d", len(args)))
	}
	if onlyAktif {
		where = append(where, "status_aktif = 1")
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mmd_items WHERE `+cond, args...).Scan(&total); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	args2 := append(append([]any{}, args...), limit, (page-1)*limit)
	rows, err := db.Query(
		`SELECT item_code, deskripsi, kategori, status_aktif,
		        potongan_utama, potongan_spesifik,
		        tanggal_masuk, nama_upload,
		        (SELECT COUNT(*) FROM mmd_revisi rv
		          WHERE rv.dedup_key = mmd_items.dedup_key)
		   FROM mmd_items WHERE `+cond+`
		  ORDER BY id LIMIT $`+itoa(len(args)+1)+` OFFSET $`+itoa(len(args)+2), args2...)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type mi struct {
		ItemCode   string `json:"itemCode"`
		Deskripsi  string `json:"deskripsi"`
		Kategori   string `json:"kategori"`
		Aktif      int    `json:"aktif"`
		Utama      string `json:"potonganUtama"`
		Spesifik   string `json:"potonganSpesifik"`
		Tanggal    string `json:"tanggal"`
		NamaUpload string `json:"namaUpload"`
		Revisi     int    `json:"revisi"`
	}
	out := []mi{}
	for rows.Next() {
		var m mi
		var tm time.Time
		if rows.Scan(&m.ItemCode, &m.Deskripsi, &m.Kategori, &m.Aktif,
			&m.Utama, &m.Spesifik, &tm, &m.NamaUpload, &m.Revisi) != nil {
			continue
		}
		m.Tanggal = tm.Format("2006-01-02")

		m.Spesifik = bersihkanPotonganSpesifik(m.Spesifik)
		out = append(out, m)
	}
	sendJSON(w, 200, map[string]any{"items": out, "total": total, "page": page, "limit": limit})
}

func (a *App) userMMDMeta(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	kategori := []string{}
	if rows, err := db.Query(
		`SELECT DISTINCT kategori FROM mmd_items
		  WHERE bersih AND kategori <> '' ORDER BY kategori`); err == nil {
		for rows.Next() {
			var k string
			if rows.Scan(&k) == nil {
				kategori = append(kategori, k)
			}
		}
		rows.Close()
	}
	tanggal := []string{}
	if rows, err := db.Query(
		`SELECT DISTINCT to_char(tanggal_masuk,'YYYY-MM-DD') FROM mmd_items
		  WHERE bersih ORDER BY 1 DESC`); err == nil {
		for rows.Next() {
			var t string
			if rows.Scan(&t) == nil {
				tanggal = append(tanggal, t)
			}
		}
		rows.Close()
	}
	var total, aktif int
	db.QueryRow(`SELECT COUNT(*) FROM mmd_items WHERE bersih`).Scan(&total)
	db.QueryRow(`SELECT COUNT(*) FROM mmd_items WHERE bersih AND status_aktif=1`).Scan(&aktif)
	sendJSON(w, 200, map[string]any{
		"kategori": kategori, "tanggal": tanggal,
		"total": total, "aktif": aktif,
	})
}

func writeXLSX(w http.ResponseWriter, filename string, buf []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", itoa(len(buf)))
	w.Write(buf)
}

func (a *App) userExportML(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	cid := latestCompareResultID(db, atoiSafe(r.URL.Query().Get("compareId"), 0))
	if cid == 0 {
		sendErr(w, 404, "Belum ada hasil compare Masterlist Compare.")
		return
	}
	list, err := scanCompareRows(db, cid)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}

	f := excelize.NewFile()
	sheet := "Sheet1"
	hdr := []interface{}{"NO CONTAINER", "BRAND", "NAMA ITEM", "ETA", "IMPORTIR",
		"SHEET", "STATUS", "MASTER ITEM CODE", "MASTER ITEM DESC", "CATATAN"}
	_ = f.SetSheetRow(sheet, "A1", &hdr)
	i := 1
	for _, it := range list {
		if strings.HasSuffix(it.NoContainer, "TOTAL") && it.NamaItem == "" {
			continue
		}
		vals := []interface{}{it.NoContainer, it.Brand, it.NamaItem, it.ETA,
			it.Importir, it.Sheet, it.Flag, it.MasterCode, it.MasterDesc, it.Note}
		_ = f.SetSheetRow(sheet, "A"+itoa(i+1), &vals)
		i++
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	writeXLSX(w, "master-item-pl.xlsx", buf.Bytes())
}

func (a *App) userMMDRevisi(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	key := cutKey(strings.TrimSpace(r.URL.Query().Get("kode")))
	if key == "" {
		sendErr(w, 400, "Item code kosong.")
		return
	}
	rows, err := db.Query(`SELECT deskripsi_lama, deskripsi_baru,
		to_char(revised_at,'YYYY-MM-DD HH24:MI'), nama_upload
		FROM mmd_revisi WHERE dedup_key = $1
		ORDER BY revised_at DESC, id DESC`, key)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type rev struct {
		Lama    string `json:"lama"`
		Baru    string `json:"baru"`
		Tanggal string `json:"tanggal"`
		Upload  string `json:"upload"`
	}
	out := []rev{}
	for rows.Next() {
		var v rev
		if rows.Scan(&v.Lama, &v.Baru, &v.Tanggal, &v.Upload) != nil {
			continue
		}
		out = append(out, v)
	}
	sendJSON(w, 200, map[string]any{"items": out})
}

func (a *App) userExportMMD(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kategori := strings.TrimSpace(r.URL.Query().Get("kategori"))
	where := []string{"bersih = TRUE"}
	args := []any{}
	if q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		i := len(args)
		where = append(where, fmt.Sprintf(
			`(lower(deskripsi) LIKE $%d OR lower(item_code) LIKE $%d
			  OR lower(potongan_utama) LIKE $%d OR lower(potongan_spesifik) LIKE $%d)`,
			i, i, i, i))
	}
	if kategori != "" {
		args = append(args, strings.ToUpper(kategori))
		where = append(where, fmt.Sprintf("kategori = $%d", len(args)))
	}
	rows, err := db.Query(
		`SELECT item_code, deskripsi, kategori, status_aktif,
		        potongan_utama, potongan_spesifik
		   FROM mmd_items WHERE `+strings.Join(where, " AND ")+
			` ORDER BY id LIMIT 20000`, args...)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	f := excelize.NewFile()
	sheet := "Sheet1"
	hdr := []interface{}{"ITEM CODE", "DESKRIPSI", "KATEGORI", "STATUS",
		"POTONGAN UTAMA", "POTONGAN SPESIFIK"}
	_ = f.SetSheetRow(sheet, "A1", &hdr)
	i := 1
	for rows.Next() {
		var ic, ds, kat, ut, sp *string
		var aktif int
		if rows.Scan(&ic, &ds, &kat, &aktif, &ut, &sp) != nil {
			continue
		}
		status := "AKTIF"
		if aktif == 0 {
			status = "TIDAK AKTIF"
		}
		vals := []interface{}{derefS(ic), derefS(ds), derefS(kat),
			status, derefS(ut), bersihkanPotonganSpesifik(derefS(sp))}
		_ = f.SetSheetRow(sheet, "A"+itoa(i+1), &vals)
		i++
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	writeXLSX(w, "mmd-master-descriptor.xlsx", buf.Bytes())
}

func (a *App) userSummary(w http.ResponseWriter, r *http.Request) {
	a.waitReady()
	out := map[string]any{}
	out["foto"] = a.stats()
	if db := a.requireDB(w); db != nil {
		var ml, mlItem, mmd, mmdAktif, plItem int
		db.QueryRow(`SELECT COUNT(*) FROM masterlist_uploads WHERE status='done'`).Scan(&ml)
		db.QueryRow(`SELECT COUNT(*) FROM masterlist_items`).Scan(&mlItem)
		db.QueryRow(`SELECT COUNT(*) FROM mmd_items WHERE bersih`).Scan(&mmd)
		db.QueryRow(`SELECT COUNT(*) FROM mmd_items WHERE bersih AND status_aktif=1`).Scan(&mmdAktif)

		if cid := latestCompareResultID(db, 0); cid != 0 {
			if list, err := scanCompareRows(db, cid); err == nil {
				for _, it := range list {
					if strings.HasSuffix(it.NoContainer, "TOTAL") && it.NamaItem == "" {
						continue
					}
					plItem++
				}
			}
		}
		out["masterlistUpload"] = ml
		out["mlItem"] = mlItem

		out["masterItem"] = plItem

		out["masterlist"] = mmd
		out["masterlistAktif"] = mmdAktif
	}
	sendJSON(w, 200, out)
}

func ensureTemplate(root string) {
	p := filepath.Join(root, "template_output.xlsx")
	if _, err := os.Stat(p); err == nil {
		return
	}
}
