package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbletea/v2"
)

type form struct {
	field  int
	fields []textinput.Model
	notice string
}

func newForm(prompts ...string) (f form, cmd tea.Cmd) {
	f.fields = make([]textinput.Model, 0, len(prompts))
	for _, prompt := range prompts {
		field := newField(prompt)
		f.fields = append(f.fields, field)
	}
	f, cmd = f.focus()
	return
}

func newField(prompt string) (field textinput.Model) {
	field = textinput.New()
	field.Prompt = prompt
	return
}

func (f form) setValue(index int, text string) (updated form) {
	updated = f
	updated.fields[index].SetValue(text)
	return
}

func (f form) value(index int) (text string) {
	text = f.fields[index].Value()
	return
}

func (f form) focus() (updated form, cmd tea.Cmd) {
	updated = f
	for i := range updated.fields {
		updated.fields[i].Blur()
	}
	index := updated.field
	field := updated.fields[index]
	cmd = field.Focus()
	updated.fields[index] = field
	return
}

func (f form) apply(msg tea.Msg) (updated form, cmd tea.Cmd) {
	updated = f
	index := updated.field
	field := updated.fields[index]
	field, cmd = field.Update(msg)
	updated.fields[index] = field
	_, isPress := msg.(tea.KeyPressMsg)
	if isPress {
		updated.notice = ""
	}
	return
}

func (f form) advance() (updated form, done bool, cmd tea.Cmd) {
	last := f.field >= len(f.fields)-1
	done = last
	if done {
		updated = f
		return
	}
	updated = f
	updated.field++
	updated.notice = ""
	updated, cmd = updated.focus()
	return
}

func (f form) help() (hints []keyHint) {
	last := f.field >= len(f.fields)-1
	if last {
		hints = formSaveKeys
		return
	}
	hints = formNextKeys
	return
}

func (f form) body() (text string) {
	for i, field := range f.fields {
		line := field.View()
		first := i == 0
		if first {
			text = line
			continue
		}
		text = text + "\n" + line
	}
	return
}

func (f form) cursor() (cursor *tea.Cursor) {
	field := f.fields[f.field]
	cursor = field.Cursor()
	if cursor == nil {
		return
	}
	cursor.Position.Y += 2 + f.field
	return
}

func movesForward(key tea.KeyPressMsg) (forward bool) {
	plain := key.Mod == 0
	enter := key.Code == tea.KeyEnter || key.Code == tea.KeyKpEnter
	tab := key.Code == tea.KeyTab
	forward = plain && (enter || tab)
	return
}
