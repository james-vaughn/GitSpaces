package tui

import (
	tea "charm.land/bubbletea/v2"
)

type Page interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Page, tea.Cmd)
	View() string
}

type pushPageMsg struct{ Page Page }

type popPageMsg struct{}

func pushPage(p Page) tea.Cmd {
	return func() tea.Msg { return pushPageMsg{Page: p} }
}

func popPage() tea.Msg { return popPageMsg{} }

type Model struct {
	Stack  []Page
	Width  int
	Height int
}

func Run() error {
	root := &Model{
		Stack: []Page{newRepositoryPage()},
	}

	p := tea.NewProgram(root)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func (m *Model) top() Page {
	if len(m.Stack) == 0 {
		return nil
	}
	return m.Stack[len(m.Stack)-1]
}

func (m *Model) Init() tea.Cmd {
	if top := m.top(); top != nil {
		return top.Init()
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case pushPageMsg:
		m.Stack = append(m.Stack, msg.Page)
		cmds := []tea.Cmd{msg.Page.Init()}
		idx := len(m.Stack) - 1
		page, cmd := m.Stack[idx].Update(tea.WindowSizeMsg{Width: m.Width, Height: m.Height})
		m.Stack[idx] = page
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case popPageMsg:
		if len(m.Stack) > 1 {
			m.Stack = m.Stack[:len(m.Stack)-1]
			return m, nil
		}
		return m, tea.Quit
	}

	if idx := len(m.Stack) - 1; idx >= 0 {
		page, cmd := m.Stack[idx].Update(msg)
		m.Stack[idx] = page
		return m, cmd
	}

	return m, nil
}

func (m *Model) View() tea.View {
	var s string
	if top := m.top(); top != nil {
		s = top.View()
	}
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}
