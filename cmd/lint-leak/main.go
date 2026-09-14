// Command lint-leak enforces INV-1 and INV-6 from docs/SPEC.md.
package main

import (
	"flag"
	"fmt"
	"os"

	"platform9.com/pcd-operator/internal/lintleak"
)

func main() {
	root := flag.String("root", ".", "module root to scan")
	flag.Parse()

	found, err := lintleak.ScanAll(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint-leak: %v\n", err)
		os.Exit(2)
	}
	for _, f := range found {
		fmt.Fprintln(os.Stderr, f.String())
	}
	if len(found) > 0 {
		fmt.Fprintf(os.Stderr, "\nlint-leak: %d violation(s). See docs/SPEC.md §3 INV-1 and INV-6.\n", len(found))
		os.Exit(1)
	}
	fmt.Println("lint-leak: clean")
}
