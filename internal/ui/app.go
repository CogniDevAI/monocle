// Package ui contiene los modelos Bubble Tea de Monocle.
//
// La pantalla principal es un menú de secciones (statusline, hooks futuros,
// permisos futuros). Cada sección es un sub-modelo independiente que se
// activa al seleccionarla y devuelve el control al menú al terminar.
package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CogniDevAI/monocle/internal/settings"
)

type appState int

const (
	stateMenu appState = iota
	stateStatusline
)

// App es el modelo raíz que orquesta los sub-modelos.
type App struct {
	state    appState
	menu     list.Model
	sub      tea.Model
	settings *settings.Settings
	width    int
	height   int
	flash    string // mensaje efímero (post-acción)
}

// NewApp construye el modelo inicial cargando settings.json.
func NewApp() (*App, error) {
	path, err := settings.DefaultPath()
	if err != nil {
		return nil, err
	}
	st, err := settings.Load(path)
	if err != nil {
		return nil, err
	}

	items := []list.Item{
		menuItem{id: "statusline", title: "Statusline", desc: "configurar la barra de estado"},
		menuItem{id: "exit", title: "Salir", desc: "cerrar Monocle"},
	}
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Monocle — configurador de Claude Code"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#F5B544")).
		Foreground(lipgloss.Color("#1A1308")).
		Padding(0, 1).
		Bold(true)

	return &App{
		state:    stateMenu,
		menu:     l,
		settings: st,
	}, nil
}

func (a *App) Init() tea.Cmd { return nil }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.menu.SetSize(m.Width-4, m.Height-6)
		if a.sub != nil {
			updated, cmd := a.sub.Update(msg)
			a.sub = updated
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		if a.state == stateMenu {
			if key.Matches(m, quitKey) {
				return a, tea.Quit
			}
			if key.Matches(m, enterKey) {
				return a.activateSelection()
			}
		}

	case backToMenuMsg:
		a.state = stateMenu
		a.sub = nil
		a.flash = string(m)
		return a, nil
	}

	if a.state == stateMenu {
		var cmd tea.Cmd
		a.menu, cmd = a.menu.Update(msg)
		return a, cmd
	}

	if a.sub != nil {
		updated, cmd := a.sub.Update(msg)
		a.sub = updated
		return a, cmd
	}
	return a, nil
}

func (a *App) View() string {
	if a.state == stateStatusline && a.sub != nil {
		return a.sub.View()
	}

	body := a.menu.View()
	if a.flash != "" {
		body = lipgloss.JoinVertical(lipgloss.Left,
			body,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#5FD6C4")).
				Padding(1, 2).Render(a.flash),
		)
	}
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2).
		Render(fmt.Sprintf("settings: %s   ↑↓ moverse · enter elegir · q salir", a.settings.Path()))
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (a *App) activateSelection() (tea.Model, tea.Cmd) {
	sel, ok := a.menu.SelectedItem().(menuItem)
	if !ok {
		return a, nil
	}
	switch sel.id {
	case "statusline":
		a.flash = ""
		a.state = stateStatusline
		a.sub = newStatuslineModel(a.settings, a.width, a.height)
		return a, a.sub.Init()
	case "exit":
		return a, tea.Quit
	}
	return a, nil
}

// backToMenuMsg lo emite un sub-modelo para volver al menú principal.
// Su contenido es el mensaje de flash que se muestra al usuario.
type backToMenuMsg string

var (
	quitKey  = key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"))
	enterKey = key.NewBinding(key.WithKeys("enter"))
)

type menuItem struct {
	id    string
	title string
	desc  string
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }
