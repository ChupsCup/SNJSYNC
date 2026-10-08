package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDebugBuildAndDiag(t *testing.T) {
	app := loadRealApp(t)

	t1 := time.Now()
	si := buildSearchEntries(app.searchSource())
	fmt.Printf("buildSearchEntries: %d entri, %d ms\n", len(si.entries), time.Since(t1).Milliseconds())

	t2 := time.Now()
	app.search("SLICE")
	fmt.Printf("panggilan search pertama (index sudah ada): %d ms\n", time.Since(t2).Milliseconds())

	n := 0
	for _, e := range si.entries {
		d := strings.ToLower(e.e.Dir)
		if strings.Contains(d, "tongue") || strings.Contains(d, "969") {
			n++
			if n <= 12 {
				fmt.Printf("TONGUE/969: %s\n", e.e.Dir)
			}
		}
	}
	fmt.Printf("total folder TONGUE/969: %d\n", n)

	target := ""
	for _, e := range si.entries {
		if leafOf(e.e.Dir) == "1. BOX PUTIH" {
			target = e.e.Dir
			break
		}
	}
	fmt.Printf("target dir = %q\n", target)
	for _, e := range si.entries {
		if e.e.Dir != target {
			continue
		}
		fmt.Printf("  text=%.200q\n", e.e.Text)
		fmt.Printf("  wSrc[slice]=%v hayContains=%v codes=%v units=%v\n",
			e.wSrc["slice"], strings.Contains(e.hay, "slice"), e.codeW, e.cutUnits)
		fmt.Printf("  cutInfer(slice)=%v fuzzyCands=%v\n", e.cutInfer("slice"), fuzzyCands(si.fz, "slice"))
		break
	}
}
