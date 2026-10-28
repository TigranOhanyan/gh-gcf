package main

import (
	"context"
	"os"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/charmbracelet/fang"
	"charm.land/log/v2"
)

func main() {
	options := log.Options{
		Level: log.InfoLevel,
	}
	logger := log.NewWithOptions(os.Stderr, options)
	log.SetDefault(logger)

	ctx := context.Background()
	root := commands.NewRoot()
	err := fang.Execute(ctx, root)
	if err != nil {
		os.Exit(1)
	}
}
