package tui

import (
	"context"
	"fmt"
	"io"

	"charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type sessionScreen interface {
	tea.Model
	release()
}

func Run(ctx context.Context, dir string) (err error) {
	err = commands.RequireCheckout(ctx, dir)
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
		return
	}
	initial := newHome(ctx, dir)
	option := tea.WithContext(ctx)
	program := tea.NewProgram(initial, option)
	previous := log.Default()
	quiet := log.New(io.Discard)
	log.SetDefault(quiet)
	defer log.SetDefault(previous)
	final, err := program.Run()
	log.SetDefault(previous)
	finished, ok := final.(sessionScreen)
	if ok {
		finished.release()
	}
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
	}
	return
}

func openRepo(ctx context.Context, dir string, owner string, name string) (db *configdb.DB, missing bool, err error) {
	path := config.ConfigPath(dir, owner, name)
	db, missing, err = config.OpenConfig(ctx, path)
	return
}

func closeDB(db *configdb.DB) {
	if db == nil {
		return
	}
	config.CloseConfig(db)
}

func appSecret(id int64) (name string) {
	name = fmt.Sprintf("GITHUB_APP_%d", id)
	return
}
