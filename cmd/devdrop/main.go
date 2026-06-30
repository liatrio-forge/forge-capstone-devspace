package main

import (
	"fmt"
	"os"

	"github.com/HexSleeves/devdrop/internal/devdrop"
)

var version = "dev"

func main() {
	if err := devdrop.NewRootCommand(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
