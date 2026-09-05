package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/james-vaughn/GitSpaces/tui"
)

func main() {
	defaultRoot := ""
	if home, err := os.UserHomeDir(); err == nil {
		defaultRoot = filepath.Join(home, "Git")
	}

	root := flag.String("r", defaultRoot, "root directory containing git repositories")
	flag.Parse()

	if err := tui.Run(*root); err != nil {
		log.Fatalf("Failed to run %v", err)
	}
}
