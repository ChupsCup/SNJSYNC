package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const dbUnavailableMsg = "Database PostgreSQL belum jalan. Jalankan Docker Desktop, tunggu container ctf-postgres aktif, lalu coba lagi."

type masterItemIn struct {
	NoContainer string `json:"no_container"`
	Brand       string `json:"brand"`
	NamaItem    string `json:"nama_item"`
	ItemCode    string `json:"item_code"`
	ItemDesc    string `json:"item_desc"`
	Gabungan    string `json:"gabungan"`
	Importir    string `json:"importir"`
}

const insertBatch = 2000

func (a *App) insertMasterItems(db *sql.DB, uploadID int, rows []parseRow) error {
	ctx, cancel := ctxTimeout(10 * time.Minute)
	defer cancel()
	for start := 0; start < len(rows); start += insertBatch {
		end := start + insertBatch
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		args := make([]any, 0, len(chunk)*8)
		place := make([]string, 0, len(chunk))
		for i, r := range chunk {
			b := i * 8
			args = append(args, uploadID, r.NoContainer, r.Brand, r.NamaItem, r.ItemCode, r.ItemDesc, r.Gabungan, r.Importir)
			place = append(place, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", b+1, b+2, b+3, b+4, b+5, b+6, b+7, b+8))
		}
		q := `INSERT INTO masterlist_items (upload_id, no_container, brand, nama_item, item_code, item_desc, gabungan, importir) VALUES ` +
			strings.Join(place, ", ")
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) insertPackingItems(db *sql.DB, uploadID int, rows []parseRow) error {
	ctx, cancel := ctxTimeout(10 * time.Minute)
	defer cancel()
	for start := 0; start < len(rows); start += insertBatch {
		end := start + insertBatch
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		args := make([]any, 0, len(chunk)*10)
		place := make([]string, 0, len(chunk))
		for i, r := range chunk {
			b := i * 10
			args = append(args, uploadID, r.Sheet, r.NoContainer, r.Brand, r.NamaItem, r.ETA,
				r.ItemCode, r.ItemDesc, r.Gabungan, r.Importir)
			place = append(place, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", b+1, b+2, b+3, b+4, b+5, b+6, b+7, b+8, b+9, b+10))
		}
		q := `INSERT INTO packing_items (upload_id, sheet, no_container, brand, nama_item, eta, item_code, item_desc, gabungan, importir) VALUES ` +
			strings.Join(place, ", ")
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) mcUploadML(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	parts, _, ok := multipartParts(r)
	if !ok {
		sendErr(w, 400, "Missing boundary")
		return
	}
	filePart := findFilePart(parts)
	if filePart == nil {
		sendErr(w, 400, "File tidak ditemukan")
		return
	}

	rows, err := parseExcelBuf(filePart.data)
	if err != nil {
		logf("[MC] masterlist %q ditolak: %v", filePart.filename, err)
		sendErr(w, 400, fmt.Sprintf("File %q tidak bisa diproses: %v", filePart.filename, err))
		return
	}
	ctx, cancel := ctxTimeout(60 * time.Second)
	var uploadID int
	err = db.QueryRowContext(ctx,
		`INSERT INTO masterlist_uploads (filename, sheet_name, status) VALUES ($1,'','processing') RETURNING id`,
		filePart.filename).Scan(&uploadID)
	cancel()
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "uploadIds": []int{uploadID},
		"total": len(rows), "sheets": uniqueSheetNames(rows), "processing": true})
	go a.processMLUpload(uploadID, rows)
}

func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logf("[%s] PANIC: %v", name, r)
			}
		}()
		fn()
	}()
}

func (a *App) processMLUpload(uploadID int, rows []parseRow) {
	db := a.dbHandle()
	if db == nil {
		logf("[MC] masterlist upload %d: PostgreSQL tidak tersedia, status tidak disimpan", uploadID)
		return
	}
	err := a.insertMasterItems(db, uploadID, rows)
	ctx, cancel := ctxTimeout(60 * time.Second)
	defer cancel()
	if err != nil {
		_, _ = db.ExecContext(ctx, `UPDATE masterlist_uploads SET status=$1 WHERE id=$2`, "error: "+err.Error(), uploadID)
		logf("[MC] masterlist upload %d gagal: %v", uploadID, err)
		return
	}
	sheets := uniqueSheetNames(rows)
	_, _ = db.ExecContext(ctx,
		`UPDATE masterlist_uploads SET sheet_name=$1, total_rows=$2, status='done' WHERE id=$3`,
		strings.Join(sheets, ", "), len(rows), uploadID)
	logf("[MC] masterlist upload %d selesai: %d baris", uploadID, len(rows))
}

