package main

import "testing"

func TestAliasRulesApply(t *testing.T) {
	ar := newAliasRules([]mcAlias{{A: "heart trimmings", B: "heart TRMG"}})
	cases := []struct{ in, want string }{
		{"ANGLO HEART TRMG", "ANGLO HEART TRIMMINGS"},
		{"ANGLO HEART TRMG.", "ANGLO HEART TRIMMINGS."},
		{"ANGLO HEART-TRMG", "ANGLO HEART TRIMMINGS"},
		{"ANGLO HEARTTRMG", "ANGLO HEART TRIMMINGS"},
		{"ANGLO HEART  TRMG", "ANGLO HEART TRIMMINGS"},
		{"ANGLO HEART TRIMMINGS", "ANGLO HEART TRIMMINGS"},
		{"HEART TRMGX", "HEART TRMGX"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ar.apply(c.in); got != c.want {
			t.Fatalf("apply(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	var nilRules *aliasRules
	if got := nilRules.apply("HEART TRMG"); got != "HEART TRMG" {
		t.Fatalf("nil rules berubah: %q", got)
	}
	if got := newAliasRules(nil).apply("A B"); got != "A B" {
		t.Fatalf("kosong berubah: %q", got)
	}
}

func TestAliasRulesTerbalik(t *testing.T) {
	ar := newAliasRules([]mcAlias{{A: "heart trmg", B: "heart trimmings"}})
	if got := ar.apply("ANGLO HEART TRIMMINGS"); got != "ANGLO HEART TRMG" {
		t.Fatalf("arah terbalik: %q", got)
	}
}

func TestAliasCompareMatch(t *testing.T) {
	masters := []mlItem{
		{ItemCode: "101", Brand: "ANGLO", NamaItem: "ANGLO HEART TRMG"},
		{ItemCode: "102", Brand: "ANGLO", NamaItem: "ANGLO NECK BONE"},
	}
	pl := plItem{Brand: "ANGLO", NamaItem: "ANGLO HEART TRIMMINGS"}

	eng := newCompareEngine()
	eng.aliases = newAliasRules([]mcAlias{{A: "heart trimmings", B: "heart trmg"}})
	eng.build(masters, nil)

	for _, e := range eng.entries {
		if e.m.ItemCode != "101" {
			continue
		}
		if !e.wordsSet.Has("trimmings") || e.wordsSet.Has("trmg") {
			t.Fatalf("master belum di-alias: %v", e.wordsSet.List())
		}
	}

	hit := eng.lookupMaster(&pl)
	if hit.status != "EXISTING" {
		t.Fatalf("status = %s (%s), want EXISTING", hit.status, hit.note)
	}
	if hit.m == nil || hit.m.ItemCode != "101" {
		t.Fatalf("kode master salah: %+v", hit.m)
	}

	other := plItem{Brand: "ANGLO", NamaItem: "ANGLO NECK BONE"}
	if hit := eng.lookupMaster(&other); hit.m != nil && hit.m.ItemCode == "101" {
		t.Fatalf("alias mengganggu item lain: %+v", hit.m)
	}
}
