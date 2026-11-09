package tui

import (
	"strconv"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbletea/v2"
)

type action int

const (
	actionNone action = iota
	actionQuit
	actionAdd
	actionOpen
	actionBack
	actionMove
)

type row struct {
	title string
	value string
}

func (item row) Title() (title string) {
	title = item.title
	return
}

func (item row) Description() (description string) {
	return
}

func (item row) FilterValue() (value string) {
	value = item.title
	return
}

func listAction(key tea.KeyPressMsg) (kind action) {
	stroke := key.String()
	switch stroke {
	case "q", "ctrl+c":
		kind = actionQuit
	case "a":
		kind = actionAdd
	case "enter":
		kind = actionOpen
	case "esc":
		kind = actionBack
	case "up", "down", "k", "j":
		kind = actionMove
	default:
		kind = actionNone
	}
	return
}

func newChooser(title string, items []string, values []string, selectValue string) (chooser list.Model) {
	rows := make([]list.Item, 0, len(items))
	for i, item := range items {
		value := item
		hasValue := i < len(values)
		if hasValue {
			value = values[i]
		}
		next := row{title: item, value: value}
		rows = append(rows, next)
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	chooser = list.New(rows, delegate, 60, 16)
	chooser.Title = title
	chooser.SetShowTitle(false)
	chooser.SetFilteringEnabled(false)
	chooser.SetShowStatusBar(false)
	chooser.SetShowHelp(false)
	chooser.SetShowPagination(false)
	chooser.SetShowFilter(false)
	hasRows := len(rows) > 0
	if hasRows {
		chooser.Select(0)
	}
	index := indexOfValue(values, selectValue)
	found := index >= 0
	if found {
		chooser.Select(index)
	}
	return
}

func listBody(chooser list.Model, empty string) (text string) {
	none := len(chooser.Items()) == 0
	if none {
		text = empty
		return
	}
	text = chooser.View()
	return
}

func selectedRow(chooser list.Model) (item row, ok bool) {
	selected := chooser.SelectedItem()
	if selected == nil {
		return
	}
	item, ok = selected.(row)
	return
}

func indexOfValue(values []string, want string) (index int) {
	index = -1
	blank := want == ""
	if blank {
		return
	}
	for i, value := range values {
		same := value == want
		if same {
			index = i
			return
		}
	}
	return
}

func idLines(ids []int64) (lines []string) {
	lines = make([]string, 0, len(ids))
	for _, id := range ids {
		text := strconv.FormatInt(id, 10)
		lines = append(lines, text)
	}
	return
}