func uniqueSheetNames(rows []parseRow) []string {
	set := newSet()
	for _, r := range rows {
		set.Add(r.Sheet)
	}
	return set.List()
}

func findFilePart(parts []mpPart) *mpPart {
	for i := range parts {
		if parts[i].filename != "" {
			return &parts[i]
		}
	}
	return nil
}

func (a *App) mcUploadPL(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	parts, _, ok := multipartParts(r)
	if !ok {
		sendErr(w, 400, "Missing boundary")
		return
	}
	filePart := findFilePart(parts)
	if filePart == nil {
		sendErr(w, 400, "File tidak ditemukan")
		return
	}
	rows, err := parseExcelBuf(filePart.data)
	if err != nil {
		logf("[MC] packing %q ditolak: %v", filePart.filename, err)
		sendErr(w, 400, fmt.Sprintf("File %q tidak bisa diproses: %v", filePart.filename, err))
		return
	}
	ctx, cancel := ctxTimeout(60 * time.Second)
	var uploadID int
	err = db.QueryRowContext(ctx,
		`INSERT INTO packing_uploads (filename, status) VALUES ($1,'processing') RETURNING id`,
		filePart.filename).Scan(&uploadID)
	cancel()
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "uploadId": uploadID,
		"total": len(rows), "sheets": uniqueSheetNames(rows), "processing": true})
	safeGo("MC-packing", func() { a.processPLUpload(uploadID, rows) })
}

func (a *App) processPLUpload(uploadID int, rows []parseRow) {
	db := a.dbHandle()
	if db == nil {
		logf("[MC] packing upload %d: PostgreSQL tidak tersedia, status tidak disimpan", uploadID)
		return
	}
	err := a.insertPackingItems(db, uploadID, rows)
	ctx, cancel := ctxTimeout(60 * time.Second)
	defer cancel()
	if err != nil {
		_, _ = db.ExecContext(ctx, `UPDATE packing_uploads SET status=$1 WHERE id=$2`, "error: "+err.Error(), uploadID)
		logf("[MC] packing upload %d gagal: %v", uploadID, err)
		return
	}
	_, _ = db.ExecContext(ctx,
		`UPDATE packing_uploads SET status='done', total_rows=$1 WHERE id=$2`, len(rows), uploadID)
	logf("[MC] packing upload %d selesai: %d baris", uploadID, len(rows))
}

func (a *App) mcUploadPedoman(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	parts, _, ok := multipartParts(r)
	if !ok {
		sendErr(w, 400, "Missing boundary")
		return
	}
	filePart := findFilePart(parts)
	if filePart == nil {
		sendErr(w, 400, "File tidak ditemukan")
		return
	}
	rows, err := parsePedomanBuf(filePart.data)
	if err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	if len(rows) == 0 {
		sendErr(w, 400, "Tidak ada baris valid (butuh kolom Kode Barang & Nama Barang).")
		return
	}
	ctx, cancel := ctxTimeout(10 * time.Minute)
	defer cancel()
	var uploadID int
	if err := db.QueryRowContext(ctx,
		`INSERT INTO pedoman_uploads (filename, total_rows) VALUES ($1,$2) RETURNING id`,
		filePart.filename, len(rows)).Scan(&uploadID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	for start := 0; start < len(rows); start += insertBatch {
		end := start + insertBatch
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		args := make([]any, 0, len(chunk)*5)
		place := make([]string, 0, len(chunk))
		for i, p := range chunk {
			b := i * 5
			args = append(args, uploadID, p.KodeBarang, p.NamaBarang, p.MerekBarang, p.NonAktif)
			place = append(place, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", b+1, b+2, b+3, b+4, b+5))
		}
		q := `INSERT INTO pedoman_items (upload_id, kode_barang, nama_barang, merek_barang, non_aktif) VALUES ` +
			strings.Join(place, ", ")
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
	}
	sendJSON(w, 200, map[string]any{"ok": true, "uploadId": uploadID, "total": len(rows)})
}

type uploadRow struct {
	ID         int     `json:"id"`
	Filename   string  `json:"filename"`
	SheetName  string  `json:"sheet_name,omitempty"`
	UploadedAt *string `json:"uploaded_at,omitempty"`
	Approved   int     `json:"approved,omitempty"`
	TotalRows  int     `json:"total_rows"`
	Status     string  `json:"status"`
}

func (a *App) mcStatus(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, map[string]any{"hasDb": a.db != nil})
}

func scanUploadRows(rows *sql.Rows, err error, withSheet bool) []uploadRow {
	if err != nil {
		logf("queryUploads error: %v", err)
		return nil
	}
	defer rows.Close()
	out := []uploadRow{}
	for rows.Next() {
		var u uploadRow
		var sheet, status sql.NullString
		var uploaded sql.NullString
		var approved sql.NullInt64
		dest := []any{&u.ID, &u.Filename}
		if withSheet {
			dest = append(dest, &sheet)
		}
		dest = append(dest, &uploaded, &approved, &u.TotalRows, &status)
		if err := rows.Scan(dest...); err != nil {
			continue
		}
		u.SheetName = nullStr(sheet)
		if uploaded.Valid {
			u.UploadedAt = &uploaded.String
		}
		u.Approved = int(approved.Int64)
		u.Status = nullStr(status)
		out = append(out, u)
	}
	return out
}

func (a *App) queryUploads(q string) []uploadRow {
	db := a.dbHandle()
	if db == nil {
		return nil
	}
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, q)
	return scanUploadRows(rows, err, strings.Contains(q, "sheet_name"))
}

