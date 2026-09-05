package main

import (
	"log"

	"github.com/james-vaughn/GitSpaces/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		log.Fatalf("Failed to run %v", err)
	}
}
