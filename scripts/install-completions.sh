#!/bin/sh
set -eu

command -v hidrive >/dev/null 2>&1 || {
  echo "hidrive not found in PATH" >&2
  exit 1
}

install_bash() {
  dir="${HOME}/.local/share/bash-completion/completions"
  mkdir -p "$dir"
  hidrive completion bash > "${dir}/hidrive"
  echo "bash completion installed to ${dir}/hidrive"
}

install_zsh() {
  dir="${HOME}/.zsh/completions"
  mkdir -p "$dir"
  hidrive completion zsh > "${dir}/_hidrive"
  echo "zsh completion installed to ${dir}/_hidrive"
  echo "Add to ~/.zshrc if needed: fpath=(${dir} \$fpath); autoload -U compinit && compinit"
}

install_fish() {
  dir="${HOME}/.config/fish/completions"
  mkdir -p "$dir"
  hidrive completion fish > "${dir}/hidrive.fish"
  echo "fish completion installed to ${dir}/hidrive.fish"
}

install_bash
install_zsh
install_fish
