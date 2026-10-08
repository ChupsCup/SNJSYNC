package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type comparePayload struct {
	UploadID  int      `json:"uploadId"`
	UploadIDs []int    `json:"uploadIds"`
	Only      []string `json:"only"`
	DryRun    bool     `json:"dryRun"`
	Verbose   bool     `json:"verbose"`
}

type compareResp struct {
	ResultID *int         `json:"resultId"`
	Total    int          `json:"total"`
	Exist    int          `json:"exist"`
	New      int          `json:"new"`
	Review   int          `json:"review"`
	Skipped  int          `json:"skipped"`
	Results  []compareRow `json:"results"`
}

func (p comparePayload) ids() []int {
	if len(p.UploadIDs) > 0 {
		return p.UploadIDs
	}
	if p.UploadID > 0 {
		return []int{p.UploadID}
	}
	return nil
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullInt(ni sql.NullInt64) int {
	if ni.Valid {
		return int(ni.Int64)
	}
	return 0
}

func (a *App) loadApprovedMaster() []mlItem {
	if a.db == nil {
		return nil
	}
	ctx, cancel := ctxTimeout(120 * time.Second)
	defer cancel()
	rows, err := a.db.QueryContext(ctx, `SELECT m.id, m.upload_id, m.no_container, m.brand, m.nama_item,
		m.item_code, m.item_desc, m.gabungan, m.importir
		FROM masterlist_items m
		JOIN masterlist_uploads u ON u.id = m.upload_id
		WHERE u.status = 'done' ORDER BY m.id`)
	if err != nil {
		logf("MC master query: %v", err)
		return nil
	}
	defer rows.Close()
	var out []mlItem
	for rows.Next() {
		var m mlItem
		var noC, brand, nama, code, desc, gab, imp sql.NullString
		if err := rows.Scan(&m.ID, &m.UploadID, &noC, &brand, &nama, &code, &desc, &gab, &imp); err != nil {
			continue
		}
		m.NoContainer, m.Brand, m.NamaItem = nullStr(noC), nullStr(brand), nullStr(nama)
		m.ItemCode, m.ItemDesc, m.Gabungan, m.Importir = nullStr(code), nullStr(desc), nullStr(gab), nullStr(imp)
		out = append(out, m)
	}
	return out
}

func (a *App) loadPackingRows(uploadIDs []int) []plItem {
	if a.db == nil || len(uploadIDs) == 0 {
		return nil
	}
	ctx, cancel := ctxTimeout(120 * time.Second)
	defer cancel()
	args := make([]any, len(uploadIDs))
	for i, v := range uploadIDs {
		args[i] = v
	}
	rows, err := a.db.QueryContext(ctx, `SELECT id, upload_id, sheet, no_container, brand, nama_item,
		eta, item_code, item_desc, gabungan, importir
		FROM packing_items WHERE upload_id IN (`+placeholders(len(uploadIDs))+`)
		ORDER BY upload_id, id`, args...)
	if err != nil {
		logf("MC packing query: %v", err)
		return nil
	}
	defer rows.Close()
	var out []plItem
	for rows.Next() {
		var r plItem
		var sh, noC, brand, nama, eta, code, desc, gab, imp sql.NullString
		if err := rows.Scan(&r.ID, &r.UploadID, &sh, &noC, &brand, &nama, &eta, &code, &desc, &gab, &imp); err != nil {
			continue
		}
		r.Sheet, r.NoContainer, r.Brand, r.NamaItem = nullStr(sh), nullStr(noC), nullStr(brand), nullStr(nama)
		r.ETA, r.ItemCode, r.ItemDesc = nullStr(eta), nullStr(code), nullStr(desc)
		r.Gabungan, r.Importir = nullStr(gab), nullStr(imp)
		out = append(out, r)
	}
	return out
}

func (a *App) packingFilenames(uploadIDs []int) map[int]string {
	out := map[int]string{}
	if a.db == nil || len(uploadIDs) == 0 {
		return out
	}
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	args := make([]any, len(uploadIDs))
	for i, v := range uploadIDs {
		args[i] = v
	}
	rows, err := a.db.QueryContext(ctx, `SELECT id, filename FROM packing_uploads
		WHERE id IN (`+placeholders(len(uploadIDs))+`)`, args...)
	if err != nil {
		logf("MC packing filename query: %v", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		out[id] = nullStr(name)
	}
	return out
}

func (a *App) loadDecisions() map[string]*reviewDecision {
	out := map[string]*reviewDecision{}
	if a.db == nil {
		return out
	}
	ctx, cancel := ctxTimeout(30 * time.Second)
	defer cancel()
	rows, err := a.db.QueryContext(ctx,
		`SELECT key, decision, master_code, master_desc FROM review_decisions`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var key, dec string
		var mc, md sql.NullString
		if err := rows.Scan(&key, &dec, &mc, &md); err != nil {
			continue
		}
		out[key] = &reviewDecision{Key: key, Decision: dec, MasterCode: nullStr(mc), MasterDesc: nullStr(md)}
	}
	return out
}

func (a *App) runCompare(payload comparePayload, fallbackRows []plItem) (*compareResp, error) {
	uploadIDs := payload.ids()

	var packingRows []plItem
	if a.db != nil && len(uploadIDs) > 0 {
		packingRows = a.loadPackingRows(uploadIDs)
	} else if len(fallbackRows) > 0 {
		packingRows = fallbackRows
	} else {
		return nil, fmt.Errorf("Upload packing list dulu!")
	}
	if len(packingRows) == 0 {
		return nil, fmt.Errorf("Packing list kosong!")
	}

	if len(uploadIDs) > 0 {
		fileByUpload := a.packingFilenames(uploadIDs)
		for i := range packingRows {
			packingRows[i].File = fileByUpload[packingRows[i].UploadID]
		}
	}

	var onlyKw []string
	for _, s := range payload.Only {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			onlyKw = append(onlyKw, s)
		}
	}
	if len(onlyKw) > 0 {
		filtered := make([]plItem, 0, len(packingRows))
		for _, r := range packingRows {
			hay := strings.ToLower(r.Brand + " " + r.NamaItem + " " + r.ItemDesc + " " + r.Gabungan)
			for _, kw := range onlyKw {
				if strings.Contains(hay, kw) {
					filtered = append(filtered, r)
					break
				}
			}
		}
		packingRows = filtered
	}

	approved := a.loadApprovedMaster()
	if len(approved) == 0 {
		return nil, fmt.Errorf("Belum ada masterlist ter-upload! Upload masterlist dulu sebelum compare.")
	}

	eng := newCompareEngine()
	eng.build(approved, a.loadDecisions())

	results := make([]compareRow, 0, len(packingRows))
	existCount, newCount, reviewCount, skipped := 0, 0, 0, 0

	rowCache := map[string]lookupHit{}
	verbose := payload.Verbose

	for i := range packingRows {
		r := &packingRows[i]
		if strings.TrimSpace(r.NamaItem) == "" {
			skipped++
			continue
		}

		sig := normStr(r.Brand + "|" + r.NamaItem + "|" + r.ItemDesc + "|" + r.Gabungan +
			"|" + r.Sheet + "|" + r.Importir + "|" + r.File)
		hit, cached := rowCache[sig]
		if !cached {
			hit = eng.lookupMaster(r)
			rowCache[sig] = hit
		}
		flag := "NEW ITEM"
		if hit.status == "EXISTING" {
			flag = "EXISTING"
		}
		needsReview := hit.review
		if flag == "EXISTING" {
			existCount++
		} else {
			newCount++
		}
		if needsReview {
			reviewCount++
		}
		mc, md := "-", "-"
		var cand *mlItem
		if hit.m != nil {
			mc = orDash(hit.m.ItemCode)
			md = orName(hit.m.ItemDesc, hit.m.NamaItem)
		} else if hit.cand != nil {
			cand = hit.cand
			mc = orDash(hit.cand.ItemCode)
			md = orName(hit.cand.ItemDesc, hit.cand.NamaItem)
		}
		if verbose {
			fmt.Println("[ROW]", strings.Join([]string{
				r.Brand, r.NamaItem, flag, mc, md, hit.note,
			}, " | "))
		}
		row := compareRow{
			No:          len(results) + 1,
			NoContainer: r.NoContainer,
			Brand:       r.Brand,
			NamaItem:    r.NamaItem,
			ETA:         r.ETA,
			ItemCode:    r.ItemCode,
			ItemDesc:    r.ItemDesc,
			Sheet:       r.Sheet,
			Importir:    r.importirPL(r.ItemDesc, r.Gabungan),
			Flag:        flag,
			NeedsReview: needsReview,
			Note:        hit.note,
			Score:       round2(hit.sym),
			MasterCode:  mc,
			MasterDesc:  md,
		}
		if cand != nil {
			row.CandCode = cand.ItemCode
			row.CandDesc = orName(cand.ItemDesc, cand.NamaItem)
		}
		results = append(results, row)
	}

	resp := &compareResp{
		Total: len(results), Exist: existCount, New: newCount,
		Review: reviewCount, Skipped: skipped, Results: results,
	}

	if a.db != nil && len(uploadIDs) > 0 && !payload.DryRun {
		js, err := json.Marshal(results)
		if err != nil {
			return nil, err
		}
		plID := uploadIDs[0]
		filename := "Compare #" + itoa(plID)
		if len(uploadIDs) > 1 {
			filename = itoa(len(uploadIDs)) + " files"
		}
		ctx, cancel := ctxTimeout(120 * time.Second)
		defer cancel()
		var id int
		err = a.db.QueryRowContext(ctx,
			`INSERT INTO compare_results (packing_upload_id, filename, total, exist_count, new_count, review_count, results_json)
			 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			plID, filename, resp.Total, resp.Exist, resp.New, resp.Review, string(js)).Scan(&id)
		if err != nil {
			logf("MC compare insert: %v", err)
			return nil, fmt.Errorf("hasil compare tidak bisa disimpan: %v", err)
		}
		resp.ResultID = &id
	}
	return resp, nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func orName(desc, nama string) string {
	if desc != "" {
		return desc
	}
	if nama != "" {
		return nama
	}
	return "-"
}

func round2(v float64) float64 {
	if v < 0 {
		v = 0
	}
	return float64(int(v*100+0.5)) / 100
}