func (a *App) mcResultsList(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx,
		`SELECT id, packing_upload_id, filename, created_at, total, exist_count, new_count, approved, review_count
		 FROM compare_results ORDER BY created_at DESC`)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type cr struct {
		ID            int     `json:"id"`
		PackingUpload int     `json:"packing_upload_id"`
		Filename      string  `json:"filename"`
		CreatedAt     *string `json:"created_at,omitempty"`
		Total         int     `json:"total"`
		ExistCount    int     `json:"exist_count"`
		NewCount      int     `json:"new_count"`
		Approved      int     `json:"approved"`
		ReviewCount   int     `json:"review_count"`
	}
	out := []cr{}
	for rows.Next() {
		var c cr
		var fn, created sql.NullString
		if err := rows.Scan(&c.ID, &c.PackingUpload, &fn, &created, &c.Total, &c.ExistCount, &c.NewCount,
			&c.Approved, &c.ReviewCount); err != nil {
			continue
		}
		c.Filename = nullStr(fn)
		if created.Valid {
			c.CreatedAt = &created.String
		}
		out = append(out, c)
	}
	sendJSON(w, 200, map[string]any{"results": out})
}

func (a *App) loadCompareResults(db *sql.DB, id int) (map[string]any, []compareRow, bool, error) {
	ctx, cancel := ctxTimeout(60 * time.Second)
	defer cancel()
	var js, created sql.NullString
	var approved int
	var total int
	err := db.QueryRowContext(ctx,
		`SELECT results_json, approved, total, created_at FROM compare_results WHERE id=$1`, id).
		Scan(&js, &approved, &total, &created)
	if err != nil {
		return nil, nil, false, err
	}
	var rows []compareRow
	if js.Valid && js.String != "" {
		if err := json.Unmarshal([]byte(js.String), &rows); err != nil {
			return nil, nil, false, err
		}
	}
	return map[string]any{"approved": approved, "total": total, "createdAt": nullStr(created)}, rows, true, nil
}

func (a *App) mcResultByID(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || id <= 0 {
		sendErr(w, 404, "Not found")
		return
	}
	db := a.dbHandle()
	if db == nil {
		sendErr(w, 404, "Not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		meta, rows, ok, err := a.loadCompareResults(db, id)
		if err != nil || !ok {
			sendErr(w, 404, "Not found")
			return
		}
		meta["id"] = id
		meta["results"] = rows
		sendJSON(w, 200, meta)
	case http.MethodDelete:
		ctx, cancel := ctxTimeout(30 * time.Second)
		defer cancel()
		res, err := db.ExecContext(ctx, `DELETE FROM compare_results WHERE id=$1`, id)
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			sendErr(w, 404, "Not found")
			return
		}
		logf("[MC] DELETE result id=%d", id)
		sendJSON(w, 200, map[string]any{"ok": true, "deleted": id})
	default:
		sendErr(w, 405, "Method not allowed")
	}
}

