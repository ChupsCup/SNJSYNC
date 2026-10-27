package main

import "testing"

func TestIgnoreWordsDasar(t *testing.T) {
	iw := newIgnoreWords([]mcIgnore{{Word: "CARTILAGE"}, {Word: "meat"}})
	if !iw.Has("cartilage") || !iw.Has("meat") {
		t.Fatalf("kata tidak terdaftar: %v", iw.set.List())
	}
	if iw.Has("lips") {
		t.Fatal("kata lain ikut terabaikan")
	}
	var nilIw *ignoreWords
	if nilIw.Has("meat") {
		t.Fatal("nil ignore tidak boleh cocok")
	}
	s := newSetFromSlice([]string{"lips", "cartilage"})
	nilIw.filterSet(s)
	if !s.Has("cartilage") {
		t.Fatal("nil ignore tidak boleh mengubah set")
	}
	iw.filterSet(s)
	if s.Has("cartilage") || !s.Has("lips") {
		t.Fatalf("filterSet salah: %v", s.List())
	}
	if newIgnoreWords([]mcIgnore{{Word: "dua kata"}}).set.Len() != 0 {
		t.Fatal("frasa harus ditolak")
	}
}

func TestIgnoreKataEkstraPL(t *testing.T) {
	masters := []mlItem{
		{ID: 1, ItemCode: "600010-10.140BR.03085", ItemDesc: "SCAPULA (AMG) --- BP --- EST.3085 AU"},
		{ID: 2, ItemCode: "600080-10.027AU.03856", ItemDesc: "LIPS (GREENHAM) ------ EST.234 AU"},
		{ID: 3, ItemCode: "600080-10.150BR.03193", ItemDesc: "LIPS (BARRA MANSA) ---PAPILLAE BKR--- EST. BR"},
		{ID: 4, ItemCode: "600010-10.057BR.03161", ItemDesc: "CHEEK (MINERVA) ---500101507 BKR--- EST. BR"},
		{ID: 5, ItemCode: "600010-10.140BR.03589", ItemDesc: "CHEEK MEAT 90VL (NOSSO) ---500101507 BKR--- EST. BR"},
		{ID: 6, ItemCode: "600010-10.153BR.03276", ItemDesc: "CHEEK (FRIGON) ---25324 PPI--- EST. BR"},
		{ID: 7, ItemCode: "600010-10.120BR.03560", ItemDesc: "CHEEK MEAT 90VL (FRIGOL) ---077530 PPI--- EST. BR"},
	}
	cases := []struct {
		nama     string
		pl       plItem
		kata     string
		wantCode string
	}{
		{
			nama:     "lips papillae",
			pl:       plItem{Brand: "GREENHAM", NamaItem: "LIPS PAPILLAE EST - 234", Gabungan: "GREENHAM LIPS PAPILLAE EST - 234"},
			kata:     "PAPILLAE",
			wantCode: "600080-10.027AU.03856",
		},
		{
			nama:     "cheek meat",
			pl:       plItem{Brand: "MINERVA", NamaItem: "CHEEK MEAT", Gabungan: "MINERVA CHEEK MEAT"},
			kata:     "MEAT",
			wantCode: "600010-10.057BR.03161",
		},
		{
			nama:     "cheek meat frigon",
			pl:       plItem{Brand: "FRIGON", NamaItem: "FROZEN BONELESS BEEF MEAT CHEEK MEAT", Gabungan: "FRIGON FROZEN BONELESS BEEF MEAT CHEEK MEAT"},
			kata:     "MEAT",
			wantCode: "600010-10.153BR.03276",
		},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			eng := newCompareEngine()
			eng.build(masters, nil)
			before := eng.lookupMaster(&c.pl)
			if before.status == "EXISTING" {
				t.Fatalf("tanpa abaikan sudah EXISTING (%s), test tidak menguji apa-apa", before.note)
			}

			eng2 := newCompareEngine()
			eng2.ignores = newIgnoreWords([]mcIgnore{{Word: c.kata}})
			eng2.build(masters, nil)
			after := eng2.lookupMaster(&c.pl)
			if after.status != "EXISTING" {
				t.Fatalf("status = %s (%s), mau EXISTING", after.status, after.note)
			}
			if after.m == nil || after.m.ItemCode != c.wantCode {
				t.Fatalf("kode master salah: %+v, mau %s", after.m, c.wantCode)
			}
		})
	}
}

func TestIgnoreTidakUbahBarangLain(t *testing.T) {
	masters := []mlItem{
		{ID: 1, ItemCode: "A1", ItemDesc: "LIPS (GREENHAM) ------ EST.234 AU"},
		{ID: 2, ItemCode: "A2", ItemDesc: "NECK BONE (GREENHAM) --- EST. 100"},
	}
	eng := newCompareEngine()
	eng.ignores = newIgnoreWords([]mcIgnore{{Word: "PAPILLAE"}})
	eng.build(masters, nil)
	hit := eng.lookupMaster(&plItem{Brand: "GREENHAM", NamaItem: "NECK BONE", Gabungan: "GREENHAM NECK BONE"})
	if hit.status != "EXISTING" || hit.m == nil || hit.m.ItemCode != "A2" {
		t.Fatalf("barang lain berubah: %s %+v (%s)", hit.status, hit.m, hit.note)
	}
}

func TestIgnoreScapulaCartilageTetapCocok(t *testing.T) {
	masters := []mlItem{
		{ID: 1, ItemCode: "17006-10.145BR.03010", ItemDesc: "SCAPULA CARTILAGE (ANGLO) --- 396396/398641 PPI --- EST. BR"},
		{ID: 2, ItemCode: "F1", ItemDesc: "HEART (AAAA) --- EST. 1"},
		{ID: 3, ItemCode: "F2", ItemDesc: "LIVER (BBBB) --- EST. 2"},
		{ID: 4, ItemCode: "F3", ItemDesc: "TONGUE (CCCC) --- EST. 3"},
	}
	eng := newCompareEngine()
	eng.ignores = newIgnoreWords([]mcIgnore{{Word: "CARTILAGE"}})
	eng.build(masters, nil)
	hit := eng.lookupMaster(&plItem{Brand: "ANGLO", NamaItem: "SCAPULA CARTILAGE", Gabungan: "ANGLO SCAPULA CARTILAGE"})
	if hit.status != "EXISTING" || hit.m == nil || hit.m.ItemCode != "17006-10.145BR.03010" {
		t.Fatalf("scapula cartilage jadi rusak: %s %+v (%s)", hit.status, hit.m, hit.note)
	}
}
