package main

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"strings"
)

const mmdSchemaSQL = `
CREATE TABLE IF NOT EXISTS mmd_items (
  id SERIAL PRIMARY KEY,
  -- data mentah apa adanya dari file (sebelum dibersihkan)
  kode_mentah   TEXT DEFAULT '',
  nama_mentah   TEXT DEFAULT '',
  merek         TEXT DEFAULT '',
  non_aktif_mentah TEXT DEFAULT '',
  -- kolom turunan
  item_code     TEXT DEFAULT '',
  deskripsi     TEXT DEFAULT '',
  kategori      TEXT DEFAULT '',
  status_aktif  INTEGER DEFAULT 1,
  potongan_utama   TEXT DEFAULT '',
  potongan_spesifik TEXT DEFAULT '',
  -- true kalau baris ini berasal dari file yang sudah dibersihkan
  bersih        BOOLEAN DEFAULT FALSE,
  -- dedup
  dedup_key     TEXT UNIQUE,
  tanggal_masuk TIMESTAMP DEFAULT NOW(),
  upload_id     INTEGER,
  nama_upload   TEXT DEFAULT ''
);
ALTER TABLE mmd_items ADD COLUMN IF NOT EXISTS deskripsi TEXT DEFAULT '';
ALTER TABLE mmd_items ADD COLUMN IF NOT EXISTS bersih BOOLEAN DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_mmd_kategori  ON mmd_items(kategori);
CREATE INDEX IF NOT EXISTS idx_mmd_status    ON mmd_items(status_aktif);
CREATE INDEX IF NOT EXISTS idx_mmd_tanggal   ON mmd_items(tanggal_masuk);
CREATE INDEX IF NOT EXISTS idx_mmd_itemcode  ON mmd_items(item_code);
CREATE INDEX IF NOT EXISTS idx_mmd_utama     ON mmd_items(potongan_utama);

-- Riwayat upload MMD. TERPISAH dari pedoman_uploads: MMD (Master
-- Descriptor untuk user) dan Pedoman (whitelist 3 grosir) dua hal beda.
CREATE TABLE IF NOT EXISTS mmd_uploads (
  id SERIAL PRIMARY KEY,
  filename TEXT NOT NULL,
  uploaded_at TIMESTAMP DEFAULT NOW(),
  total_rows INTEGER DEFAULT 0,
  baru INTEGER DEFAULT 0,
  diperbarui INTEGER DEFAULT 0
);

-- Bersihkan sisa jalur lama: MMD pernah nyimpen upload_id ke
-- pedoman_uploads, sehingga file MMD nongol di daftar pedoman.
-- Hapus hanya baris yang BENAR-BENAR bukan pedoman (nggak punya
-- pedoman_items). Pedoman asli ikut hapus kalau cascade, jadi
-- upload_id MMD dilepas dulu.
UPDATE mmd_items
   SET upload_id = NULL
 WHERE upload_id IN (SELECT p.id FROM pedoman_uploads p
                      WHERE NOT EXISTS (SELECT 1 FROM pedoman_items i WHERE i.upload_id = p.id));

DELETE FROM pedoman_uploads p
      WHERE NOT EXISTS (SELECT 1 FROM pedoman_items i WHERE i.upload_id = p.id);

-- Baris MMD bukan-bersih berasal dari jalur lama saat file mentah
-- (.xls hierarki) salah dipakai sebagai file bersih. Sudah tidak
-- dipakai user view (query hanya ambil bersih = TRUE).
DELETE FROM mmd_items WHERE NOT bersih;

-- Backfill dedup_key ke gaya "per item code".
-- Baris dengan item_code kosong dibuang supaya tidak saling tabrak
-- di unique index.
DELETE FROM mmd_items WHERE btrim(item_code) = '' AND bersih;

UPDATE mmd_items
   SET dedup_key = upper(regexp_replace(btrim(item_code), '[^A-Za-z0-9]', '', 'g'))
 WHERE btrim(item_code) <> ''
   AND (dedup_key IS NULL
        OR dedup_key <> upper(regexp_replace(btrim(item_code), '[^A-Za-z0-9]', '', 'g')));

-- Kalau sisa kode dobel (jarang, hanya dari file lama), sisakan yang
-- terbaru supaya unique index tetap bisa dibuat.
DELETE FROM mmd_items a
      USING mmd_items b
      WHERE a.dedup_key = b.dedup_key
        AND a.dedup_key IS NOT NULL
        AND (a.id < b.id
             OR (a.id = b.id AND a.tanggal_masuk < b.tanggal_masuk));

CREATE UNIQUE INDEX IF NOT EXISTS idx_mmd_dedup ON mmd_items(dedup_key);

-- Jejak revisi deskripsi. File hari-2 memuat teks deskripsi yang sudah
-- diperbaiki (mis. "CEDAR MEAT" -> "CEDAR MEATS") dan tetap menimpa baris
-- lama; teks sebelumnya dicatat di sini supaya portal bisa menandai baris
-- yang berubah dan menampilkan isinya saat diklik.
-- diubah_pada = waktu terakhir deskripsi BENAR-BENAR berubah (beda teks),
-- bukan sekadar baris disentuh file baru; tanggal_masuk tetap tanggal asli
-- baris masuk dan tidak boleh di-reset.
ALTER TABLE mmd_items ADD COLUMN IF NOT EXISTS diubah_pada TIMESTAMP DEFAULT NOW();
CREATE TABLE IF NOT EXISTS mmd_revisi (
  id SERIAL PRIMARY KEY,
  dedup_key      TEXT NOT NULL,
  item_code      TEXT DEFAULT '',
  deskripsi_lama TEXT NOT NULL,
  deskripsi_baru TEXT NOT NULL,
  revised_at     TIMESTAMP DEFAULT NOW(),
  nama_upload    TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_mmd_revisi_key ON mmd_revisi(dedup_key);
`

