package main

import (
	"database/sql"
	"os"
	"testing"
)

func TestDebugMMDRevisi(t *testing.T) {
	if os.Getenv("CTF_DEBUG") == "" {
		t.Skip("audit data asli (set CTF_DEBUG=1 untuk menjalankan)")
	}
	db := openDB()
	if db == nil {
		t.Fatal("database tidak tersedia")
	}
	defer db.Close()

	var total, bersih int
	db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE bersih) FROM mmd_items`).Scan(&total, &bersih)
	t.Logf("mmd_items: total=%d bersih=%d", total, bersih)

	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name='mmd_items' AND column_name='diubah_pada'`).Scan(&cnt); err != nil {
		t.Errorf("cek kolom diubah_pada: %v", err)
	} else {
		t.Logf("kolom mmd_items.diubah_pada: %d", cnt)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_name='mmd_revisi'`).Scan(&cnt); err != nil {
		t.Errorf("cek tabel mmd_revisi: %v", err)
	} else {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM mmd_revisi`).Scan(&n)
		t.Logf("tabel mmd_revisi: %d (baris: %d)", cnt, n)
	}

	var last sql.NullString
	db.QueryRow(`SELECT MAX(tanggal_masuk)::date FROM mmd_items`).Scan(&last)
	t.Logf("tanggal_masuk terakhir: %s", last.String)

	rows, err := db.Query(`SELECT item_code, deskripsi, kategori, status_aktif,
		to_char(tanggal_masuk,'YYYY-MM-DD'), nama_upload, bersih
		FROM mmd_items WHERE upper(deskripsi) LIKE '%CEDAR MEAT%'
		ORDER BY item_code`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var code, desc, kategori, tgl, namaUpload string
		var aktif int
		var isBersih bool
		if err := rows.Scan(&code, &desc, &kategori, &aktif, &tgl, &namaUpload, &isBersih); err != nil {
			t.Fatal(err)
		}
		n++
		t.Logf("[%d] code=%q desc=%q kategori=%q aktif=%d tgl=%s upload=%q bersih=%v",
			n, code, desc, kategori, aktif, tgl, namaUpload, isBersih)
	}
	if n == 0 {
		t.Log("tidak ada baris CEDAR MEAT sama sekali")
	}

	all, err := db.Query(`SELECT item_code, deskripsi FROM mmd_items WHERE bersih`)
	if err != nil {
		t.Fatalf("query semua: %v", err)
	}
	defer all.Close()
	type row struct{ code, desc, flat string }
	var list []row
	for all.Next() {
		var r row
		if err := all.Scan(&r.code, &r.desc); err != nil {
			break
		}
		r.flat = flatKey(r.desc)
		list = append(list, r)
	}
	t.Logf("baris bersih dibaca: %d", len(list))

	byFlat := map[string][]row{}
	for _, r := range list {
		pref := r.flat
		if len(pref) > 12 {
			pref = pref[:12]
		}
		byFlat[pref] = append(byFlat[pref], r)
	}
	ketemu := 0
	for _, rs := range byFlat {
		if len(rs) < 2 {
			continue
		}
		if len(rs) > 80 {
			t.Logf("grup prefix %q dilewati (terlalu besar: %d baris)", rs[0].flat, len(rs))
			continue
		}
		for i := 0; i < len(rs) && ketemu < 20; i++ {
			for j := i + 1; j < len(rs); j++ {
				if rs[i].code == rs[j].code {
					continue
				}
				d := levDistSimple(rs[i].flat, rs[j].flat)
				if d == 0 || d > 3 {
					continue
				}
				ketemu++
				t.Logf("kandidat revisi (beda %d huruf): code=%q desc=%q || code=%q desc=%q",
					d, rs[i].code, rs[i].desc, rs[j].code, rs[j].desc)
				break
			}
		}
	}
	t.Logf("kandidat revisi deskripsi antar kode: %d (ditampilkan maks 20)", ketemu)

	days, err := db.Query(`SELECT to_char(tanggal_masuk,'YYYY-MM-DD'), COUNT(*)
		FROM mmd_items WHERE bersih GROUP BY 1 ORDER BY 1 DESC LIMIT 10`)
	if err == nil {
		defer days.Close()
		for days.Next() {
			var d string
			var c int
			days.Scan(&d, &c)
			t.Logf("tanggal %s: %d baris", d, c)
		}
	}
	ups, err := db.Query(`SELECT id, filename, to_char(uploaded_at,'YYYY-MM-DD HH24:MI'), total_rows, baru, diperbarui
		FROM mmd_uploads ORDER BY id DESC LIMIT 10`)
	if err == nil {
		defer ups.Close()
		for ups.Next() {
			var id, total, baru, diperbarui int
			var fn, at string
			ups.Scan(&id, &fn, &at, &total, &baru, &diperbarui)
			t.Logf("upload #%d %s %s total=%d baru=%d diperbarui=%d", id, at, fn, total, baru, diperbarui)
		}
	}
}

func flatKey(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c)
		case c >= 'a' && c <= 'z':
			out = append(out, c-32)
		case c >= '0' && c <= '9':
			out = append(out, c)
		}
	}
	return string(out)
}