func (a *App) mcApproveML(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		UploadID int  `json:"uploadId"`
		Approved bool `json:"approved"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	av := 0
	if payload.Approved {
		av = 1
	}
	db := a.dbHandle()
	if db == nil {
		sendErr(w, 503, dbUnavailableMsg)
		return
	}
	if _, err := db.Exec(`UPDATE masterlist_uploads SET approved=$1 WHERE id=$2`, av, payload.UploadID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

type savedItem struct {
	NamaItem string `json:"namaItem"`
	Brand    string `json:"brand"`
	ItemCode string `json:"itemCode"`
	ItemDesc string `json:"itemDesc"`
	Gabungan string `json:"gabungan"`
}

func (a *App) saveMasterItems(db *sql.DB, filename, sheetName string, items []savedItem) (int, error) {
	ctx, cancel := ctxTimeout(10 * time.Minute)
	defer cancel()
	var uid int
	if err := db.QueryRowContext(ctx,
		`INSERT INTO masterlist_uploads (filename, sheet_name, total_rows, approved, status) VALUES ($1,$2,$3,1,'done') RETURNING id`,
		filename, sheetName, len(items)).Scan(&uid); err != nil {
		return 0, err
	}
	rows := make([]parseRow, 0, len(items))
	for _, it := range items {
		rows = append(rows, parseRow{Brand: it.Brand, NamaItem: it.NamaItem,
			ItemCode: it.ItemCode, ItemDesc: it.ItemDesc, Gabungan: it.Gabungan})
	}
	if err := a.insertMasterItems(db, uid, rows); err != nil {
		return uid, err
	}
	return uid, nil
}

func (a *App) mcSaveMasterItems(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Items []savedItem `json:"items"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	cleaned := []savedItem{}
	for _, it := range payload.Items {
		namaItem := strings.TrimSpace(it.NamaItem)
		brand := strings.TrimSpace(it.Brand)
		itemCode := strings.TrimSpace(it.ItemCode)
		itemDesc := strings.TrimSpace(it.ItemDesc)
		if itemDesc == "" {
			itemDesc = namaItem
		}
		if namaItem == "" && brand == "" {
			continue
		}
		label := namaItem
		if label == "" {
			label = itemDesc
		}
		if label == "" {
			continue
		}
		if namaItem == "" {
			namaItem = label
		}
		if itemDesc == "" {
			itemDesc = label
		}
		gab := strings.TrimSpace(strings.Join(strings.Fields(brand+" "+label), " "))
		cleaned = append(cleaned, savedItem{NamaItem: label, Brand: brand,
			ItemCode: itemCode, ItemDesc: itemDesc, Gabungan: gab})
	}
	if len(cleaned) == 0 {
		sendErr(w, 400, "Tidak ada item valid untuk disimpan.")
		return
	}
	sheet := "NEW ITEM " + time.Now().UTC().Format("2006-01-02")
	db := a.dbHandle()
	if db == nil {
		sendErr(w, 503, dbUnavailableMsg)
		return
	}
	uid, err := a.saveMasterItems(db, "Master Data Baru", sheet, cleaned)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "uploadId": uid,
		"saved": len(cleaned), "items": len(cleaned)})
}

