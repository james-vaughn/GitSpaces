package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/james-vaughn/GitSpaces/internal/git"
	"github.com/james-vaughn/GitSpaces/internal/space"
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
	reposPaneNumerator   = 3
	reposPaneDenominator = 5
	listPaneGap          = 2
	minSideBySideWidth   = 80
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

type repoCloneDoneMsg struct {
	Repo string
	Gen  int
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
	Confirming  bool
	Pending     string
	SideBySide  bool
	CloneName   string
	CloneRepos  []string
	CloneDone   map[string]error
	CloneGen    int
	CloneCancel context.CancelFunc
	Err         error
}

func newRepositoryPage(dir string) *RepositoryPage {
	selected := map[string]bool{}

	repos := list.New(repoItems(dir), repoDelegate{Selected: selected}, 0, 0)
	repos.SetShowStatusBar(true)
	repos.SetFilteringEnabled(true)
	repos.Styles.Title = headerStyle
	repos.DisableQuitKeybindings()
	repos.AdditionalShortHelpKeys = repoHelpKeys
	repos.AdditionalFullHelpKeys = repoHelpKeys

	spaces := list.New(spaceItems(dir), spaceDelegate{}, 0, 0)
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
		key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "delete space")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	}
}

func (p *RepositoryPage) updateTitles() {
	repoTitle := "Repositories — " + filepath.Base(p.Dir)
	spaceTitle := "Spaces"
	if p.Focus == focusRepos {
		repoTitle = "▸ " + repoTitle
	} else {
		spaceTitle = "▸ " + spaceTitle
	}
	p.List.Title = repoTitle
	p.Spaces.Title = spaceTitle
}

func repoItems(root string) []list.Item {
	names := space.Repos(root)
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = repoItem{Name: n, IsDir: true}
	}
	return items
}

func spaceItems(root string) []list.Item {
	names := space.Names(root)
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = spaceItem{Name: n}
	}
	return items
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
	for _, name := range space.Names(p.Dir) {
		disabled := len(sel) > 0 && space.ContainsRepos(p.Dir, name, sel)
		opts = append(opts, spaceOption{Name: name, Disabled: disabled})
	}
	return opts
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

func cloneCmd(ctx context.Context, gen int, root, name, repo string) tea.Cmd {
	return func() tea.Msg {
		spaceDir := space.Path(root, name)
		dst := space.RepoPath(spaceDir, repo)

		done := func(err error) tea.Msg {
			return repoCloneDoneMsg{Repo: repo, Gen: gen, Err: err}
		}

		abandon := func() tea.Msg {
			space.RemoveRepo(spaceDir, repo)
			return done(nil)
		}

		if space.Exists(dst) {
			return done(nil)
		}

		url, err := git.RemoteURL(filepath.Join(root, repo))
		if err != nil {
			return done(fmt.Errorf("remote for %s: %w", repo, err))
		}

		if err := git.Clone(ctx, url, dst); err != nil {
			if ctx.Err() != nil {
				return abandon()
			}
			return done(fmt.Errorf("clone %s: %w", repo, err))
		}

		if ctx.Err() != nil {
			return abandon()
		}

		start := ""
		if base := git.BaseBranch(dst); base != "" {
			start = "origin/" + base
		}

		if err := git.Checkout(dst, name, start); err != nil {
			return done(fmt.Errorf("checkout %s in %s: %w", name, repo, err))
		}

		if err := git.SetUpstream(dst, name); err != nil {
			return done(fmt.Errorf("track %s in %s: %w", name, repo, err))
		}

		return done(nil)
	}
}

func (p *RepositoryPage) stopClone() {
	p.CloneGen++
	if p.CloneCancel != nil {
		p.CloneCancel()
		p.CloneCancel = nil
	}
}

func (p *RepositoryPage) cloneErrors() []error {
	var errs []error
	for _, r := range p.CloneRepos {
		if err := p.CloneDone[r]; err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (p *RepositoryPage) Init() tea.Cmd { return nil }

func (p *RepositoryPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		w := msg.Width - h
		total := msg.Height - v
		p.SideBySide = w >= minSideBySideWidth
		if p.SideBySide {
			reposW := (w - listPaneGap) * reposPaneNumerator / reposPaneDenominator
			p.List.SetSize(reposW, total)
			p.Spaces.SetSize(w-listPaneGap-reposW, total)
			return p, nil
		}
		reposH := total * reposPaneNumerator / reposPaneDenominator
		p.List.SetSize(w, reposH)
		p.Spaces.SetSize(w, total-reposH-listPaneGap)
		return p, nil

	case repoCloneDoneMsg:
		if msg.Gen != p.CloneGen {
			return p, nil
		}

		p.CloneDone[msg.Repo] = msg.Err
		if len(p.CloneDone) < len(p.CloneRepos) {
			return p, nil
		}

		p.stopClone()
		p.Mode = modeBrowse
		refresh := p.Spaces.SetItems(spaceItems(p.Dir))
		if errs := p.cloneErrors(); len(errs) > 0 {
			p.Err = errors.Join(errs...)
			return p, refresh
		}

		spaceDir := space.Path(p.Dir, p.CloneName)
		return p, tea.Batch(refresh, pushPage(newSpacePage(spaceDir, p.CloneName, space.ReposIn(spaceDir))))

	case tea.KeyPressMsg:
		switch p.Mode {
		case modeCloning:
			if msg.String() == "esc" {
				p.stopClone()
				p.Mode = modeBrowse
				p.Err = nil
				return p, p.Spaces.SetItems(spaceItems(p.Dir))
			}
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
				if _, err := space.Create(p.Dir, name); err != nil {
					p.Err = err
					return p, nil
				}

				p.Mode = modeCloning
				p.Err = nil
				p.Input.Blur()
				p.CloneName = name
				p.CloneRepos = p.selectedNames()
				p.CloneDone = map[string]error{}
				p.CloneGen++

				ctx, cancel := context.WithCancel(context.Background())
				p.CloneCancel = cancel

				cmds := make([]tea.Cmd, len(p.CloneRepos))
				for i, r := range p.CloneRepos {
					cmds[i] = cloneCmd(ctx, p.CloneGen, p.Dir, name, r)
				}
				return p, tea.Batch(cmds...)
			}
			if p.NameOnInput {
				var cmd tea.Cmd
				p.Input, cmd = p.Input.Update(msg)
				return p, cmd
			}
			return p, nil

		default:
			if p.Confirming {
				switch msg.String() {
				case "y", "Y":
					if err := space.Delete(p.Dir, p.Pending); err != nil {
						p.Err = err
					}
					p.Confirming = false
					p.Pending = ""
					return p, p.Spaces.SetItems(spaceItems(p.Dir))
				case "n", "N", "esc":
					p.Confirming = false
					p.Pending = ""
				}
				return p, nil
			}

			focused := &p.List
			if p.Focus == focusSpaces {
				focused = &p.Spaces
			}
			if focused.FilterState() == list.Filtering {
				break
			}

			switch msg.String() {
			case "esc":
				if focused.FilterState() == list.Unfiltered {
					return p, tea.Quit
				}

			case "backspace", "delete":
				if p.Focus == focusSpaces {
					if it, ok := p.Spaces.SelectedItem().(spaceItem); ok {
						p.Confirming = true
						p.Pending = it.Name
						p.Err = nil
					}
					return p, nil
				}

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
					spaceDir := space.Path(p.Dir, it.Name)
					return p, pushPage(newSpacePage(spaceDir, it.Name, space.ReposIn(spaceDir)))
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
		body := promptStyle.Render("Cloning repositories into "+p.CloneName+"…") + "\n\n"
		for _, r := range p.CloneRepos {
			err, done := p.CloneDone[r]
			switch {
			case !done:
				body += hintStyle.Render("  … "+r) + "\n"
			case err != nil:
				body += errStyle.Render("  ✗ "+r) + "\n"
			default:
				body += okStyle.Render("  ✓ "+r) + "\n"
			}
		}
		body += "\n" + hintStyle.Render("esc: cancel")
		return docStyle.Render(body)

	default:
		view := p.List.View() + "\n" + p.Spaces.View()
		if p.SideBySide {
			view = lipgloss.JoinHorizontal(
				lipgloss.Top,
				p.List.View(),
				strings.Repeat(" ", listPaneGap),
				p.Spaces.View(),
			)
		}
		if p.Confirming {
			view += "\n" + confirmStyle.Render(fmt.Sprintf("Delete space %q? This permanently removes the folder and all its repos.  y / n", p.Pending))
		} else if p.Err != nil {
			view += "\n" + errStyle.Render(p.Err.Error())
		}
		return docStyle.Render(view)
	}
}
