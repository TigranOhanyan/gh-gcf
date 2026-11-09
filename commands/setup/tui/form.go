package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbletea/v2"
)

type form struct {
	field  int
	first  textinput.Model
	second textinput.Model
	two    bool
	notice string
}

func newForm(firstPrompt string, secondPrompt string) (f form, cmd tea.Cmd) {
	f.first = newField(firstPrompt)
	two := secondPrompt != ""
	f.two = two
	if two {
		f.second = newField(secondPrompt)
	}
	f, cmd = f.focus()
	return
}

func newField(prompt string) (field textinput.Model) {
	field = textinput.New()
	field.Prompt = prompt
	return
}

func (f form) active() (field textinput.Model) {
	second := f.field == 1 && f.two
	if second {
		field = f.second
		return
	}
	field = f.first
	return
}

func (f form) setActive(field textinput.Model) (updated form) {
	updated = f
	second := updated.field == 1 && updated.two
	if second {
		updated.second = field
		return
	}
	updated.first = field
	return
}

func (f form) focus() (updated form, cmd tea.Cmd) {
	updated = f
	updated.first.Blur()
	updated.second.Blur()
	field := updated.active()
	cmd = field.Focus()
	updated = updated.setActive(field)
	return
}

func (f form) apply(msg tea.Msg) (updated form, cmd tea.Cmd) {
	updated = f
	field := updated.active()
	field, cmd = field.Update(msg)
	updated = updated.setActive(field)
	_, isPress := msg.(tea.KeyPressMsg)
	if isPress {
		updated.notice = ""
	}
	return
}

func (f form) advance() (updated form, done bool, cmd tea.Cmd) {
	one := !f.two
	last := f.field == 1
	done = one || last
	if done {
		updated = f
		return
	}
	updated = f
	updated.field = 1
	updated.notice = ""
	updated, cmd = updated.focus()
	return
}

func (f form) help() (keys string) {
	one := !f.two
	last := f.field == 1
	saving := one || last
	if saving {
		keys = formSaveKeys
		return
	}
	keys = formNextKeys
	return
}

func (f form) body() (text string) {
	text = f.first.View()
	if f.two {
		text = text + "\n" + f.second.View()
	}
	return
}

func (f form) cursor() (cursor *tea.Cursor) {
	field := f.active()
	cursor = field.Cursor()
	if cursor == nil {
		return
	}
	cursor.Position.Y += 2
	second := f.field == 1 && f.two
	if second {
		cursor.Position.Y++
	}
	return
}

func movesForward(key tea.KeyPressMsg) (forward bool) {
	plain := key.Mod == 0
	enter := key.Code == tea.KeyEnter || key.Code == tea.KeyKpEnter
	tab := key.Code == tea.KeyTab
	forward = plain && (enter || tab)
	return
}
