package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type pageMode int

const (
	modeBrowse pageMode = iota
	modeNaming
	modeCloning
)

type focusArea int

const (
	focusRepos focusArea = iota
	focusSpaces
)

const (
	spacesFolder = "Spaces"

	reposPaneNumerator   = 3
	reposPaneDenominator = 5
	listPaneGap          = 1
)

type repoItem struct {
	Name  string
	IsDir bool
}

func (i repoItem) FilterValue() string { return i.Name }

type spaceItem struct {
	Name string
}

func (i spaceItem) FilterValue() string { return i.Name }

type cloneDoneMsg struct {
	Name string
	Err  error
}

type spaceOption struct {
	Name     string
	Disabled bool
}

type repoDelegate struct {
	Selected map[string]bool
}

func (d repoDelegate) Height() int                         { return 1 }
func (d repoDelegate) Spacing() int                        { return 0 }
func (d repoDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d repoDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(repoItem)
	if !ok {
		return
	}

	cursor := "  "
	if index == m.Index() {
		cursor = cursorStyle.Render("> ")
	}

	check := "   "
	if it.IsDir {
		if d.Selected[it.Name] {
			check = "[x]"
		} else {
			check = "[ ]"
		}
	}

	name := it.Name
	if !it.IsDir {
		name = fileStyle.Render(name)
	} else if d.Selected[it.Name] {
		name = selectedStyle.Render(name)
	}

	fmt.Fprintf(w, "%s%s %s", cursor, check, name)
}

type spaceDelegate struct{}

func (spaceDelegate) Height() int                         { return 1 }
func (spaceDelegate) Spacing() int                        { return 0 }
func (spaceDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (spaceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(spaceItem)
	if !ok {
		return
	}

	cursor := "  "
	if index == m.Index() {
		cursor = cursorStyle.Render("> ")
	}

	fmt.Fprintf(w, "%s%s", cursor, it.Name)
}

type RepositoryPage struct {
	List        list.Model
	Spaces      list.Model
	Selected    map[string]bool
	Dir         string
	Focus       focusArea
	Mode        pageMode
	Input       textinput.Model
	Existing    []spaceOption
	NameOnInput bool
	NameIdx     int
	Err         error
}

func newRepositoryPage() *RepositoryPage {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Git")

	selected := map[string]bool{}

	repos := list.New(readDirItems(dir), repoDelegate{Selected: selected}, 0, 0)
	repos.SetShowStatusBar(true)
	repos.SetFilteringEnabled(true)
	repos.Styles.Title = headerStyle
	repos.DisableQuitKeybindings()
	repos.AdditionalShortHelpKeys = repoHelpKeys
	repos.AdditionalFullHelpKeys = repoHelpKeys

	spaces := list.New(readSpaceItems(dir), spaceDelegate{}, 0, 0)
	spaces.SetShowStatusBar(true)
	spaces.SetFilteringEnabled(true)
	spaces.Styles.Title = headerStyle
	spaces.DisableQuitKeybindings()
	spaces.AdditionalShortHelpKeys = spaceHelpKeys
	spaces.AdditionalFullHelpKeys = spaceHelpKeys

	ti := textinput.New()
	ti.Placeholder = "YCHS-1234"
	ti.Prompt = ""

	p := &RepositoryPage{
		List:     repos,
		Spaces:   spaces,
		Selected: selected,
		Dir:      dir,
		Focus:    focusRepos,
		Mode:     modeBrowse,
		Input:    ti,
	}
	p.updateTitles()
	return p
}

func repoHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "new space")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	}
}

func spaceHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open space")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	}
}

func (p *RepositoryPage) updateTitles() {
	repoTitle := "Repositories — " + p.Dir
	spaceTitle := "Spaces"
	if p.Focus == focusRepos {
		repoTitle = "▸ " + repoTitle
	} else {
		spaceTitle = "▸ " + spaceTitle
	}
	p.List.Title = repoTitle
	p.Spaces.Title = spaceTitle
}

func readDirItems(dir string) []list.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	items := make([]list.Item, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && e.Name() == spacesFolder {
			continue
		}
		items = append(items, repoItem{Name: e.Name(), IsDir: e.IsDir()})
	}
	return items
}

