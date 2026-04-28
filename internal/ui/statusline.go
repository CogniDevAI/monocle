package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CogniDevAI/monocle/internal/presets"
	"github.com/CogniDevAI/monocle/internal/settings"
)

// statuslineModel es el sub-modelo para elegir y aplicar un preset
// de statusline. Flujo: list de presets → preview + confirm → apply.
// Si el usuario elige "Custom", el flujo pasa por stepEditCustom (textarea).
type statuslineModel struct {
	list       list.Model
	preview    string
	settings   *settings.Settings
	width      int
	height     int
	step       stepState
	chosen     *presets.Preset
	customArea textarea.Model
	err        error
}

type stepState int

const (
	stepPick stepState = iota
	stepConfirm
	stepEditCustom
	stepDone
)

const customPlaceholder = `#!/usr/bin/env bash
# escribí tu statusline acá
input=$(cat)
printf "..."
`

func newStatuslineModel(st *settings.Settings, w, h int) *statuslineModel {
	all := presets.All()
	items := make([]list.Item, 0, len(all)+1)
	for _, p := range all {
		items = append(items, presetItem{preset: p})
	}
	items = append(items, presetItem{
		isCustom:    true,
		customName:  "Custom (escribir el mío)",
		customDescr: "abrí un editor y pegá/escribí tu propio script bash",
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
		if m.step == stepEditCustom {
			m.customArea.SetWidth(v.Width - 6)
			m.customArea.SetHeight(v.Height - 8)
		}

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
					m.customArea = newCustomTextarea(m.width, m.height)
					m.step = stepEditCustom
					return m, textarea.Blink
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
		case stepEditCustom:
			// En el editor solo `esc` cancela — `q` debe poder escribirse
			// como parte del script bash sin abortar la edición.
			if v.String() == "esc" {
				m.step = stepPick
				m.customArea.Reset()
				m.err = nil
				return m, nil
			}
			if key.Matches(v, saveKey) {
				path, err := m.applyCustom()
				if err != nil {
					m.err = err
					return m, nil
				}
				return m, sendBack(fmt.Sprintf("✓ Statusline custom aplicado en %s", path))
			}
			var cmd tea.Cmd
			m.customArea, cmd = m.customArea.Update(msg)
			return m, cmd
		case stepDone:
			return m, sendBack("")
		}
	}

	switch m.step {
	case stepPick:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case stepEditCustom:
		var cmd tea.Cmd
		m.customArea, cmd = m.customArea.Update(msg)
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

	case stepEditCustom:
		title := titleStyle.Render("Statusline custom")
		hint := dimStyle.Render("ctrl+s para guardar · esc para volver")
		warn := warnStyle.Render(
			"Se guardará en ~/.claude/statusline.sh y se hará backup de settings.json.",
		)
		return lipgloss.JoinVertical(lipgloss.Left, title, m.customArea.View(), warn, hint)

	default:
		return ""
	}
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

// applyCustom valida el contenido del textarea, lo escribe a
// ~/.claude/statusline.sh y actualiza settings.json. Retorna el path final.
func (m *statuslineModel) applyCustom() (string, error) {
	body := strings.TrimSpace(m.customArea.Value())
	if body == "" {
		return "", fmt.Errorf("el script está vacío")
	}
	if !strings.HasPrefix(body, "#!") {
		return "", fmt.Errorf("falta el shebang (la primera línea debe ser tipo #!/usr/bin/env bash)")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	path := filepath.Join(home, ".claude", "statusline.sh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creando directorio: %w", err)
	}
	// Asegurar newline final
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
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

func newCustomTextarea(w, h int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = customPlaceholder
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.SetWidth(w - 6)
	ta.SetHeight(h - 8)
	ta.Focus()
	return ta
}

// presetItem representa un item de la lista de selección. Si isCustom es true,
// el item dispara el flujo de editor custom y los campos de preset se ignoran.
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
	backKey  = key.NewBinding(key.WithKeys("esc", "q"))
	applyKey = key.NewBinding(key.WithKeys("y", "enter"))
	saveKey  = key.NewBinding(key.WithKeys("ctrl+s"))

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
)
