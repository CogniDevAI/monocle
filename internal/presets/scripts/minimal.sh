#!/usr/bin/env bash
# monocle preset: minimal — carpeta │ branch
input=$(cat)

cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd // "?"')

branch=""
dirty=""
if git -C "$cwd" rev-parse --git-dir >/dev/null 2>&1; then
  branch=$(git -C "$cwd" symbolic-ref --short HEAD 2>/dev/null \
           || git -C "$cwd" rev-parse --short HEAD 2>/dev/null)
  if ! git -C "$cwd" diff --quiet 2>/dev/null \
      || ! git -C "$cwd" diff --cached --quiet 2>/dev/null; then
    dirty="*"
  fi
fi

RESET='\033[0m'
DIM='\033[2m'
CYAN='\033[36m'
GREEN='\033[32m'
YELLOW='\033[33m'
BOLD='\033[1m'

folder=$(basename "$cwd")
out="${CYAN}${BOLD}${folder}${RESET}"

if [ -n "$branch" ]; then
  if [ -n "$dirty" ]; then
    out="${out}${DIM} │ ${RESET}${YELLOW}${branch}${dirty}${RESET}"
  else
    out="${out}${DIM} │ ${RESET}${GREEN}${branch}${RESET}"
  fi
fi

printf "%b\n" "$out"