func readSpaceItems(root string) []list.Item {
	entries, err := os.ReadDir(filepath.Join(root, spacesFolder))
	if err != nil {
		return nil
	}

	var items []list.Item
	for _, e := range entries {
		if e.IsDir() {
			items = append(items, spaceItem{Name: e.Name()})
		}
	}
	return items
}

func readSpaceRepos(spaceDir string) []string {
	entries, err := os.ReadDir(spaceDir)
	if err != nil {
		return nil
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func (p *RepositoryPage) selectedNames() []string {
	var names []string
	for _, item := range p.List.Items() {
		if it, ok := item.(repoItem); ok && p.Selected[it.Name] {
			names = append(names, it.Name)
		}
	}
	return names
}

func (p *RepositoryPage) spaceOptions() []spaceOption {
	sel := p.selectedNames()
	var opts []spaceOption
	for _, item := range readSpaceItems(p.Dir) {
		name := item.(spaceItem).Name
		opts = append(opts, spaceOption{Name: name, Disabled: allCloned(p.Dir, name, sel)})
	}
	return opts
}

func allCloned(root, space string, repos []string) bool {
	if len(repos) == 0 {
		return false
	}
	dir := filepath.Join(root, spacesFolder, space)
	for _, r := range repos {
		if _, err := os.Stat(filepath.Join(dir, r)); err != nil {
			return false
		}
	}
	return true
}

func firstEnabled(opts []spaceOption) int {
	for i, o := range opts {
		if !o.Disabled {
			return i
		}
	}
	return 0
}

func (p *RepositoryPage) nameMove(delta int) {
	n := len(p.Existing)
	if n == 0 {
		return
	}
	i := p.NameIdx
	for step := 0; step < n; step++ {
		i = (i + delta + n) % n
		if !p.Existing[i].Disabled {
			p.NameIdx = i
			return
		}
	}
}

func cloneCmd(root, name string, repos []string) tea.Cmd {
	return func() tea.Msg {
		spaceDir := filepath.Join(root, spacesFolder, name)
		if err := os.MkdirAll(spaceDir, 0o755); err != nil {
			return cloneDoneMsg{Name: name, Err: err}
		}

		for _, r := range repos {
			src := filepath.Join(root, r)
			dst := filepath.Join(spaceDir, r)

			if _, err := os.Stat(dst); err == nil {
				continue
			}

			clone := exec.Command("git", "clone", src, dst)
			if out, err := clone.CombinedOutput(); err != nil {
				return cloneDoneMsg{Name: name, Err: fmt.Errorf("clone %s: %w: %s", r, err, strings.TrimSpace(string(out)))}
			}

			checkout := exec.Command("git", "-C", dst, "checkout", "-B", name)
			if out, err := checkout.CombinedOutput(); err != nil {
				return cloneDoneMsg{Name: name, Err: fmt.Errorf("checkout %s in %s: %w: %s", name, r, err, strings.TrimSpace(string(out)))}
			}
		}

		return cloneDoneMsg{Name: name}
	}
}

func (p *RepositoryPage) Init() tea.Cmd { return nil }

func (p *RepositoryPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		w := msg.Width - h
		total := msg.Height - v
		reposH := total * reposPaneNumerator / reposPaneDenominator
		spacesH := total - reposH - listPaneGap
		p.List.SetSize(w, reposH)
		p.Spaces.SetSize(w, spacesH)
		return p, nil

	case cloneDoneMsg:
		p.Mode = modeBrowse
		if msg.Err != nil {
			p.Err = msg.Err
			return p, nil
		}
		spaceDir := filepath.Join(p.Dir, spacesFolder, msg.Name)
		refresh := p.Spaces.SetItems(readSpaceItems(p.Dir))
		return p, tea.Batch(refresh, pushPage(newSpacePage(spaceDir, msg.Name, readSpaceRepos(spaceDir))))

	case tea.KeyPressMsg:
		switch p.Mode {
		case modeCloning:
			return p, nil

		case modeNaming:
			switch msg.String() {
			case "esc":
				p.Mode = modeBrowse
				p.Input.Blur()
				return p, nil
			case "tab":
				if len(p.Existing) > 0 {
					p.NameOnInput = !p.NameOnInput
					if p.NameOnInput {
						return p, p.Input.Focus()
					}
					p.Input.Blur()
				}
				return p, nil
			case "up":
				if !p.NameOnInput {
					p.nameMove(-1)
					return p, nil
				}
			case "down":
				if !p.NameOnInput {
					p.nameMove(1)
					return p, nil
				}
			case "enter":
				name := ""
				if p.NameOnInput {
					name = strings.TrimSpace(p.Input.Value())
				} else if p.NameIdx >= 0 && p.NameIdx < len(p.Existing) {
					if opt := p.Existing[p.NameIdx]; !opt.Disabled {
						name = opt.Name
					}
				}
				if name == "" {
					return p, nil
				}
				p.Mode = modeCloning
				p.Err = nil
				p.Input.Blur()
				return p, cloneCmd(p.Dir, name, p.selectedNames())
			}
			if p.NameOnInput {
				var cmd tea.Cmd
				p.Input, cmd = p.Input.Update(msg)
				return p, cmd
			}
			return p, nil

		default:
			focused := &p.List
			if p.Focus == focusSpaces {
				focused = &p.Spaces
			}
			if focused.FilterState() == list.Filtering {
				break
			}

			switch msg.String() {
			case "tab":
				if p.Focus == focusRepos {
					p.Focus = focusSpaces
				} else {
					p.Focus = focusRepos
				}
				p.updateTitles()
				return p, nil

			case "space", " ":
				if p.Focus == focusRepos {
					if it, ok := p.List.SelectedItem().(repoItem); ok && it.IsDir {
						p.Selected[it.Name] = !p.Selected[it.Name]
					}
					return p, nil
				}

			case "enter":
				if p.Focus == focusRepos {
					if len(p.selectedNames()) == 0 {
						return p, nil
					}
					p.Mode = modeNaming
					p.Err = nil
					p.Input.Reset()
					p.Existing = p.spaceOptions()
					p.NameOnInput = true
					p.NameIdx = firstEnabled(p.Existing)
					return p, p.Input.Focus()
				}
				if it, ok := p.Spaces.SelectedItem().(spaceItem); ok {
					spaceDir := filepath.Join(p.Dir, spacesFolder, it.Name)
					return p, pushPage(newSpacePage(spaceDir, it.Name, readSpaceRepos(spaceDir)))
				}
				return p, nil
			}
		}
	}

	if p.Mode == modeBrowse {
		var cmd tea.Cmd
		if p.Focus == focusSpaces {
			p.Spaces, cmd = p.Spaces.Update(msg)
		} else {
			p.List, cmd = p.List.Update(msg)
		}
		return p, cmd
	}

	return p, nil
}

func (p *RepositoryPage) View() string {
	switch p.Mode {
	case modeNaming:
		inputCursor := "  "
		if p.NameOnInput {
			inputCursor = cursorStyle.Render("> ")
		}
		body := promptStyle.Render("Name a new space:") + "\n\n" +
			inputCursor + p.Input.View() + "\n"

		if len(p.Existing) > 0 {
			body += "\n" + promptStyle.Render("…or clone into an existing space:") + "\n"
			for i, opt := range p.Existing {
				cursor := "  "
				if !p.NameOnInput && i == p.NameIdx {
					cursor = cursorStyle.Render("> ")
				}
				label := opt.Name
				if opt.Disabled {
					label = fileStyle.Render(opt.Name + "  (already cloned)")
				}
				body += cursor + label + "\n"
			}
		}

		body += "\n" + fileStyle.Render("tab: switch · ↑/↓: choose · enter: clone · esc: cancel")
		if p.Err != nil {
			body += "\n\n" + errStyle.Render(p.Err.Error())
		}
		return docStyle.Render(body)

	case modeCloning:
		name := strings.TrimSpace(p.Input.Value())
		return docStyle.Render(promptStyle.Render("Cloning repositories into " + name + "…"))

	default:
		view := p.List.View() + "\n" + p.Spaces.View()
		if p.Err != nil {
			view += "\n" + errStyle.Render(p.Err.Error())
		}
		return docStyle.Render(view)
	}
}
