package main

var cutDistinctGroup = map[string][]string{

	"Loin - short vs striploin": {
		"SHORTLOIN",
		"STRIPLOIN",
	},

	"Brisket bone - grade E dan P": {
		"BRISKETBONE",
		"BRISKETBONE-E",
		"BRISKETBONE-P",
	},
}

func cutDistinct(a, b string) bool {
	if a == "" || b == "" || a == b {
		return false
	}
	for _, group := range cutDistinctGroup {
		hasA, hasB := false, false
		for _, k := range group {
			if k == a {
				hasA = true
			}
			if k == b {
				hasB = true
			}
		}
		if hasA && hasB {
			return true
		}
	}
	return false
}

const cutSpecBerbedaProduk = true
