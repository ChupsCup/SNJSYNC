package main

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
)

func TestDumpDraftKamus(t *testing.T) {
	dir := os.Getenv("CTF_ROOT")
	if dir == "" {
		t.Skip("CTF_ROOT kosong")
	}
	db := openDB()
	if db == nil {
		t.Skip("DB tidak hidup")
	}
	defer db.Close()
	rows, err := db.Query("SELECT item_desc FROM masterlist_items WHERE item_desc <> ''")
	if err != nil {
		t.Fatal(err)
	}
	var descs []string
	for rows.Next() {
		var d sql.NullString
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		if d.Valid {
			descs = append(descs, d.String)
		}
	}
	rows.Close()

	d := deriveDictDraft(descs)
	fmt.Printf("=== DRAFT KAMUS dari ML ===\n")
	fmt.Printf("baris ML        : %d\n", len(descs))
	fmt.Printf("nama unik       : %d\n", d.TotalName)
	fmt.Printf("kunci kanonik   : %d\n", d.UniqueKey)
	fmt.Printf("nama yang kolaps: %d\n", d.Collapsed)
	fmt.Printf("dengan kelompok : %d\n", len(d.Groups))
	fmt.Printf("BELUM kelompok  : %d\n\n", len(d.NoGroup))

	fmt.Printf("--- VARIAN GABUNG (nama berbeda, produk sama) ---\n")
	n := 0
	for _, g := range d.Groups {
		if len(g.Variants) > 1 {
			fmt.Printf("  %-24s <- %v\n", g.Key, g.Variants)
			n++
		}
	}
	for _, g := range d.NoGroup {
		if len(g.Variants) > 1 {
			fmt.Printf("  %-24s <- %v\n", g.Key, g.Variants)
			n++
		}
	}
	fmt.Printf("total grup variant: %d\n\n", n)

	fmt.Printf("--- BELUM TERKLASIFIKASI (grup kosong, wajib diputuskan manusia) ---\n")
	for i, g := range d.NoGroup {
		if i >= 40 {
			fmt.Printf("  ... (+%d lagi)\n", len(d.NoGroup)-40)
			break
		}
		tag := ""
		if len(g.Variants) > 1 {
			tag = fmt.Sprintf("  <-- %d VARIAN", len(g.Variants))
		}
		fmt.Printf("  %-26s (%d baris)%s\n", g.Key, g.Count, tag)
	}
}
