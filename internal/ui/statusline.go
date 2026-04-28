package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CogniDevAI/monocle/internal/presets"
	"github.com/CogniDevAI/monocle/internal/settings"
)

// statuslineModel es el sub-modelo para elegir y aplicar un preset
// de statusline. Flujo: list de presets → preview + confirm → apply.
// Si el usuario elige "Custom (armá el tuyo)", el flujo pasa por stepBuild,
// donde activa/desactiva segments y Monocle genera el script bash.
type statuslineModel struct {
	list     list.Model
	preview  string
	settings *settings.Settings
	width    int
	height   int
	step     stepState
	chosen   *presets.Preset

	// stepBuild state
	segments []segmentItem
	cursor   int

	err error
}

type stepState int

const (
	stepPick stepState = iota
	stepConfirm
	stepBuild
	stepDone
)

// segmentItem envuelve un Segment con su flag enabled para el builder.
type segmentItem struct {
	seg     presets.Segment
	enabled bool
}

func newStatuslineModel(st *settings.Settings, w, h int) *statuslineModel {
	all := presets.All()
	items := make([]list.Item, 0, len(all)+1)
	for _, p := range all {
		items = append(items, presetItem{preset: p})
	}
	items = append(items, presetItem{
		isCustom:    true,
		customName:  "Custom (armá el tuyo)",
		customDescr: "elegí qué segmentos mostrar (folder, rama, modelo, contexto, tokens, etc.)",
	})

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, w-4, h-6)
	l.Title = "Elegí un preset de statusline"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#F5B544")).
		Foreground(lipgloss.Color("#1A1308")).
		Padding(0, 1).
		Bold(true)

	return &statuslineModel{
		list:     l,
		settings: st,
		width:    w,
		height:   h,
	}
}

func (m *statuslineModel) Init() tea.Cmd { return nil }

func (m *statuslineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		m.list.SetSize(v.Width-4, v.Height-6)

	case tea.KeyMsg:
		switch m.step {
		case stepPick:
			if key.Matches(v, backKey) {
				return m, sendBack("")
			}
			if key.Matches(v, enterKey) {
				it, ok := m.list.SelectedItem().(presetItem)
				if !ok {
					return m, nil
				}
				if it.isCustom {
					m.segments = newSegmentItems()
					m.cursor = 0
					m.step = stepBuild
					return m, nil
				}
				body, err := it.preset.Content()
				if err != nil {
					m.err = err
					return m, nil
				}
				m.preview = string(body)
				m.chosen = &it.preset
				m.step = stepConfirm
				return m, nil
			}
		case stepConfirm:
			if key.Matches(v, backKey) {
				m.step = stepPick
				m.preview = ""
				m.chosen = nil
				return m, nil
			}
			if key.Matches(v, applyKey) {
				if err := m.apply(); err != nil {
					m.err = err
					return m, nil
				}
				name := m.chosen.Name
				return m, sendBack(fmt.Sprintf("✓ Statusline aplicado: %s", name))
			}
		case stepBuild:
			if key.Matches(v, backKey) {
				m.step = stepPick
				m.segments = nil
				m.cursor = 0
				m.err = nil
				return m, nil
			}
			switch {
			case key.Matches(v, upKey):
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case key.Matches(v, downKey):
				if m.cursor < len(m.segments)-1 {
					m.cursor++
				}
				return m, nil
			case key.Matches(v, toggleKey):
				if m.cursor >= 0 && m.cursor < len(m.segments) {
					m.segments[m.cursor].enabled = !m.segments[m.cursor].enabled
				}
				return m, nil
			case key.Matches(v, saveKey):
				path, err := m.applyCustom()
				if err != nil {
					m.err = err
					return m, nil
				}
				return m, sendBack(fmt.Sprintf("✓ Statusline custom aplicado en %s", path))
			}
		case stepDone:
			return m, sendBack("")
		}
	}

	if m.step == stepPick {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *statuslineModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepPick:
		hint := dimStyle.Render("↑↓ moverse · enter para previsualizar · esc para volver")
		return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), hint)

	case stepConfirm:
		title := titleStyle.Render(fmt.Sprintf("Preview: %s", m.chosen.Name))
		body := previewStyle.Render(m.preview)
		hint := dimStyle.Render("y para aplicar · esc para volver a la lista")
		warn := warnStyle.Render(
			"Se hará backup de ~/.claude/settings.json antes de modificarlo.",
		)
		return lipgloss.JoinVertical(lipgloss.Left, title, body, warn, hint)

	case stepBuild:
		title := titleStyle.Render("Custom — armá tu statusline")
		body := m.renderSegmentList()
		activeCount := m.activeCount()
		var status string
		if activeCount == 0 {
			status = warnStyle.Render("Activá al menos un segmento para poder guardar.")
		} else {
			status = dimStyle.Render(fmt.Sprintf("%d segmento(s) activo(s)", activeCount))
		}
		hint := dimStyle.Render("espacio toggle · ↑↓ moverse · s para guardar · esc para volver")
		return lipgloss.JoinVertical(lipgloss.Left, title, body, status, hint)

	default:
		return ""
	}
}

