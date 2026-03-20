package app

import tea "github.com/charmbracelet/bubbletea"

type Tab string

const (
	TabOverview Tab = "overview"
)

type Config struct{}

type Model struct {
	ActiveTab       Tab
	LeftPaneWidth   int
	CenterPaneWidth int
	RightPaneWidth  int
}

func NewModel(_ Config) Model {
	return Model{
		ActiveTab:       TabOverview,
		LeftPaneWidth:   30,
		CenterPaneWidth: 80,
		RightPaneWidth:  40,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	return "workspace gitops release tui"
}
