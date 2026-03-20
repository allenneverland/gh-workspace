package app

import tea "github.com/charmbracelet/bubbletea"

type RepoPathInput struct {
	value  []rune
	cursor int
}

func newRepoPathInput() RepoPathInput {
	return RepoPathInput{}
}

func (s RepoPathInput) Value() string {
	return string(s.value)
}

func (s RepoPathInput) Render() string {
	cursor := s.cursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(s.value) {
		cursor = len(s.value)
	}
	prefix := string(s.value[:cursor])
	suffix := string(s.value[cursor:])
	return prefix + "|" + suffix
}

func (s *RepoPathInput) Update(msg tea.KeyMsg) (submit bool, cancel bool, handled bool) {
	if s == nil {
		return false, false, false
	}

	switch msg.Type {
	case tea.KeyEnter:
		return true, false, true
	case tea.KeyEsc:
		return false, true, true
	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			s.insertRunes(msg.Runes)
		}
		return false, false, true
	case tea.KeyBackspace:
		s.backspace()
		return false, false, true
	case tea.KeyDelete:
		s.delete()
		return false, false, true
	case tea.KeyLeft:
		if s.cursor > 0 {
			s.cursor--
		}
		return false, false, true
	case tea.KeyRight:
		if s.cursor < len(s.value) {
			s.cursor++
		}
		return false, false, true
	case tea.KeyHome:
		s.cursor = 0
		return false, false, true
	case tea.KeyEnd:
		s.cursor = len(s.value)
		return false, false, true
	}

	return false, false, false
}

func (s *RepoPathInput) insertRunes(chars []rune) {
	if len(chars) == 0 {
		return
	}
	cursor := s.cursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(s.value) {
		cursor = len(s.value)
	}

	next := make([]rune, 0, len(s.value)+len(chars))
	next = append(next, s.value[:cursor]...)
	next = append(next, chars...)
	next = append(next, s.value[cursor:]...)
	s.value = next
	s.cursor = cursor + len(chars)
}

func (s *RepoPathInput) backspace() {
	if s.cursor <= 0 || len(s.value) == 0 {
		return
	}
	idx := s.cursor - 1
	s.value = append(s.value[:idx], s.value[s.cursor:]...)
	s.cursor = idx
}

func (s *RepoPathInput) delete() {
	if s.cursor < 0 || s.cursor >= len(s.value) {
		return
	}
	s.value = append(s.value[:s.cursor], s.value[s.cursor+1:]...)
}
