package config

import (
	"context"

	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

// Setup records config in the factory checkout at dir.
// It fails when dir is not a git checkout. It does not commit.
func Setup(ctx context.Context, dir string) (err error) {
	err = commands.RequireCheckout(ctx, dir)
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
		return
	}
	log.Info("setup", "path", dir)

	initial := newModel(ctx, dir)
	option := tea.WithContext(ctx)
	program := tea.NewProgram(initial, option)
	final, err := program.Run()
	finished, isModel := final.(model)
	if isModel {
		finished.closeDB()
	}
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
	}
	return
}
