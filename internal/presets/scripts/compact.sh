#!/usr/bin/env bash
# monocle preset: compact — carpeta │ branch │ modelo │ XX%
input=$(cat)

cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd // "?"')
model=$(echo "$input" | jq -r '.model.display_name // "?"')
used_pct=$(echo "$input" | jq -r '.context_window.used_percentage // empty')

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
RED='\033[31m'
MAGENTA='\033[35m'
BOLD='\033[1m'

folder=$(basename "$cwd")

pct_seg=""
if [ -n "$used_pct" ]; then
  used_int=$(printf '%.0f' "$used_pct")
  if [ "$used_int" -ge 80 ]; then
    pct_seg="${RED}${used_int}%${RESET}"
  elif [ "$used_int" -ge 50 ]; then
    pct_seg="${YELLOW}${used_int}%${RESET}"
  else
    pct_seg="${GREEN}${used_int}%${RESET}"
  fi
fi

git_seg=""
if [ -n "$branch" ]; then
  if [ -n "$dirty" ]; then
    git_seg="${YELLOW}${branch}${dirty}${RESET}"
  else
    git_seg="${GREEN}${branch}${RESET}"
  fi
fi

sep="${DIM} │ ${RESET}"
out="${CYAN}${BOLD}${folder}${RESET}"
[ -n "$git_seg" ] && out="${out}${sep}${git_seg}"
out="${out}${sep}${MAGENTA}${model}${RESET}"
[ -n "$pct_seg" ] && out="${out}${sep}${pct_seg}"

printf "%b\n" "$out"
