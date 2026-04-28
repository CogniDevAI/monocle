package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CogniDevAI/monocle/internal/settings"
)

// hooksModel es el sub-modelo para inspeccionar y eliminar hooks en
// settings.json. v0.1: solo listar y borrar — no se crean ni editan
// entradas (eso queda para v0.2).
//
// Flujo:
//   stepHookEvents       → lista de eventos (PreToolUse, etc.) con su count
//   stepHookEntries      → lista de hooks de un evento (matcher + command)
//   stepHookConfirmDel   → confirm para eliminar un hook → save → flash
type hooksModel struct {
	settings *settings.Settings
	width    int
	height   int

	step hookStep

	events list.Model // step 1
	entries list.Model // step 2

	currentEvent string
	pendingIdx   int // índice del entry a borrar (en stepHookConfirmDel)
	flash        string
	err          error
}

type hookStep int

const (
	stepHookEvents hookStep = iota
	stepHookEntries
	stepHookConfirmDel
)

// hookEvents son los eventos válidos según la doc de Claude Code.
// Los listamos siempre, aunque el bloque hooks esté vacío, para que el
// usuario tenga un mapa completo de qué se puede configurar.
var hookEvents = []string{
	"PreToolUse",
	"PostToolUse",
	"UserPromptSubmit",
	"Notification",
	"Stop",
	"SubagentStop",
	"SessionStart",
	"SessionEnd",
	"PreCompact",
}

func newHooksModel(st *settings.Settings, w, h int) *hooksModel {
	m := &hooksModel{
		settings: st,
		width:    w,
		height:   h,
		step:     stepHookEvents,
	}
	m.rebuildEventsList()
	return m
}

func (m *hooksModel) rebuildEventsList() {
	hooks := m.settings.Hooks()
	items := make([]list.Item, 0, len(hookEvents))
	for _, ev := range hookEvents {
		count := len(hooks[ev])
		desc := "sin hooks configurados"
		if count == 1 {
			desc = "1 hook configurado"
		} else if count > 1 {
			desc = fmt.Sprintf("%d hooks configurados", count)
		}
		items = append(items, hookEventItem{event: ev, count: count, desc: desc})
	}
	w, h := m.listSize()
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, w, h)
	l.Title = "Hooks — elegí un evento"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#F5B544")).
		Foreground(lipgloss.Color("#1A1308")).
		Padding(0, 1).
		Bold(true)
	m.events = l
}

func (m *hooksModel) rebuildEntriesList() {
	hooks := m.settings.Hooks()
	entries := hooks[m.currentEvent]
	items := make([]list.Item, 0, len(entries))
	for i, raw := range entries {
		entry, _ := raw.(map[string]any)
		matcher, _ := entry["matcher"].(string)
		if matcher == "" {
			matcher = "(sin matcher)"
		}
		cmd := summarizeHookCommands(entry)
		items = append(items, hookEntryItem{
			idx:     i,
			matcher: matcher,
			cmd:     cmd,
		})
	}
	w, h := m.listSize()
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, w, h)
	l.Title = fmt.Sprintf("Hooks de %s", m.currentEvent)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#F5B544")).
		Foreground(lipgloss.Color("#1A1308")).
		Padding(0, 1).
		Bold(true)
	m.entries = l
}

// summarizeHookCommands extrae una línea legible de los comandos de un entry.
// Un entry tiene la forma {matcher, hooks: [{type, command}, ...]}. Mostramos
// el primer comando y, si hay más, anotamos "(+N más)".
func summarizeHookCommands(entry map[string]any) string {
	rawHooks, ok := entry["hooks"].([]any)
	if !ok || len(rawHooks) == 0 {
		return "(sin comandos)"
	}
	first, _ := rawHooks[0].(map[string]any)
	cmd, _ := first["command"].(string)
	if cmd == "" {
		cmd = "(comando vacío)"
	}
	if len(rawHooks) > 1 {
		return fmt.Sprintf("%s  (+%d más)", cmd, len(rawHooks)-1)
	}
	return cmd
}

func (m *hooksModel) listSize() (int, int) {
	w := m.width - 4
	h := m.height - 6
	if w < 20 {
		w = 20
	}
	if h < 5 {
		h = 5
	}
	return w, h
}

func (m *hooksModel) Init() tea.Cmd { return nil }

func (m *hooksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.events.SetSize(w, h)
		m.entries.SetSize(w, h)

	case tea.KeyMsg:
		switch m.step {
		case stepHookEvents:
			if key.Matches(v, backKey) {
				return m, sendBack("")
			}
			if key.Matches(v, enterKey) {
				it, ok := m.events.SelectedItem().(hookEventItem)
				if !ok {
					return m, nil
				}
				m.currentEvent = it.event
				m.flash = ""
				m.err = nil
				m.rebuildEntriesList()
				m.step = stepHookEntries
				return m, nil
			}

		case stepHookEntries:
			if key.Matches(v, backKey) {
				m.step = stepHookEvents
				m.flash = ""
				m.err = nil
				m.rebuildEventsList()
				return m, nil
			}
			if key.Matches(v, deleteKey) {
				it, ok := m.entries.SelectedItem().(hookEntryItem)
				if !ok {
					return m, nil
				}
				m.pendingIdx = it.idx
				m.step = stepHookConfirmDel
				return m, nil
			}

		case stepHookConfirmDel:
			if key.Matches(v, backKey) {
				m.step = stepHookEntries
				return m, nil
			}
			if key.Matches(v, applyKey) {
				if err := m.deleteCurrent(); err != nil {
					m.err = err
					return m, nil
				}
				m.flash = "✓ Hook eliminado"
				m.rebuildEntriesList()
				m.step = stepHookEntries
				return m, nil
			}
		}
	}

	switch m.step {
	case stepHookEvents:
		var cmd tea.Cmd
		m.events, cmd = m.events.Update(msg)
		return m, cmd
	case stepHookEntries:
		var cmd tea.Cmd
		m.entries, cmd = m.entries.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *hooksModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepHookEvents:
		hint := dimStyle.Render(
			"↑↓ moverse · enter elegir evento · esc volver al menú\n" +
				"agregar/editar próximamente — por ahora solo listar y eliminar",
		)
		return lipgloss.JoinVertical(lipgloss.Left, m.events.View(), hint)

	case stepHookEntries:
		body := m.entries.View()
		if len(m.entries.Items()) == 0 {
			body = dimStyle.Render(
				fmt.Sprintf("No hay hooks configurados para %s.", m.currentEvent),
			)
		}
		flash := ""
		if m.flash != "" {
			flash = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5FD6C4")).
				Padding(0, 2).
				Render(m.flash)
		}
		hint := dimStyle.Render(
			"↑↓ moverse · d eliminar · esc volver\n" +
				"agregar/editar próximamente",
		)
		parts := []string{body}
		if flash != "" {
			parts = append(parts, flash)
		}
		parts = append(parts, hint)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)

	case stepHookConfirmDel:
		hooks := m.settings.Hooks()
		entries := hooks[m.currentEvent]
		var detail string
		if m.pendingIdx >= 0 && m.pendingIdx < len(entries) {
			entry, _ := entries[m.pendingIdx].(map[string]any)
			matcher, _ := entry["matcher"].(string)
			if matcher == "" {
				matcher = "(sin matcher)"
			}
			detail = fmt.Sprintf("%s · %s", matcher, summarizeHookCommands(entry))
		}
		title := titleStyle.Render(fmt.Sprintf("Eliminar hook de %s", m.currentEvent))
		body := previewStyle.Render(detail)
		warn := warnStyle.Render(
			"Se hará backup de ~/.claude/settings.json antes de modificarlo.",
		)
		hint := dimStyle.Render("y para confirmar · esc para cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, title, body, warn, hint)
	}

	return ""
}

func (m *hooksModel) deleteCurrent() error {
	hooks := m.settings.Hooks()
	entries := hooks[m.currentEvent]
	if m.pendingIdx < 0 || m.pendingIdx >= len(entries) {
		return fmt.Errorf("índice fuera de rango")
	}
	entries = append(entries[:m.pendingIdx], entries[m.pendingIdx+1:]...)
	if len(entries) == 0 {
		delete(hooks, m.currentEvent)
	} else {
		hooks[m.currentEvent] = entries
	}
	m.settings.SetHooks(hooks)
	if err := m.settings.Save(); err != nil {
		return fmt.Errorf("guardando settings: %w", err)
	}
	return nil
}

// hookEventItem es una entrada de la lista de eventos.
type hookEventItem struct {
	event string
	count int
	desc  string
}

func (h hookEventItem) Title() string {
	if h.count == 0 {
		return h.event
	}
	return fmt.Sprintf("%s (%d)", h.event, h.count)
}
func (h hookEventItem) Description() string { return h.desc }
func (h hookEventItem) FilterValue() string { return h.event }

// hookEntryItem es una entrada concreta dentro de un evento.
type hookEntryItem struct {
	idx     int
	matcher string
	cmd     string
}

func (h hookEntryItem) Title() string       { return h.matcher }
func (h hookEntryItem) Description() string { return h.cmd }
func (h hookEntryItem) FilterValue() string { return h.matcher }

var deleteKey = key.NewBinding(key.WithKeys("d", "delete", "backspace"))
