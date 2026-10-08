package main

import (
	"errors"
	"regexp"
	"strings"
)

type mmdBersihRow struct {
	ItemCode  string
	Deskripsi string
	Kategori  string
	NonAktif  string
}

type mmdPotongan struct {
	Utama    string
	Spesifik string
}

var (
	bersihKodeIdx      = 0
	bersihDeskripsiIdx = 1
	bersihNonAktifIdx  = 10
	bersihKategoriIdx  = 14

	mentahIndukIdx  = 0
	mentahAnakIdx   = 1
	mentahCucukIdx  = 2
	mentahUtamaIdx  = 4
	mentahAnakDesc  = 5
	mentahCucukDesc = 6
)

func bersihkan(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func idxHeaderMencari(rows [][]cellVal) int {
	for i := 0; i < len(rows) && i < 10; i++ {
		if bersihkan(cellText(rows[i], 0)) != "" {
			return i
		}
	}
	return -1
}

var (
	rePotonganKurung = regexp.MustCompile(`\([^()]*\)|\[[^\[\]]*\]`)

	rePotonganPisah = regexp.MustCompile(`\s*-{2,}\s*|\s+-\s+`)

	rePotonganEst = regexp.MustCompile(`^(EST|ME)\b[\s.]*`)

	rePotonganEstInline = regexp.MustCompile(`\s+(EST|ME)\b[\s.]*[0-9]`)
)

func catatanEst(seg string) bool {
	return rePotonganEst.MatchString(strings.TrimSpace(seg))
}

func bersihkanPotonganSpesifik(s string) string {
	s = bersihkan(s)
	if s == "" {
		return ""
	}
	s = rePotonganKurung.ReplaceAllString(s, " ")

	parts := []string{}
	for _, seg := range rePotonganPisah.Split(s, -1) {
		seg = bersihkan(seg)
		if seg == "" || catatanEst(seg) {
			continue
		}

		if rePotonganEstInline.MatchString(seg) {
			seg = bersihkan(rePotonganEstInline.Split(seg, 2)[0])
		}
		if seg == "" {
			continue
		}
		parts = append(parts, seg)
	}
	return bersihkan(strings.Join(parts, " --- "))
}

func parseMMDBersih(data []byte) ([]mmdBersihRow, error) {
	sheets, _, err := openSheets(data)
	if err != nil {
		return nil, err
	}
	if len(sheets) == 0 {
		return nil, errors.New("File tidak punya sheet.")
	}
	rows := sheets[0].Rows
	h := idxHeaderMencari(rows)
	if h < 0 || len(rows) < h+2 {
		return nil, errors.New("Tidak ada baris data di file.")
	}

	kode, deskripsi, nonAktif, kategori := bersihKodeIdx, bersihDeskripsiIdx, bersihNonAktifIdx, bersihKategoriIdx
	found := false
	for ci := 0; ci < len(rows[h]); ci++ {
		v := strings.ToLower(bersihkan(cellText(rows[h], ci)))
		if v == "" {
			continue
		}
		if strings.Contains(v, "no") && strings.Contains(v, "barang") {
			kode = ci
			found = true
		}
		if strings.Contains(v, "deskripsi") {
			deskripsi = ci
			found = true
		}
		if strings.Contains(v, "non") && strings.Contains(v, "aktif") {
			nonAktif = ci
			found = true
		}
		if strings.Contains(v, "kategori") {
			kategori = ci
			found = true
		}
	}
	_ = found

	get := func(r []cellVal, i int) string {
		if i < 0 || i >= len(r) {
			return ""
		}
		return bersihkan(cellText(r, i))
	}

	out := []mmdBersihRow{}
	for _, r := range rows[h+1:] {
		ic := get(r, kode)
		if ic == "" {
			continue
		}
		out = append(out, mmdBersihRow{
			ItemCode:  ic,
			Deskripsi: get(r, deskripsi),
			Kategori:  strings.ToUpper(get(r, kategori)),
			NonAktif:  get(r, nonAktif),
		})
	}
	if len(out) == 0 {
		return nil, errors.New("Tidak ada item di file bersih.")
	}
	return out, nil
}

func parseMMDMentah(data []byte) (map[string]mmdPotongan, error) {
	out := map[string]mmdPotongan{}
	sheets, _, err := openSheets(data)
	if err != nil {
		return out, err
	}
	if len(sheets) == 0 {
		return out, errors.New("File mentah tidak punya sheet.")
	}
	rows := sheets[0].Rows
	h := idxHeaderMencari(rows)
	if h < 0 {
		return out, nil
	}

	get := func(r []cellVal, i int) string {
		if i < 0 || i >= len(r) {
			return ""
		}
		return bersihkan(cellText(r, i))
	}

	var indukTerakhir string

	for _, r := range rows[h+1:] {

		if e := get(r, mentahUtamaIdx); e != "" {
			indukTerakhir = e
		}
		spes := get(r, mentahCucukDesc)
		if spes == "" {
			spes = get(r, mentahAnakDesc)
		}
		spes = bersihkanPotonganSpesifik(spes)

		kodeCucuk := get(r, mentahCucukIdx)
		kodeAnak := get(r, mentahAnakIdx)
		kodeInduk := get(r, mentahIndukIdx)

		p := mmdPotongan{Utama: indukTerakhir, Spesifik: spes}
		for _, k := range []string{kodeCucuk, kodeAnak} {
			if k == "" {
				continue
			}
			if p.Spesifik != "" || p.Utama != "" {
				out[k] = p
			}
		}

		if kodeInduk != "" && indukTerakhir != "" {
			if _, ada := out[kodeInduk]; !ada {
				out[kodeInduk] = mmdPotongan{Utama: indukTerakhir, Spesifik: get(r, mentahAnakDesc)}
			}
		}
	}
	return out, nil
}
