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

## 🚀 Usage

Inside a Git repository:

```sh
git istage
```

- ↑/↓ – navigate files
- space – stage/unstage selected file
- q or Ctrl+C – quit

