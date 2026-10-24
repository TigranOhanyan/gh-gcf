package main

import (
	"fmt"
	"os"

	"github.com/TigranOhanyan/gh-gcf/internal/cli"
	"github.com/TigranOhanyan/gh-gcf/internal/source"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gh gcf: %s\n", err)
		os.Exit(1)
	}
	if err := cli.Run(os.Args[1:], cwd, os.Stdout, os.Stderr, source.GitHub{}); err != nil {
		fmt.Fprintf(os.Stderr, "gh gcf: %s\n", err)
		os.Exit(1)
	}
}
