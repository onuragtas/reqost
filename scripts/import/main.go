// Command import parses a Postman collection.json and loads it into the local
// reqost SQLite index — the same pipeline the app's ImportCollection uses.
// Handy for populating the index from the CLI. Usage: go run ./scripts/import [path]
package main

import (
	"fmt"
	"os"

	"reqost/internal/collection"
	"reqost/internal/index"
)

func main() {
	path := "collection-50k.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	items, err := collection.ParseFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	db, err := index.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open index: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat: %v\n", err)
		os.Exit(1)
	}
	if err := db.ImportItems(path, info.ModTime().Unix(), items); err != nil {
		fmt.Fprintf(os.Stderr, "import: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("imported %d items from %s into the index\n", len(items), path)
}
