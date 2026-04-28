package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CogniDevAI/monocle/internal/presets"
	"github.com/CogniDevAI/monocle/internal/settings"
)

// statuslineModel es el sub-modelo para elegir y aplicar un preset
// de statusline. Flujo: list de presets → preview + confirm → apply.
type statuslineModel struct {
	list     list.Model
	preview  string
	settings *settings.Settings
	width    int
	height   int
	step     stepState
	chosen   *presets.Preset
	err      error
}

type stepState int

const (
	stepPick stepState = iota
	stepConfirm
	stepDone
)

func newStatuslineModel(st *settings.Settings, w, h int) *statuslineModel {
	items := make([]list.Item, 0, len(presets.All()))
	for _, p := range presets.All() {
		items = append(items, presetItem{p})
	}
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

type presetItem struct{ preset presets.Preset }

func (p presetItem) Title() string       { return p.preset.Name }
func (p presetItem) Description() string { return p.preset.Description }
func (p presetItem) FilterValue() string { return p.preset.Name }

func sendBack(msg string) tea.Cmd {
	return func() tea.Msg { return backToMenuMsg(msg) }
}

var (
	backKey  = key.NewBinding(key.WithKeys("esc", "q"))
	applyKey = key.NewBinding(key.WithKeys("y", "enter"))

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
