package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
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

	Confirming bool
	Pending    string
	Updating   bool
	Checking   bool
	Err        error
}

type scanDoneMsg struct {
	Results map[string]repoResult
	Merged  bool
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
		Name:     name,
		SpaceDir: spaceDir,
		Table:    t,
		Repos:    repos,
		Results:  map[string]repoResult{},
		Checking: true,
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
	if err := os.RemoveAll(filepath.Join(p.SpaceDir, repo)); err != nil {
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

func scanCmd(spaceDir, branch string, repos []string, merge bool) tea.Cmd {
	return func() tea.Msg {
		results := make(map[string]repoResult, len(repos))
		for _, r := range repos {
			results[r] = scanRepo(filepath.Join(spaceDir, r), branch, merge)
		}
		return scanDoneMsg{Results: results, Merged: merge}
	}
}

func scanRepo(repoPath, branch string, merge bool) repoResult {
	base := baseBranch(repoPath)
	if base == "" {
		return repoResult{Status: "no base branch", Diff: "—"}
	}

	var res repoResult
	if err := exec.Command("git", "-C", repoPath, "fetch", "origin").Run(); err != nil {
		res.Status = "fetch error"
	} else if merge {
		if err := exec.Command("git", "-C", repoPath, "merge", "origin/"+base).Run(); err != nil {
			res.Status = "merge error"
		}
	}

	res.Behind = isBehind(repoPath, base)
	res.Diff = diffStat(repoPath, base, branch)
	return res
}

func baseBranch(repoPath string) string {
	for _, b := range []string{"main", "master"} {
		if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", b).Run() == nil {
			return b
		}
	}
	return ""
}

func isBehind(repoPath, base string) bool {
	out, err := exec.Command("git", "-C", repoPath, "rev-list", "--count", "HEAD..origin/"+base).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != "0"
}

func diffStat(repoPath, base, branch string) string {
	out, err := exec.Command("git", "-C", repoPath, "diff", "--numstat", base, branch).Output()
	if err != nil {
		return "—"
	}

	added, deleted := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		a, _ := strconv.Atoi(fields[0])
		d, _ := strconv.Atoi(fields[1])
		added += a
		deleted += d
	}

	return fmt.Sprintf("+%d -%d", added, deleted)
}

func (p *SpacePage) Init() tea.Cmd {
	return scanCmd(p.SpaceDir, p.Name, p.Repos, false)
}

func (p *SpacePage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		p.Table.SetWidth(msg.Width - h)
		p.Table.SetHeight(msg.Height - v - titleRows - footerRows)
		return p, nil

	case scanDoneMsg:
		if msg.Merged {
			p.Updating = false
		} else {
			p.Checking = false
		}
		p.Results = msg.Results
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
			if len(p.Repos) > 0 {
				p.Updating = true
				p.Err = nil
				return p, scanCmd(p.SpaceDir, p.Name, p.Repos, true)
			}
			return p, nil
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
