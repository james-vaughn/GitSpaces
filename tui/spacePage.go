package tui

import (
	"fmt"
	"slices"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/james-vaughn/GitSpaces/internal/git"
	"github.com/james-vaughn/GitSpaces/internal/space"
)

const (
	titleRows  = 2
	footerRows = 2
)

type repoResult struct {
	Status string
	Behind bool
	Diff   string
}

type SpacePage struct {
	Name     string
	SpaceDir string
	Table    table.Model
	Repos    []string
	Results  map[string]repoResult

	Confirming   bool
	Pending      string
	Updating     bool
	Checking     bool
	PendingScans int
	Err          error
}

type repoScanDoneMsg struct {
	Repo   string
	Result repoResult
	Merged bool
}

func newSpacePage(spaceDir, name string, repos []string) *SpacePage {
	columns := []table.Column{
		{Title: "Repository", Width: 30},
		{Title: "Diff vs main/master", Width: 20},
		{Title: "Status", Width: 14},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)

	styles := table.DefaultStyles()
	styles.Header = headerStyle
	t.SetStyles(styles)

	p := &SpacePage{
		Name:         name,
		SpaceDir:     spaceDir,
		Table:        t,
		Repos:        repos,
		Results:      map[string]repoResult{},
		Checking:     len(repos) > 0,
		PendingScans: len(repos),
	}
	p.rebuildRows()
	return p
}

func (p *SpacePage) rebuildRows() {
	rows := make([]table.Row, len(p.Repos))
	for i, r := range p.Repos {
		res, scanned := p.Results[r]

		name := r
		diff := res.Diff
		if !scanned {
			diff = "…"
		}
		status := ""

		switch {
		case res.Status != "":
			name = errStyle.Render(name)
			diff = errStyle.Render(diff)
			status = errStyle.Render(res.Status)
		case res.Behind:
			status = warnStyle.Render("needs update")
		case scanned:
			status = okStyle.Render("Up-to-date")
		}

		rows[i] = table.Row{name, diff, status}
	}
	p.Table.SetRows(rows)
}

func (p *SpacePage) issueCount() int {
	n := 0
	for _, res := range p.Results {
		if res.Status != "" {
			n++
		}
	}
	return n
}

func (p *SpacePage) selectedRepo() string {
	i := p.Table.Cursor()
	if i < 0 || i >= len(p.Repos) {
		return ""
	}
	return p.Repos[i]
}

func (p *SpacePage) deleteRepo(repo string) error {
	if err := space.RemoveRepo(p.SpaceDir, repo); err != nil {
		return err
	}
	for i, r := range p.Repos {
		if r == repo {
			p.Repos = append(p.Repos[:i], p.Repos[i+1:]...)
			break
		}
	}
	delete(p.Results, repo)
	p.rebuildRows()
	return nil
}

func scanCmd(spaceDir, branch, repo string, merge bool) tea.Cmd {
	return func() tea.Msg {
		return repoScanDoneMsg{
			Repo:   repo,
			Result: scanRepo(space.RepoPath(spaceDir, repo), branch, merge),
			Merged: merge,
		}
	}
}

func scanAllCmd(spaceDir, branch string, repos []string, merge bool) tea.Cmd {
	cmds := make([]tea.Cmd, len(repos))
	for i, r := range repos {
		cmds[i] = scanCmd(spaceDir, branch, r, merge)
	}
	return tea.Batch(cmds...)
}

func scanRepo(repoPath, branch string, merge bool) repoResult {
	base := git.BaseBranch(repoPath)
	if base == "" {
		return repoResult{Status: "no base branch", Diff: "—"}
	}

	var res repoResult
	if err := git.Fetch(repoPath); err != nil {
		res.Status = "fetch error"
	} else if merge {
		if err := git.Merge(repoPath, "origin/"+base); err != nil {
			res.Status = "merge error"
		}
	}

	res.Behind = git.Behind(repoPath, base)

	added, deleted, err := git.DiffStat(repoPath, base, branch)
	if err != nil {
		if res.Status == "" {
			res.Status = "diff error"
		}
		res.Diff = "—"
		return res
	}

	res.Diff = fmt.Sprintf("+%d -%d", added, deleted)
	return res
}

func (p *SpacePage) Init() tea.Cmd {
	return scanAllCmd(p.SpaceDir, p.Name, p.Repos, false)
}

func (p *SpacePage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		p.Table.SetWidth(msg.Width - h)
		p.Table.SetHeight(msg.Height - v - titleRows - footerRows)
		return p, nil

	case repoScanDoneMsg:
		if p.PendingScans > 0 {
			p.PendingScans--
		}
		if p.PendingScans == 0 {
			if msg.Merged {
				p.Updating = false
			} else {
				p.Checking = false
			}
		}
		if !slices.Contains(p.Repos, msg.Repo) {
			return p, nil
		}
		p.Results[msg.Repo] = msg.Result
		p.rebuildRows()
		return p, nil

	case tea.KeyPressMsg:
		if p.Updating {
			return p, nil
		}

		if p.Confirming {
			switch msg.String() {
			case "y", "Y":
				p.Err = p.deleteRepo(p.Pending)
				p.Confirming = false
				p.Pending = ""
			case "n", "N", "esc":
				p.Confirming = false
				p.Pending = ""
			}
			return p, nil
		}

		switch msg.String() {
		case "esc":
			return p, tea.Quit
		case "left", "h":
			return p, popPage
		case "u":
			if p.Checking || len(p.Repos) == 0 {
				return p, nil
			}
			p.Updating = true
			p.PendingScans = len(p.Repos)
			p.Err = nil
			return p, scanAllCmd(p.SpaceDir, p.Name, p.Repos, true)
		case "backspace", "delete":
			if repo := p.selectedRepo(); repo != "" {
				p.Confirming = true
				p.Pending = repo
				p.Err = nil
			}
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.Table, cmd = p.Table.Update(msg)
	return p, cmd
}

func (p *SpacePage) View() string {
	title := spaceTitleStyle.Render("Space — " + p.Name)

	var footer string
	switch {
	case p.Updating:
		footer = promptStyle.Render("Updating repositories…")
	case p.Checking:
		footer = hintStyle.Render("Checking for updates…")
	case p.Confirming:
		footer = confirmStyle.Render(fmt.Sprintf("Delete %q? This permanently removes the folder.  y / n", p.Pending))
	case p.Err != nil:
		footer = errStyle.Render(p.Err.Error())
	case p.issueCount() > 0:
		footer = errStyle.Render(fmt.Sprintf("%d repo(s) reported issues — see Status column", p.issueCount()))
	default:
		footer = hintStyle.Render("u: fetch+merge main · backspace/delete: remove repo · left: back")
	}

	return docStyle.Render(title + "\n" + p.Table.View() + "\n\n" + footer)
}