func (a *App) mcApproveResult(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ResultID int `json:"resultId"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	if payload.ResultID == 0 {
		sendErr(w, 400, "resultId wajib diisi.")
		return
	}
	db := a.dbHandle()
	if db == nil {
		sendErr(w, 503, dbUnavailableMsg)
		return
	}
	meta, results, ok, err := a.loadCompareResults(db, payload.ResultID)
	if err != nil || !ok {
		sendErr(w, 404, "Hasil compare tidak ditemukan.")
		return
	}
	if meta["approved"].(int) != 0 {
		sendJSON(w, 200, map[string]any{"ok": true, "already": true, "uploadId": nil,
			"saved": 0, "approvedAt": meta["createdAt"]})
		return
	}
	toSave := []savedItem{}
	for _, r := range results {
		brand := strings.TrimSpace(r.Brand)
		namaItem := strings.TrimSpace(r.NamaItem)
		itemCode := strings.TrimSpace(r.MasterCode)
		itemDesc := strings.TrimSpace(r.MasterDesc)
		if namaItem == "" && brand == "" {
			continue
		}
		if itemCode == "" || itemCode == "-" {
			continue
		}
		if itemDesc == "" {
			itemDesc = namaItem
		}
		if namaItem == "" {
			namaItem = itemDesc
		}
		gab := strings.TrimSpace(strings.Join(strings.Fields(brand+" "+namaItem), " "))
		if brand == "" {
			brand = "-"
		}
		toSave = append(toSave, savedItem{Brand: brand, NamaItem: namaItem, ItemCode: itemCode,
			ItemDesc: itemDesc, Gabungan: gab})
	}
	var uploadID any
	if len(toSave) > 0 {
		sheet := "APPROVED " + time.Now().UTC().Format("2006-01-02") + " #" + itoa(payload.ResultID)
		uid, err := a.saveMasterItems(db, "Master Data #"+itoa(payload.ResultID), sheet, toSave)
		if err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		uploadID = uid
	}
	if _, err := db.Exec(`UPDATE compare_results SET approved=1 WHERE id=$1`, payload.ResultID); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "already": false, "uploadId": uploadID,
		"saved": len(toSave), "approvedAt": time.Now().UTC().Format(time.RFC3339)})
}

type reviewPayload struct {
	ResultID int    `json:"resultId"`
	No       int    `json:"no"`
	Key      string `json:"key"`
}

func (a *App) reviewTargets(results []compareRow, p reviewPayload) []int {
	if p.Key != "" {
		nk := normStr(p.Key)
		out := []int{}
		for i := range results {
			if results[i].NeedsReview && normStr(results[i].Brand+"|"+results[i].NamaItem) == nk {
				out = append(out, i)
			}
		}
		return out
	}
	for i := range results {
		if results[i].No == p.No {
			return []int{i}
		}
	}
	return nil
}

func (a *App) saveReviewResults(db *sql.DB, resultID int, results []compareRow) (exist, review int, err error) {
	exist, review = 0, 0
	for _, r := range results {
		if r.Flag == "EXISTING" {
			exist++
		}
		if r.NeedsReview {
			review++
		}
	}
	js, err := json.Marshal(results)
	if err != nil {
		return exist, review, err
	}
	_, err = db.Exec(
		`UPDATE compare_results SET results_json=$1, exist_count=$2, new_count=$3, review_count=$4 WHERE id=$5`,
		string(js), exist, len(results)-exist, review, resultID)
	return exist, review, err
}

func (a *App) mcConfirmReview(w http.ResponseWriter, r *http.Request) {
	a.reviewAction(w, r, true)
}

func (a *App) mcRejectReview(w http.ResponseWriter, r *http.Request) {
	a.reviewAction(w, r, false)
}

func (a *App) reviewAction(w http.ResponseWriter, r *http.Request, confirm bool) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	var p reviewPayload
	if err := readJSONBody(r, &p); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	if p.ResultID == 0 || (p.No == 0 && p.Key == "") {
		sendErr(w, 400, "resultId dan no/key wajib diisi.")
		return
	}
	meta, results, ok, err := a.loadCompareResults(db, p.ResultID)
	if err != nil || !ok {
		sendErr(w, 404, "Hasil compare tidak ditemukan.")
		return
	}
	if meta["approved"].(int) != 0 {
		sendErr(w, 400, "Hasil sudah di-approve, tidak bisa diubah.")
		return
	}
	idx := a.reviewTargets(results, p)
	if len(idx) == 0 {
		sendErr(w, 404, "Baris tidak ditemukan.")
		return
	}

	target := &results[idx[0]]
	if !target.NeedsReview {
		sendErr(w, 400, "Baris ini bukan REVIEW.")
		return
	}
	if confirm && target.CandCode == "" {
		sendErr(w, 400, "Baris ini bukan REVIEW yang bisa dikonfirmasi.")
		return
	}

	candCode := target.CandCode
	candDescOrCode := target.CandDesc
	candDesc := firstNonEmpty(candDescOrCode, candCode)
	for _, i := range idx {
		if confirm {
			results[i].Flag = "EXISTING"
			results[i].NeedsReview = false
			results[i].MasterCode = candCode
			results[i].MasterDesc = orDash(candDescOrCode)
			results[i].Note = "Dikonfirmasi manual dari REVIEW: " + candDesc
		} else {
			results[i].Flag = "NEW ITEM"
			results[i].NeedsReview = false
			results[i].MasterCode = "-"
			results[i].MasterDesc = "-"
			results[i].Note = "Divalidasi manual: item baru (bukan " + candDesc + ")"
			results[i].CandCode = ""
			results[i].CandDesc = ""
		}
	}

	dKey := normStr(target.Brand) + "|" + normStr(target.NamaItem)
	if confirm {
		_, _ = db.Exec(
			`INSERT INTO review_decisions (key, brand, nama_item, decision, master_code, master_desc, hits)
			 VALUES ($1,$2,$3,'link',$4,$5,1)
			 ON CONFLICT(key) DO UPDATE SET decision='link', master_code=excluded.master_code,
			   master_desc=excluded.master_desc, hits=review_decisions.hits+$6`,
			dKey, target.Brand, target.NamaItem, candCode, candDescOrCode, len(idx))
	} else {
		_, _ = db.Exec(
			`INSERT INTO review_decisions (key, brand, nama_item, decision, hits) VALUES ($1,$2,$3,'new',1)
			 ON CONFLICT(key) DO UPDATE SET decision='new', hits=review_decisions.hits+$4`,
			dKey, target.Brand, target.NamaItem, len(idx))
	}
	exist, review, err := a.saveReviewResults(db, p.ResultID, results)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "row": target, "exist": exist,
		"new": len(results) - exist, "review": review, "applied": len(idx)})
}

func (a *App) mcExport(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ResultID int          `json:"resultId"`
		Results  []compareRow `json:"results"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	results := payload.Results
	if payload.ResultID != 0 {
		db := a.requireDB(w)
		if db == nil {
			return
		}
		_, rows, ok, err := a.loadCompareResults(db, payload.ResultID)
		if err != nil || !ok {
			sendErr(w, 500, "Hasil compare tidak ditemukan.")
			return
		}
		results = rows
	}
	buf, err := a.exportMCExcel(results)
	if err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	filename := "MC_Hasil_" + time.Now().UTC().Format("2006-01-02") + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	_, _ = w.Write(buf)
}

