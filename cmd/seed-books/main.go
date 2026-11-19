package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"depop-store-manager/internal/store"
)

func main() {
	csvPath := flag.String("csv", "", "path to books CSV (Depop sheet export)")
	catalogPath := flag.String("catalog", "data/catalog.json", "path to catalog.json")
	threshold := flag.Float64("threshold", store.BooksMatchThreshold, "min match score to apply")
	dryRun := flag.Bool("dry-run", false, "match + write temp copy only; leave real catalog untouched")
	flag.Parse()
	if *csvPath == "" {
		fmt.Fprintln(os.Stderr, "usage: seed-books -csv path/to/books.csv [-catalog data/catalog.json] [-dry-run]")
		os.Exit(2)
	}

	f, err := os.Open(*csvPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	rows, err := store.ReadBooksCSV(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("books rows: %d\n", len(rows))

	target := *catalogPath
	if *dryRun {
		tmp, err := os.CreateTemp("", "catalog-dry-*.json")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		target = tmp.Name()
		tmp.Close()
		defer os.Remove(target)
		if err := copyFile(*catalogPath, target); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("dry-run catalog copy: %s\n", target)
	}

	cs := store.NewCatalogStore(target)
	res, err := cs.ApplyBooksSeed(rows, *threshold)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("matched=%d applied=%d skipped=%d ambiguous=%d\n",
		res.Matched, res.Applied, res.Skipped, len(res.Ambiguous))
	if len(res.Weak) > 0 {
		n := len(res.Weak)
		if n > 15 {
			n = 15
		}
		fmt.Printf("skipped (showing %d/%d):\n", n, len(res.Weak))
		for _, w := range res.Weak[:n] {
			fmt.Println(" ", w)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
