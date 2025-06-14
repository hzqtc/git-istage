# git-stage

An interactive command-line utility for quickly staging and unstaging files in a Git repository.

![](https://raw.github.com/hzqtc/git-istage/master/demo.gif)

---

## ✨ Features

- Navigate modified files with arrow keys
- Toggle staged/unstaged files with spacebar

---

## 📦 Installation

Make sure you have [Go](https://golang.org/dl/) installed.

```sh
make install
```

This will build and install the git-stage binary to ~/.local/bin.
Ensure that ~/.local/bin is in your $PATH.

Add a git alias 'git d' that would be used to show diff.
[`difftasic`](https://github.com/Wilfred/difftastic) is recommended.
For example, add the following lines to `~/.gitconfig`:

```gitconfig
[diff]
  external = difft
  tool = difftastic
[difftool]
  prompt = false
[difftool "difftastic"]
  cmd = difft --display side-by-side-show-both --color always "$MERGED" "$LOCAL" "abcdef1" "100644" "$REMOTE" "abcdef2" "100644"
[alias]
  d = difftool
```

## 🚀 Usage

Inside a Git repository:

```sh
git istage
```

- ↑/↓ – navigate files
- space – stage/unstage selected file
- d - view diff (both staged/unstaged change vs HEAD)
  - ↑/↓ - scroll by 1 line
  - PgUp/PgDown - scroll by half screen
  - g - scroll to top
  - G - scroll to bottom
- q or Ctrl+C – quit

