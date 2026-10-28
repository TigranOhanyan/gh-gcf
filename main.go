package main

import (
	"context"
	"os"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/charmbracelet/fang"
)

func main() {
	ctx := context.Background()
	root := commands.NewRoot()
	err := fang.Execute(ctx, root)
	if err != nil {
		os.Exit(1)
	}
}