type revisiBaris struct {
	key, code, lama, baru string
}

func bedaDeskripsi(lama, baru string) bool {
	return strings.TrimSpace(lama) != strings.TrimSpace(baru)
}

func revisiValues(kolom, baris int) string {
	parts := make([]string, 0, baris)
	n := 0
	for i := 0; i < baris; i++ {
		ps := make([]string, 0, kolom)
		for j := 0; j < kolom; j++ {
			n++
			ps = append(ps, "$"+strconv.Itoa(n))
		}
		parts = append(parts, "("+strings.Join(ps, ",")+")")
	}
	return strings.Join(parts, ",")
}

func simpanRevisiMMD(ctx context.Context, db *sql.DB, list []revisiBaris, namaUpload string) error {
	if db == nil || len(list) == 0 {
		return nil
	}
	const kolom, chunk = 5, 500
	for start := 0; start < len(list); start += chunk {
		end := start + chunk
		if end > len(list) {
			end = len(list)
		}
		baris := end - start
		args := make([]any, 0, baris*kolom)
		for _, rv := range list[start:end] {
			args = append(args, rv.key, rv.code, rv.lama, rv.baru, namaUpload)
		}
		q := `INSERT INTO mmd_revisi (dedup_key, item_code, deskripsi_lama, deskripsi_baru, nama_upload)
		      VALUES ` + revisiValues(kolom, baris)
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return nil
}

func applyMMDSchema(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec(mmdSchemaSQL); err != nil {
		for _, stmt := range splitStatements(mmdSchemaSQL) {
			if _, err := db.Exec(stmt); err != nil {
				log.Println("mmd schema:", err)
			}
		}
	}
}

func backfillDedupMMD(db *sql.DB) {
	if db == nil {
		return
	}
	stmts := []string{
		`DELETE FROM mmd_items WHERE btrim(item_code) = '' AND bersih`,
		`UPDATE mmd_items
		    SET dedup_key = upper(regexp_replace(btrim(item_code), '[^A-Za-z0-9]', '', 'g'))
		  WHERE btrim(item_code) <> ''
		    AND (dedup_key IS NULL
		         OR dedup_key <> upper(regexp_replace(btrim(item_code), '[^A-Za-z0-9]', '', 'g')))`,
		`DELETE FROM mmd_items a USING mmd_items b
		  WHERE a.dedup_key = b.dedup_key
		    AND a.dedup_key IS NOT NULL
		    AND (a.id < b.id
		         OR (a.id = b.id AND a.tanggal_masuk < b.tanggal_masuk))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_mmd_dedup ON mmd_items(dedup_key)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			log.Println("mmd dedup:", err)
		}
	}
}

func mmdAktif(nonAktif string) int {
	v := strings.TrimSpace(strings.ToLower(nonAktif))
	if v == "" {
		return 1
	}
	if strings.HasPrefix(v, "t") {
		return 1
	}
	if v == "ya" || v == "y" || v == "1" || v == "true" {
		return 0
	}
	return 1
}

func mmdPecahDeskripsi(desc string) (utama, spesifik string) {
	d := bersihkan(desc)
	if d == "" {
		return "", ""
	}
	utama = d
	if i := strings.Index(d, "---"); i >= 0 {
		utama = bersihkan(d[:i])
		sisa := bersihkan(d[i:])
		sisa = strings.TrimLeft(sisa, "- ")
		if sisa != "" {
			return utama, sisa
		}
		return utama, ""
	}

	if i := strings.Index(d, "("); i > 0 {
		return bersihkan(d[:i]), d
	}
	return utama, ""
}