// renderSegmentList renderiza la lista de segments con checkboxes y cursor.
func (m *statuslineModel) renderSegmentList() string {
	var b strings.Builder
	b.WriteString("\n")
	for i, s := range m.segments {
		check := "[ ]"
		if s.enabled {
			check = "[x]"
		}
		cursor := "  "
		line := fmt.Sprintf("%s%s %s — %s", cursor, check, s.seg.Label, s.seg.Description)
		if i == m.cursor {
			line = segmentCursorStyle.Render(fmt.Sprintf("> %s %s — %s", check, s.seg.Label, s.seg.Description))
		} else if s.enabled {
			line = segmentActiveStyle.Render(line)
		} else {
			line = segmentInactiveStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *statuslineModel) activeCount() int {
	n := 0
	for _, s := range m.segments {
		if s.enabled {
			n++
		}
	}
	return n
}

func (m *statuslineModel) activeIDs() []string {
	ids := make([]string, 0, len(m.segments))
	for _, s := range m.segments {
		if s.enabled {
			ids = append(ids, s.seg.ID)
		}
	}
	return ids
}

// newSegmentItems arma la lista inicial: todos los segments DESACTIVADOS,
// para que el usuario elija explícitamente qué quiere ver.
func newSegmentItems() []segmentItem {
	all := presets.AllSegments()
	out := make([]segmentItem, 0, len(all))
	for _, s := range all {
		out = append(out, segmentItem{seg: s, enabled: false})
	}
	return out
}

func (m *statuslineModel) apply() error {
	scriptPath, err := m.chosen.Install("")
	if err != nil {
		return fmt.Errorf("instalando preset: %w", err)
	}
	m.settings.SetStatusLineCommand(scriptPath)
	if err := m.settings.Save(); err != nil {
		return fmt.Errorf("guardando settings: %w", err)
	}
	return nil
}

// applyCustom genera el script bash a partir de los segments activos,
// lo escribe a ~/.claude/statusline.sh y actualiza settings.json.
// Retorna el path final.
func (m *statuslineModel) applyCustom() (string, error) {
	ids := m.activeIDs()
	if len(ids) == 0 {
		return "", fmt.Errorf("activá al menos un segmento")
	}
	body, err := presets.BuildCustomScript(ids)
	if err != nil {
		return "", fmt.Errorf("generando script: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	path := filepath.Join(home, ".claude", "statusline.sh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creando directorio: %w", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return "", fmt.Errorf("escribiendo %s: %w", path, err)
	}
	m.settings.SetStatusLineCommand(path)
	if err := m.settings.Save(); err != nil {
		return "", fmt.Errorf("guardando settings: %w", err)
	}
	return path, nil
}

// presetItem representa un item de la lista de selección. Si isCustom es true,
// el item dispara el flujo del builder de segments y los campos de preset se ignoran.
type presetItem struct {
	preset      presets.Preset
	isCustom    bool
	customName  string
	customDescr string
}

func (p presetItem) Title() string {
	if p.isCustom {
		return p.customName
	}
	return p.preset.Name
}

func (p presetItem) Description() string {
	if p.isCustom {
		return p.customDescr
	}
	return p.preset.Description
}

func (p presetItem) FilterValue() string {
	if p.isCustom {
		return p.customName
	}
	return p.preset.Name
}

func sendBack(msg string) tea.Cmd {
	return func() tea.Msg { return backToMenuMsg(msg) }
}

var (
	backKey   = key.NewBinding(key.WithKeys("esc", "q"))
	applyKey  = key.NewBinding(key.WithKeys("y", "enter"))
	saveKey   = key.NewBinding(key.WithKeys("s"))
	toggleKey = key.NewBinding(key.WithKeys(" "))
	upKey     = key.NewBinding(key.WithKeys("up", "k"))
	downKey   = key.NewBinding(key.WithKeys("down", "j"))

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F5B544")).
			Bold(true).
			Padding(0, 2).
			MarginTop(1)
	previewStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			MarginTop(1).
			MarginBottom(1).
			Foreground(lipgloss.Color("250"))
	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F5B544")).
			Padding(0, 2).
			Italic(true)
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 2).
			MarginTop(1)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Padding(2)

	segmentCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F5B544")).
				Bold(true).
				Padding(0, 2)
	segmentActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				Padding(0, 2)
	segmentInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Padding(0, 2)
)
