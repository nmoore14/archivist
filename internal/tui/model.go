package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	workspaces screen = iota
	workspaceDetail
	serverStatus
)

type Model struct {
	screen     screen
	cursor     int
	workspaces []string
}

func New() Model {
	return Model{workspaces: []string{"Foundations of AI", "Research Methods", "Capstone Studio"}}
}
func (m Model) Init() tea.Cmd { return nil }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.workspaces)-1 {
				m.cursor++
			}
		case "enter":
			m.screen = workspaceDetail
		case "s":
			m.screen = serverStatus
		case "esc":
			m.screen = workspaces
		}
	}
	return m, nil
}
func (m Model) View() string {
	head := "\n  ARCHIVIST  ·  local course intelligence\n  ───────────────────────────────────────\n"
	switch m.screen {
	case workspaceDetail:
		return head + fmt.Sprintf("\n  %s\n\n  Collection detail\n  Documents and members will appear here.\n\n  esc back  ·  q quit\n", m.workspaces[m.cursor])
	case serverStatus:
		return head + "\n  Server status\n\n  Web server     configured\n  SQLite         local\n  Ollama         check /api/tags\n\n  esc back  ·  q quit\n"
	default:
		out := head + "\n  Workspaces\n\n"
		for i, w := range m.workspaces {
			mark := "  "
			if i == m.cursor {
				mark = "› "
			}
			out += fmt.Sprintf("  %s%s\n", mark, w)
		}
		return out + "\n  ↑↓ move  ·  enter open  ·  s status  ·  q quit\n"
	}
}
