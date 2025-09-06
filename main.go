package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StagingStatus int

const (
	Unstaged StagingStatus = iota
	Staged
	PartiallyStaged
)

type fileEntry struct {
	name   string
	status StagingStatus
	// Command to stage, or re-stage after unstaging
	stageCmd *exec.Cmd
	// Command to unstage, or re-unstage after staging
	unstageCmd *exec.Cmd
}

type model struct {
	files    []fileEntry
	cursor   int
	quitting bool
}

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	stagedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	unstagedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func interpretGitStatus(xy string, filename string) (StagingStatus, *exec.Cmd, *exec.Cmd) {
	x, y := xy[0], xy[1]

	switch {
	case x == '?' && y == '?':
		// Cover cases: '??'
		return Unstaged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
	case x == 'A' && y != ' ':
		// Cover cases: 'AM'
		return PartiallyStaged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
	case x != ' ' && y != ' ':
		// Cover cases: '*M'
		return PartiallyStaged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
	case x == 'A':
		// Cover cases: 'A '
		return Staged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
	case x != ' ':
		// Cover cases: '* '
		return Staged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
	default:
		// Cover cases: ' *'
		return Unstaged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
	}
}

func getGitChanges() ([]fileEntry, error) {
	// Check if we are in a git repository
	checkCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	checkOutput, err := checkCmd.Output()
	if err != nil || strings.TrimSpace(string(checkOutput)) != "true" {
		return nil, fmt.Errorf("Not inside a git repository")
	}

	// Get porcelain status
	statusCmd := exec.Command("git", "status", "--porcelain")
	out, err := statusCmd.Output()
	if err != nil {
		return nil, err
	}

	// Get the git root
	gitRootCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	gitRootBytes, _ := gitRootCmd.Output()
	gitRoot := strings.TrimSpace(string(gitRootBytes))
	// Get cwd
	cwd, _ := os.Getwd()

	var files []fileEntry
	for line := range strings.SplitSeq(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		// The first 2 letters on each line of `git status --porcelain` output represent status
		xy := line[:2]
		// `git status --porcelain` always output file path relative to git root
		filepathFromGitRoot := strings.TrimSpace(line[3:])
		// So we need to convert it to relative path to CWD
		absPath := filepath.Join(gitRoot, filepathFromGitRoot)
		relPath, _ := filepath.Rel(cwd, absPath)
		status, stageCmd, unstageCmd := interpretGitStatus(xy, relPath)
		files = append(files, fileEntry{name: filepathFromGitRoot, status: status, stageCmd: stageCmd, unstageCmd: unstageCmd})
	}
	return files, nil
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
	case Staged:
		f.unstageCmd.Run()
		f.status = Unstaged
	case PartiallyStaged, Unstaged:
		f.stageCmd.Run()
		f.status = Staged
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
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
		case Staged:
			checkbox = stagedStyle.Render("[✓]")
		case PartiallyStaged:
			checkbox = unstagedStyle.Render("[~]")
		case Unstaged:
			checkbox = unstagedStyle.Render("[ ]")
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checkbox, f.name))
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
