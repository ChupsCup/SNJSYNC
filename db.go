package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func ctxTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

type dbConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	MaxOpen  int
}

func loadDBConfig() dbConfig {

	c := dbConfig{Host: "localhost", Port: 5432, User: "ctf", Database: "ctf_db", MaxOpen: 10}
	if v := os.Getenv("CTF_DB_HOST"); v != "" {
		c.Host = v
	}
	if v := os.Getenv("CTF_DB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.Port = n
		}
	}
	if v := os.Getenv("CTF_DB_USER"); v != "" {
		c.User = v
	}
	c.Password = secretValue("CTF_DB_PASSWORD")
	if v := os.Getenv("CTF_DB_NAME"); v != "" {
		c.Database = v
	}
	return c
}

func secretValue(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dotEnvValue(key)
}

func dotEnvValue(key string) string {
	b, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i < 1 || strings.TrimSpace(line[:i]) != key {
			continue
		}
		v := strings.TrimSpace(line[i+1:])
		return strings.Trim(v, `"'`)
	}
	return ""
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS masterlist_uploads (
  id SERIAL PRIMARY KEY,
  filename TEXT NOT NULL,
  sheet_name TEXT NOT NULL DEFAULT '',
  uploaded_at TIMESTAMP DEFAULT NOW(),
  approved INTEGER DEFAULT 0,
  total_rows INTEGER DEFAULT 0,
  status TEXT DEFAULT 'done'
);
CREATE TABLE IF NOT EXISTS masterlist_items (
  id SERIAL PRIMARY KEY,
  upload_id INTEGER NOT NULL,
  no_container TEXT,
  brand TEXT,
  nama_item TEXT,
  item_code TEXT,
  item_desc TEXT,
  gabungan TEXT,
  importir TEXT,
  FOREIGN KEY (upload_id) REFERENCES masterlist_uploads(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS packing_uploads (
  id SERIAL PRIMARY KEY,
  filename TEXT NOT NULL,
  uploaded_at TIMESTAMP DEFAULT NOW(),
  total_rows INTEGER DEFAULT 0,
  status TEXT DEFAULT 'done'
);
CREATE TABLE IF NOT EXISTS packing_items (
  id SERIAL PRIMARY KEY,
  upload_id INTEGER NOT NULL,
  sheet TEXT,
  no_container TEXT,
  brand TEXT,
  nama_item TEXT,
  eta TEXT,
  item_code TEXT,
  item_desc TEXT,
  gabungan TEXT,
  importir TEXT,
  FOREIGN KEY (upload_id) REFERENCES packing_uploads(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS compare_results (
  id SERIAL PRIMARY KEY,
  packing_upload_id INTEGER NOT NULL,
  filename TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  total INTEGER DEFAULT 0,
  exist_count INTEGER DEFAULT 0,
  new_count INTEGER DEFAULT 0,
  approved INTEGER DEFAULT 0,
  results_json TEXT,
  review_count INTEGER DEFAULT 0,
  FOREIGN KEY (packing_upload_id) REFERENCES packing_uploads(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS pedoman_uploads (
  id SERIAL PRIMARY KEY,
  filename TEXT NOT NULL,
  uploaded_at TIMESTAMP DEFAULT NOW(),
  total_rows INTEGER DEFAULT 0
);
CREATE TABLE IF NOT EXISTS pedoman_items (
  id SERIAL PRIMARY KEY,
  upload_id INTEGER NOT NULL,
  kode_barang TEXT,
  nama_barang TEXT,
  merek_barang TEXT,
  non_aktif TEXT,
  FOREIGN KEY (upload_id) REFERENCES pedoman_uploads(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS review_decisions (
  key TEXT PRIMARY KEY,
  brand TEXT,
  nama_item TEXT,
  decision TEXT DEFAULT 'link',
  master_code TEXT,
  master_desc TEXT,
  hits INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS mc_aliases (
  id SERIAL PRIMARY KEY,
  left_text TEXT NOT NULL,
  right_text TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mc_aliases_pair ON mc_aliases(left_text, right_text);
CREATE TABLE IF NOT EXISTS mc_ignore_words (
  id SERIAL PRIMARY KEY,
  word TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mc_ignore_words_word ON mc_ignore_words(word);
CREATE INDEX IF NOT EXISTS idx_packing_items_upload ON packing_items(upload_id);
CREATE INDEX IF NOT EXISTS idx_masterlist_items_upload ON masterlist_items(upload_id);
CREATE INDEX IF NOT EXISTS idx_compare_results_packing ON compare_results(packing_upload_id);
`

func openDB() *sql.DB {
	c := loadDBConfig()
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password, c.Database)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Println("MC DB error:", err)
		return nil
	}
	db.SetMaxOpenConns(c.MaxOpen)
	db.SetMaxIdleConns(c.MaxOpen)
	db.SetConnMaxLifetime(time.Hour)
	pingCtx, cancel := ctxTimeout(10 * time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Println("MC DB error:", err)
		db.Close()
		return nil
	}
	if _, err := db.Exec(schemaSQL); err != nil {

		for _, stmt := range splitStatements(schemaSQL) {
			if _, err := db.Exec(stmt); err != nil {
				log.Println("MC DB schema:", err)
			}
		}
	}
	log.Println("MC database OK (PostgreSQL)")
	return db
}

func splitStatements(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ";") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func placeholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "$" + itoa(i+1)
	}
	return strings.Join(parts, ",")
}
