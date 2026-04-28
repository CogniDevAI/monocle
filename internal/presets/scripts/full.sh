#!/usr/bin/env bash
# monocle preset: full — carpeta │ branch │ modelo │ ████░░░░░░ XX% │ ↑in ↓out
input=$(cat)

cwd=$(echo "$input"        | jq -r '.workspace.current_dir // .cwd // "?"')
model=$(echo "$input"      | jq -r '.model.display_name // "?"')
used_pct=$(echo "$input"   | jq -r '.context_window.used_percentage // empty')
total_in=$(echo "$input"   | jq -r '.context_window.total_input_tokens // 0')
total_out=$(echo "$input"  | jq -r '.context_window.total_output_tokens // 0')

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
YELLOW='\033[33m'
GREEN='\033[32m'
RED='\033[31m'
MAGENTA='\033[35m'
BOLD='\033[1m'

folder=$(basename "$cwd")

bar=""
if [ -n "$used_pct" ]; then
  used_int=$(printf '%.0f' "$used_pct")
  filled=$(( used_int / 10 ))
  empty=$(( 10 - filled ))
  bar_fill=$(python3 -c "print('█'*$filled, end='')" 2>/dev/null || echo "")
  bar_empty=$(python3 -c "print('░'*$empty, end='')" 2>/dev/null || echo "")
  if [ "$used_int" -ge 80 ]; then
    bar_color="$RED"
  elif [ "$used_int" -ge 50 ]; then
    bar_color="$YELLOW"
  else
    bar_color="$GREEN"
  fi
  bar="${bar_color}${bar_fill}${DIM}${bar_empty}${RESET} ${bar_color}${used_int}%${RESET}"
fi

tokens=""
if [ "$total_in" -gt 0 ] || [ "$total_out" -gt 0 ]; then
  fmt_k() {
    local n=$1
    if [ "$n" -ge 1000 ]; then
      printf '%.1fk' "$(echo "scale=1; $n/1000" | bc)"
    else
      echo "$n"
    fi
  }
  in_fmt=$(fmt_k "$total_in")
  out_fmt=$(fmt_k "$total_out")
  tokens="${DIM}↑${in_fmt} ↓${out_fmt}${RESET}"
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
[ -n "$bar" ]     && out="${out}${sep}${bar}"
[ -n "$tokens" ]  && out="${out}${sep}${tokens}"

printf "%b\n" "$out"