func (a *App) mcDelete(w http.ResponseWriter, r *http.Request, kind string) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	var payload struct {
		UploadID int `json:"uploadId"`
	}
	if err := readJSONBody(r, &payload); err != nil {
		sendErr(w, 400, err.Error())
		return
	}
	switch kind {
	case "ml":
		if _, err := db.Exec(`DELETE FROM masterlist_items WHERE upload_id=$1`, payload.UploadID); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if _, err := db.Exec(`DELETE FROM masterlist_uploads WHERE id=$1`, payload.UploadID); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
	case "pl":
		if _, err := db.Exec(`DELETE FROM packing_items WHERE upload_id=$1`, payload.UploadID); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if _, err := db.Exec(`DELETE FROM compare_results WHERE packing_upload_id=$1`, payload.UploadID); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
		if _, err := db.Exec(`DELETE FROM packing_uploads WHERE id=$1`, payload.UploadID); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) mcClearAll(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	for _, t := range []string{"packing_items", "packing_uploads", "masterlist_items", "masterlist_uploads", "compare_results"} {
		if _, err := db.Exec(`DELETE FROM ` + t); err != nil {
			sendErr(w, 500, err.Error())
			return
		}
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) pedomanList(w http.ResponseWriter, r *http.Request) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	uploads := a.queryUploads(`SELECT id, filename, uploaded_at, 0 AS approved, total_rows, 'done' AS status FROM pedoman_uploads ORDER BY uploaded_at DESC`)
	if uploads == nil {
		uploads = []uploadRow{}
	}
	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pedoman_items`).Scan(&total)
	sendJSON(w, 200, map[string]any{"uploads": uploads, "total": total})
}

func (a *App) pedomanDelete(w http.ResponseWriter, r *http.Request, idStr string) {
	db := a.requireDB(w)
	if db == nil {
		return
	}
	id, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || id <= 0 {
		sendErr(w, 400, "ID tidak valid")
		return
	}
	if _, err := db.Exec(`DELETE FROM pedoman_uploads WHERE id=$1`, id); err != nil {
		sendErr(w, 500, err.Error())
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}
