package presets

import (
	"fmt"
	"strings"
)

// Segment describe un componente activable del statusline custom.
//
// Cada segment aporta:
//   - jqExtract: línea(s) bash que extraen variables del JSON de stdin (puede
//     estar vacío si el segment usa variables ya extraídas por otro segment
//     que se materializan globalmente).
//   - prep: bloque opcional que prepara variables intermedias (ej. detectar
//     branch, calcular barra de contexto, formatear tokens).
//   - render: la expresión bash que produce el fragmento coloreado final;
//     se asigna a una variable seg_<id> y se concatena con separadores.
//
// El script generado por BuildCustomScript siempre incluye el shebang, la
// lectura de stdin, paleta ANSI compartida, y compone la línea con
// separadores `│` dimmed en el orden en que aparecen los IDs activos.
type Segment struct {
	ID          string
	Label       string
	Description string

	// needsCwd marca si el segment necesita la variable $cwd extraída.
	needsCwd bool
	// extras son extracciones jq adicionales (folder/branch no usan jq, son derivados).
	extracts []string
	// prep es un bloque bash opcional que computa la variable seg_<id>.
	prep string
	// render es la expresión que se asigna a seg_<id>; si prep ya la define,
	// dejá render vacío. Si render está seteado, prep es vacío y se usa
	// `seg_<id>="<render>"` directamente.
	render string
}

// AllSegments retorna los segments disponibles para el builder Custom,
// en el orden por defecto en que se muestran y se renderizan.
func AllSegments() []Segment {
	return []Segment{
		{
			ID:          "folder",
			Label:       "Carpeta actual",
			Description: "nombre de la carpeta de trabajo (cyan, bold)",
			needsCwd:    true,
			prep: `folder=$(basename "$cwd")
seg_folder="${CYAN}${BOLD}${folder}${RESET}"`,
		},
		{
			ID:          "branch",
			Label:       "Rama git con dirty indicator",
			Description: "branch actual; * cuando hay cambios sin commitear",
			needsCwd:    true,
			prep: `branch=""
dirty=""
if git -C "$cwd" rev-parse --git-dir >/dev/null 2>&1; then
  branch=$(git -C "$cwd" symbolic-ref --short HEAD 2>/dev/null \
           || git -C "$cwd" rev-parse --short HEAD 2>/dev/null)
  if ! git -C "$cwd" diff --quiet 2>/dev/null \
      || ! git -C "$cwd" diff --cached --quiet 2>/dev/null; then
    dirty="*"
  fi
fi
seg_branch=""
if [ -n "$branch" ]; then
  if [ -n "$dirty" ]; then
    seg_branch="${YELLOW}${branch}${dirty}${RESET}"
  else
    seg_branch="${GREEN}${branch}${RESET}"
  fi
fi`,
		},
		{
			ID:          "model",
			Label:       "Modelo en uso",
			Description: "nombre del modelo activo (magenta)",
			extracts:    []string{`model=$(echo "$input" | jq -r '.model.display_name // "?"')`},
			render:      `${MAGENTA}${model}${RESET}`,
		},
		{
			ID:          "contextBar",
			Label:       "Barra de contexto con %",
			Description: "barra ████░░░░░░ con porcentaje, color según uso",
			extracts:    []string{`used_pct=$(echo "$input" | jq -r '.context_window.used_percentage // empty')`},
			prep: `seg_contextBar=""
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
  seg_contextBar="${bar_color}${bar_fill}${DIM}${bar_empty}${RESET} ${bar_color}${used_int}%${RESET}"
fi`,
		},
		{
			ID:          "contextPercent",
			Label:       "% de contexto solo",
			Description: "porcentaje de contexto sin barra, color según uso",
			extracts:    []string{`used_pct=$(echo "$input" | jq -r '.context_window.used_percentage // empty')`},
			prep: `seg_contextPercent=""
if [ -n "$used_pct" ]; then
  used_int=$(printf '%.0f' "$used_pct")
  if [ "$used_int" -ge 80 ]; then
    seg_contextPercent="${RED}${used_int}%${RESET}"
  elif [ "$used_int" -ge 50 ]; then
    seg_contextPercent="${YELLOW}${used_int}%${RESET}"
  else
    seg_contextPercent="${GREEN}${used_int}%${RESET}"
  fi
fi`,
		},
		{
			ID:          "tokens",
			Label:       "Tokens entrada/salida",
			Description: "↑in ↓out con formato Xk cuando aplica",
			extracts: []string{
				`total_in=$(echo "$input" | jq -r '.context_window.total_input_tokens // 0')`,
				`total_out=$(echo "$input" | jq -r '.context_window.total_output_tokens // 0')`,
			},
			prep: `seg_tokens=""
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
  seg_tokens="${DIM}↑${in_fmt} ↓${out_fmt}${RESET}"
fi`,
		},
		{
			ID:          "outputStyle",
			Label:       "Output style activo",
			Description: "nombre del output style configurado",
			extracts:    []string{`output_style=$(echo "$input" | jq -r '.output_style.name // empty')`},
			prep: `seg_outputStyle=""
if [ -n "$output_style" ]; then
  seg_outputStyle="${DIM}style:${RESET} ${CYAN}${output_style}${RESET}"
fi`,
		},
		{
			ID:          "permissionMode",
			Label:       "Permission mode",
			Description: "modo de permisos actual (default/acceptEdits/etc)",
			extracts:    []string{`permission_mode=$(echo "$input" | jq -r '.permission_mode // empty')`},
			prep: `seg_permissionMode=""
if [ -n "$permission_mode" ]; then
  seg_permissionMode="${DIM}perm:${RESET} ${YELLOW}${permission_mode}${RESET}"
fi`,
		},
		{
			ID:          "time",
			Label:       "Hora local HH:MM",
			Description: "hora del sistema en formato 24h",
			prep:        `seg_time="${DIM}$(date +%H:%M)${RESET}"`,
		},
	}
}

// FindSegment busca un segment por ID. Retorna nil si no existe.
func FindSegment(id string) *Segment {
	all := AllSegments()
	for i := range all {
		if all[i].ID == id {
			return &all[i]
		}
	}
	return nil
}

// BuildCustomScript genera el script bash completo a partir de los IDs
// activos, en el orden recibido. IDs desconocidos se ignoran. Si activeIDs
// está vacío retorna error: el script vacío no tiene sentido.
func BuildCustomScript(activeIDs []string) (string, error) {
	if len(activeIDs) == 0 {
		return "", fmt.Errorf("se requiere al menos un segmento activo")
	}

	// Resolver y deduplicar segments preservando orden de entrada.
	seen := make(map[string]bool, len(activeIDs))
	active := make([]Segment, 0, len(activeIDs))
	for _, id := range activeIDs {
		if seen[id] {
			continue
		}
		seg := FindSegment(id)
		if seg == nil {
			continue
		}
		seen[id] = true
		active = append(active, *seg)
	}
	if len(active) == 0 {
		return "", fmt.Errorf("ningún ID activo es válido")
	}

	needsCwd := false
	for _, s := range active {
		if s.needsCwd {
			needsCwd = true
			break
		}
	}

	// Deduplicar líneas de extract (mismo jq se reusa entre segments).
	var extracts []string
	extractSeen := make(map[string]bool)
	addExtract := func(line string) {
		if extractSeen[line] {
			return
		}
		extractSeen[line] = true
		extracts = append(extracts, line)
	}
	if needsCwd {
		addExtract(`cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd // "?"')`)
	}
	for _, s := range active {
		for _, e := range s.extracts {
			addExtract(e)
		}
	}

	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("# monocle preset: custom — generado por el segment builder\n")
	b.WriteString("input=$(cat)\n\n")

	for _, e := range extracts {
		b.WriteString(e)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Paleta ANSI compartida (alineada con full.sh / compact.sh).
	b.WriteString(`RESET='\033[0m'
DIM='\033[2m'
CYAN='\033[36m'
YELLOW='\033[33m'
GREEN='\033[32m'
RED='\033[31m'
MAGENTA='\033[35m'
BOLD='\033[1m'

`)

	// Bloques de preparación / render por segment.
	for _, s := range active {
		if s.prep != "" {
			b.WriteString(s.prep)
			b.WriteString("\n\n")
			continue
		}
		// render-only: armar `seg_<id>="<render>"`.
		fmt.Fprintf(&b, `seg_%s="%s"`+"\n\n", s.ID, s.render)
	}

	// Composición final con separadores dimmed.
	b.WriteString(`sep="${DIM} │ ${RESET}"` + "\n")
	b.WriteString("out=\"\"\n")
	for _, s := range active {
		fmt.Fprintf(&b, `if [ -n "$seg_%s" ]; then
  if [ -z "$out" ]; then
    out="$seg_%s"
  else
    out="${out}${sep}$seg_%s"
  fi
fi
`, s.ID, s.ID, s.ID)
	}
	b.WriteString("\nprintf \"%b\\n\" \"$out\"\n")

	return b.String(), nil
}
