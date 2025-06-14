package main

import (
  "fmt"
  "os"
  "os/exec"
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
  name        string
  status      StagingStatus
  // Command to stage, or re-stage after unstaging
  stageCmd    *exec.Cmd
  // Command to unstage, or re-unstage after staging
  unstageCmd  *exec.Cmd
}

type model struct {
  files           []fileEntry
  cursor          int
  quitting        bool
  diffMode        bool
  diffContent     string
  scrollOffset    int
  viewportHeight  int
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
    return Unstaged, exec.Command("git", "add", filename), exec.Command("git", "rm", "--cached", filename)
  case x == 'A' && y != ' ':
    // Cover cases: 'AM'
    return PartiallyStaged, exec.Command("git", "add", filename), exec.Command("git", "rm", "--cached", filename)
  case x != ' ' && y != ' ':
    // Cover cases: '*M'
    return PartiallyStaged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
  case x == 'A':
    // Cover cases: 'A '
    return Staged, exec.Command("git", "add", filename), exec.Command("git", "rm", "--cached", filename)
  case x != ' ':
    // Cover cases: '* '
    return Staged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
  default:
    // Cover cases: ' *'
    return Unstaged, exec.Command("git", "add", filename), exec.Command("git", "restore", "--staged", filename)
  }
}

func getDiff(f *fileEntry) string {
  var diffCmd *exec.Cmd
  switch f.status {
  case Staged:
    // Diff between staged vs HEAD
    diffCmd =  exec.Command("git", "d", "--staged", f.name)
  case Unstaged:
    // Diff between unstaged vs HEAD
    diffCmd =  exec.Command("git", "d", f.name)
  case PartiallyStaged:
    // Diff between working dir vs HEAD
    diffCmd =  exec.Command("git", "d", "HEAD", f.name)
  }
  out, err := diffCmd.CombinedOutput()
  if err != nil {
    return fmt.Sprintf("Failed to show diff: %v", err)
  }
  return string(out)
}

func getGitChanges() ([]fileEntry, error) {
  // Check if we are in a git repository
  checkCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
  checkOutput, err := checkCmd.Output()
  if err != nil || strings.TrimSpace(string(checkOutput)) != "true" {
    return nil, fmt.Errorf("Not inside a git repository")
  }

  // Get porcelain status
  cmd := exec.Command("git", "status", "--porcelain")
  out, err := cmd.Output()
  if err != nil {
    return nil, err
  }

  var files []fileEntry
  lines := strings.Split(string(out), "\n")
  for _, line := range lines {
    if len(line) < 4 {
      continue
    }
    // The first 2 letters on each line of `git status --porcelain` output represent status
    status := line[:2]
    filename := strings.TrimSpace(line[3:])
    stagingStatus, stageCmd, unstageCmd := interpretGitStatus(status, filename)
    files = append(files, fileEntry{name: filename, status: stagingStatus, stageCmd: stageCmd, unstageCmd: unstageCmd})
  }
  return files, nil
}

func getMaxScroll(m *model) int {
  lines := len(strings.Split(m.diffContent, "\n"))
  if lines > m.viewportHeight {
    return lines - m.viewportHeight
  } else {
    return 0
  }
}

func moveCursorUp(m *model) {
  if m.cursor > 0 {
    m.cursor--
    m.scrollOffset = 0
  }
}

func moveCursorDown(m *model) {
  if m.cursor < len(m.files) - 1 {
    m.cursor++
    m.scrollOffset = 0
  }
}

func (m model) Init() tea.Cmd {
  return tea.EnterAltScreen
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  switch msg := msg.(type) {
  case tea.WindowSizeMsg:
    m.viewportHeight = msg.Height - len(m.files) - 1
    return m, nil
  case tea.KeyMsg:
    switch msg.String() {
    case "ctrl+c", "q":
      m.quitting = true
      return m, tea.Quit
    case "up":
      if m.diffMode {
        if m.scrollOffset > 0 {
          m.scrollOffset--
        } else {
          // Go to previous file if at top of scroll
          moveCursorUp(&m)
        }
      } else {
        moveCursorUp(&m)
      }
    case "down":
      if m.diffMode {
        if m.scrollOffset < getMaxScroll(&m) {
          m.scrollOffset++
        } else {
          // Go to next file if at top of scroll
          moveCursorDown(&m)
        }
      } else {
        moveCursorDown(&m)
      }
    case "pgdown":
      if m.diffMode {
        maxScroll := getMaxScroll(&m)
        newScrollOffset := m.scrollOffset + m.viewportHeight / 2
        if newScrollOffset > maxScroll {
          newScrollOffset = maxScroll
        }
        m.scrollOffset = newScrollOffset
      }
    case "pgup":
      if m.diffMode {
        newScrollOffset := m.scrollOffset - m.viewportHeight / 2
        if newScrollOffset < 0 {
          newScrollOffset = 0
        }
        m.scrollOffset = newScrollOffset
      }
    case " ":
      if m.diffMode {
        break;
      }
      f := &m.files[m.cursor]
      switch f.status {
      case Staged:
        f.unstageCmd.Run()
        f.status = Unstaged
      case PartiallyStaged, Unstaged:
        f.stageCmd.Run()
        f.status = Staged
      }
    case "d":
      m.diffMode = !m.diffMode
    case "g":
      if m.diffMode {
        m.scrollOffset = 0
      }
    case "G":
      if m.diffMode {
        m.scrollOffset = getMaxScroll(&m)
      }
    }
    if m.diffMode {
      m.diffContent = getDiff(&m.files[m.cursor])
    }
  }
  return m, nil
}

func (m model) View() string {
  if m.quitting {
    return ""
  }

  var b strings.Builder

  if m.diffMode {
    lines := strings.Split(m.diffContent, "\n")

    // Slice visible lines
    end := m.scrollOffset + m.viewportHeight
    if end > len(lines) {
      end = len(lines)
    }
    visibleLines := lines[m.scrollOffset:end]

    b.WriteString(strings.Join(visibleLines, "\n"))
    b.WriteString(fmt.Sprintf(
      "\n↑/↓/PageUp/PageDown scroll (%d/%d)  g: top  G: bottom  d: back  q: quit\n",
      end,
      len(lines),
    ))
    return b.String()
  }

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

  b.WriteString("\n↑/↓: navigate  space: toggle  d: diff  q: quit\n")
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
    return
  }

  m := model{files: files}
  p := tea.NewProgram(m)
  if _, err := p.Run(); err != nil {
    fmt.Println("Error running program:", err)
    os.Exit(1)
  }
}
