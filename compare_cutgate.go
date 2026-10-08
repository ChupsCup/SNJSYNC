package main

import "strings"

func specTokensToText(s *strSet) string {
	if s == nil {
		return ""
	}
	var out []string
	for _, t := range s.List() {
		if t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

func (e *compareEngine) cutGate(r *plItem, be *masterEntry, plNama string,
	plSpec *strSet, plEsts []string) *lookupHit {

	if be == nil {
		return nil
	}

	mlNama := be.m.NamaItem
	if mlNama == "" {
		mlNama = cutNameFromDesc(be.m.ItemDesc)
	}

	plCode := ""
	if c := normKode(r.ItemCode); c != "" {
		plCode = c
	} else if c := normKode(r.Gabungan); c != "" {
		plCode = c
	}
	mlCode := normKode(be.m.ItemCode)
	if mlCode == "" {
		mlCode = normKode(be.m.NoContainer)
	}

	mlSpec := ""
	if be.m.ItemDesc != "" {
		if ks := cutSpecKeys(be.m.ItemDesc); len(ks) > 0 {
			mlSpec = strings.Join(dedupStrings(ks), " ")
		}
	}

	v := cutVerdictAdvanced(plNama, specTokensToText(plSpec), mlNama, mlSpec, plCode, mlCode)

	konkret := v.CodeConflict || v.SameKey || v.ScopeFact || len(v.SpecDiff) > 0
	if !konkret {
		return nil
	}
	if v.Outcome == outMatch && !v.SameKey {
		return nil
	}

	alasan := "Aturan kamus: " + strings.Join(v.Reason, "; ")
	if len(plEsts) > 0 && len(be.estList) > 0 {
		if strings.Join(plEsts, ",") != strings.Join(be.estList, ",") {
			alasan += "; EST beda: " + strings.Join(plEsts, ",") + " vs " + strings.Join(be.estList, ",")
		}
	}
	short := trimTo(mlNama, 40)
	code := ""
	if be.m.ItemCode != "" {
		code = be.m.ItemCode
	}

	switch v.Outcome {
	case outNotMatch:

		return &lookupHit{status: "NEW", sym: 0, cand: be.m,
			note: "Bukan pasangan yang sama — " + alasan +
				". Kandidat terdekat: " + short + " [" + code + "]"}

	case outReview:
		return &lookupHit{status: "NEW", review: true, cand: be.m,
			note: "Perlu dicek — " + alasan +
				". Kandidat terdekat: " + short + " [" + code + "]"}

	default:

		return nil
	}
}

func trimTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func init() {
}
