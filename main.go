package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type stagingStatus int

const (
	unstaged stagingStatus = iota
	staged
	partiallyStaged
)

type fileEntry struct {
	pathFromGitRoot string
	pathFromCwd     string
	status          stagingStatus
	diff            diffStat
}

type diffStat struct {
	added   int
	deleted int
}

func (d diffStat) combine(o diffStat) diffStat {
	d.added += o.added
	d.deleted += o.deleted
	return d
}

type model struct {
	files    []fileEntry
	cursor   int
	quitting bool
}

var (
	cursorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	stagedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	partiallyStagedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	unstagedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

var gitRootPath = func() string {
	gitRootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	gitRootBytes, _ := gitRootCmd.Output()
	return strings.TrimSpace(string(gitRootBytes))
}()

var currentPath = func() string {
	cwd, _ := os.Getwd()
	return cwd
}()

func interpretGitStatus(xy string) stagingStatus {
	x, y := xy[0], xy[1]

	switch {
	case x == '?' && y == '?':
		// Cover cases: '??'
		return unstaged
	case x == 'A' && y != ' ':
		// Cover cases: 'AM'
		return partiallyStaged
	case x != ' ' && y != ' ':
		// Cover cases: '*M'
		return partiallyStaged
	case x == 'A':
		// Cover cases: 'A '
		return staged
	case x != ' ':
		// Cover cases: '* '
		return staged
	default:
		// Cover cases: ' *'
		return unstaged
	}
}

func getGitChanges() ([]fileEntry, error) {
	// Check if we are in a git repository
	checkCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	checkOutput, err := checkCmd.Output()
	if err != nil || strings.TrimSpace(string(checkOutput)) != "true" {
		return nil, fmt.Errorf("Not inside a git repository")
	}

	statusCh := make(chan map[string]stagingStatus)
	diffStatsCh := make(chan map[string]diffStat)
	go func() {
		statusCh <- getFileStatus()
	}()
	go func() {
		diffStatsCh <- getFileDiffStats()
	}()
	status := <-statusCh
	diffStats := <-diffStatsCh

	var files []fileEntry
	for path, st := range status {
		files = append(files,
			fileEntry{
				pathFromGitRoot: path,
				pathFromCwd:     getRelPath(path),
				status:          st,
				diff:            diffStats[path],
			})
	}
	return files, nil
}

func getFileStatus() map[string]stagingStatus {
	result := make(map[string]stagingStatus)
	// Get porcelain status
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	for line := range strings.SplitSeq(string(output), "\n") {
		if len(line) < 4 {
			continue
		}
		// The first 2 letters on each line of `git status --porcelain` output represent status
		xy := line[:2]
		pathFromGitRoot := line[3:]
		result[pathFromGitRoot] = interpretGitStatus(xy)
	}
	return result
}

func getFileDiffStats() map[string]diffStat {
	result := make(map[string]diffStat)
	diffCh := make(chan map[string]diffStat)
	const diffCmdNum = 2

	go func() {
		cmd := exec.Command("git", "diff", "--numstat")
		output, err := cmd.Output()
		if err != nil {
			diffCh <- nil
		}
		diffCh <- parseDiffOutput(string(output))
	}()

	go func() {
		cmd := exec.Command("git", "diff", "--numstat", "--cached")
		output, err := cmd.Output()
		if err != nil {
			diffCh <- nil
		}
		diffCh <- parseDiffOutput(string(output))
	}()

	for range diffCmdNum {
		diff := <-diffCh
		for path, d := range diff {
			result[path] = result[path].combine(d)
		}
	}

	return result
}

func parseDiffOutput(output string) map[string]diffStat {
	result := make(map[string]diffStat)
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		pathFromGitRoot := parts[2]
		result[pathFromGitRoot] = diffStat{added, deleted}
	}
	return result
}

// Convert a path from git root to a relative path of cwd
func getRelPath(pathFromGitRoot string) string {
	// many git commands output file path relative to git root
	// So we need to convert it to relative path to CWD
	absPath := filepath.Join(gitRootPath, pathFromGitRoot)
	pathFromCwd, _ := filepath.Rel(currentPath, absPath)
	return pathFromCwd
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			m.cursorUp()
		case "down", "j":
			m.cursorDown()
		case " ":
			m.toggle(m.cursor)
		case "a":
			for i := range len(m.files) {
				m.toggle(i)
			}
		case "tab":
			m.toggle(m.cursor)
			m.cursorDown()
		case "shift+tab":
			m.toggle(m.cursor)
			m.cursorUp()
		}
	}
	return m, nil
}

func (m *model) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *model) cursorDown() {
	if m.cursor < len(m.files)-1 {
		m.cursor++
	}
}

func (m *model) toggle(index int) {
	f := &m.files[index]
	switch f.status {
	case staged:
		exec.Command("git", "restore", "--staged", f.pathFromCwd).Run()
		f.status = unstaged
	case partiallyStaged, unstaged:
		exec.Command("git", "add", f.pathFromCwd).Run()
		f.status = staged
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	maxFilenameLen := 0
	maxAddedLen := 0
	for _, f := range m.files {
		maxFilenameLen = max(maxFilenameLen, len(f.pathFromGitRoot))
		maxAddedLen = max(maxAddedLen, len(strconv.Itoa(f.diff.added)))
	}

	var b strings.Builder
	for i, f := range m.files {
		var cursor string
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
		} else {
			cursor = cursorStyle.Render("  ")
		}
		var checkbox string
		switch f.status {
		case staged:
			checkbox = stagedStyle.Render("[✓]")
		case partiallyStaged:
			checkbox = partiallyStagedStyle.Render("[~]")
		case unstaged:
			checkbox = unstagedStyle.Render("[ ]")
		}
		b.WriteString(fmt.Sprintf(
			"%s%s %s%s %s+%d/-%d\n",
			cursor,
			checkbox,
			f.pathFromGitRoot,
			strings.Repeat(" ", maxFilenameLen-len(f.pathFromGitRoot)),
			strings.Repeat(" ", maxAddedLen-len(strconv.Itoa(f.diff.added))),
			f.diff.added,
			f.diff.deleted,
		))
	}

	b.WriteString("\nj/k/↑/↓: navigate | space: toggle | a: toggle all | q: quit\n")
	return b.String()
}

func main() {
	files, err := getGitChanges()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No changes to stage or unstage.")
		os.Exit(0)
	}

	m := model{files: files}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
