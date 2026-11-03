package main

import (
	"context"
	"os"

	"charm.land/log/v2"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/charmbracelet/fang"
)

func main() {
	options := log.Options{
		Level: log.InfoLevel,
	}
	logger := log.NewWithOptions(os.Stderr, options)
	log.SetDefault(logger)

	ctx := context.Background()
	rootCmd := root.NewRoot()
	err := fang.Execute(ctx, rootCmd)
	if err != nil {
		os.Exit(1)
	}
}
