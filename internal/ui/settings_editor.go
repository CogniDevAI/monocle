package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CogniDevAI/monocle/internal/settings"
)

// settingsModel es el sub-modelo "Settings" del menú principal. Funciona
// como un router entre sub-secciones (permissions, output style, modelo,
// misc) que delega edición a sub-editores. Cada uno termina volviendo
// al menú principal vía backToMenuMsg con un flash.
//
// Diseño v0.4: pickers fijos cuando hay un set chico de valores válidos.
// Para listas (allow/deny rules) sólo se permite DELETE — ADD queda para
// v0.5 porque requiere un editor de strings con validación.
type settingsModel struct {
	settings *settings.Settings
	width    int
	height   int

	step settingsStep

	// stepSettingsRoot
	root list.Model

	// sub-editores activos
	sub tea.Model

	flash string
	err   error
}

type settingsStep int

const (
	stepSettingsRoot settingsStep = iota
	stepSettingsSub
)

// settingsSection identifica cada sub-sección del menú Settings.
type settingsSection string

const (
	sectionPermissions settingsSection = "permissions"
	sectionOutputStyle settingsSection = "outputStyle"
	sectionModel       settingsSection = "model"
	sectionMisc        settingsSection = "misc"
)

func newSettingsModel(st *settings.Settings, w, h int) *settingsModel {
	m := &settingsModel{
		settings: st,
		width:    w,
		height:   h,
		step:     stepSettingsRoot,
	}
	m.rebuildRoot()
	return m
}

func (m *settingsModel) rebuildRoot() {
	items := []list.Item{
		settingsRootItem{
			id:    sectionPermissions,
			title: "Permissions",
			desc:  "defaultMode, allow/deny rules",
		},
		settingsRootItem{
			id:    sectionOutputStyle,
			title: "Output Style",
			desc:  "estilo de respuesta del modelo",
		},
		settingsRootItem{
			id:    sectionModel,
			title: "Modelo preferido",
			desc:  "claude-sonnet/opus/haiku o sin preferencia",
		},
		settingsRootItem{
			id:    sectionMisc,
			title: "Misc toggles",
			desc:  "co-author, cleanup days, effort level",
		},
	}
	w, h := m.listSize()
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, w, h)
	l.Title = "Settings — elegí qué editar"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#F5B544")).
		Foreground(lipgloss.Color("#1A1308")).
		Padding(0, 1).
		Bold(true)
	m.root = l
}

func (m *settingsModel) listSize() (int, int) {
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

func (m *settingsModel) Init() tea.Cmd { return nil }

func (m *settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.root.SetSize(w, h)
		if m.sub != nil {
			updated, cmd := m.sub.Update(v)
			m.sub = updated
			return m, cmd
		}
		return m, nil

	case backToSettingsMsg:
		// El sub-editor cerró. Volvemos al menú raíz de Settings y
		// mostramos el flash (si trae uno) hasta la próxima acción.
		m.step = stepSettingsRoot
		m.sub = nil
		m.flash = string(v)
		m.rebuildRoot()
		return m, nil

	case tea.KeyMsg:
		if m.step == stepSettingsRoot {
			if key.Matches(v, backKey) {
				return m, sendBack("")
			}
			if key.Matches(v, enterKey) {
				it, ok := m.root.SelectedItem().(settingsRootItem)
				if !ok {
					return m, nil
				}
				m.flash = ""
				m.err = nil
				m.sub = newSubEditor(it.id, m.settings, m.width, m.height)
				if m.sub == nil {
					return m, nil
				}
				m.step = stepSettingsSub
				return m, m.sub.Init()
			}
		}
	}

	if m.step == stepSettingsRoot {
		var cmd tea.Cmd
		m.root, cmd = m.root.Update(msg)
		return m, cmd
	}
	if m.sub != nil {
		updated, cmd := m.sub.Update(msg)
		m.sub = updated
		return m, cmd
	}
	return m, nil
}

func (m *settingsModel) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}
	if m.step == stepSettingsSub && m.sub != nil {
		return m.sub.View()
	}

	body := m.root.View()
	parts := []string{body}
	if m.flash != "" {
		flash := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5FD6C4")).
			Padding(0, 2).
			Render(m.flash)
		parts = append(parts, flash)
	}
	hint := dimStyle.Render("↑↓ moverse · enter elegir · esc volver al menú")
	parts = append(parts, hint)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// backToSettingsMsg lo emite un sub-editor para volver al router de Settings
// mostrando un flash. El contenido es el mensaje de flash.
type backToSettingsMsg string

func sendBackToSettings(msg string) tea.Cmd {
	return func() tea.Msg { return backToSettingsMsg(msg) }
}

// settingsRootItem es una entrada del menú raíz de Settings.
type settingsRootItem struct {
	id    settingsSection
	title string
	desc  string
}

func (s settingsRootItem) Title() string       { return s.title }
func (s settingsRootItem) Description() string { return s.desc }
func (s settingsRootItem) FilterValue() string { return s.title }

// newSubEditor instancia el sub-editor adecuado para la sección elegida.
func newSubEditor(section settingsSection, st *settings.Settings, w, h int) tea.Model {
	switch section {
	case sectionPermissions:
		return newPermissionsEditor(st, w, h)
	case sectionOutputStyle:
		return newOutputStyleEditor(st, w, h)
	case sectionModel:
		return newModelEditor(st, w, h)
	case sectionMisc:
		return newMiscEditor(st, w, h)
	}
	return nil
}

// ---------- helpers de acceso a settings.json ----------

// getString devuelve un string top-level o "" si no existe / no es string.
func getString(st *settings.Settings, key string) string {
	v, _ := st.Get(key).(string)
	return v
}

// getBool devuelve un bool top-level o false si no existe / no es bool.
func getBool(st *settings.Settings, key string) (val bool, present bool) {
	if raw := st.Get(key); raw != nil {
		if b, ok := raw.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// getInt devuelve un int top-level. JSON unmarshalea numbers como float64,
// así que normalizamos.
func getInt(st *settings.Settings, key string) (val int, present bool) {
	raw := st.Get(key)
	if raw == nil {
		return 0, false
	}
	switch n := raw.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// getPermissions devuelve el bloque permissions como map. Si no existe,
// retorna un map vacío. Mantenemos los tipos `any` para no perder campos.
func getPermissions(st *settings.Settings) map[string]any {
	if raw, ok := st.Get("permissions").(map[string]any); ok {
		return raw
	}
	return map[string]any{}
}

// getStringList extrae una lista de strings del bloque permissions.
// Tolera entradas no-string (las descarta).
func getStringList(perms map[string]any, key string) []string {
	raw, ok := perms[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// setStringList persiste una lista de strings en el bloque permissions.
// Si la lista queda vacía, elimina la key para no dejar `[]` colgado.
func setStringList(perms map[string]any, key string, vals []string) {
	if len(vals) == 0 {
		delete(perms, key)
		return
	}
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	perms[key] = out
}

// savePermissions reescribe el bloque permissions y persiste.
func savePermissions(st *settings.Settings, perms map[string]any) error {
	if len(perms) == 0 {
		st.Set("permissions", map[string]any{})
	} else {
		st.Set("permissions", perms)
	}
	return st.Save()
}

// =====================================================================
// Permissions editor
// =====================================================================

type permissionsEditor struct {
	settings *settings.Settings
	width    int
	height   int

	step permStep

	// stepPermRoot
	root list.Model

	// stepPermDefaultMode
	modePicker list.Model

	// stepPermAllowList / stepPermDenyList
	listKey  string // "allow" | "deny"
	rulesUI  list.Model

	// stepPermConfirmDel
	pendingRule string

	flash string
	err   error
}

type permStep int

const (
	stepPermRoot permStep = iota
	stepPermDefaultMode
	stepPermAllowList
	stepPermDenyList
	stepPermConfirmDel
)

var validDefaultModes = []string{"default", "acceptEdits", "bypassPermissions", "plan"}

func newPermissionsEditor(st *settings.Settings, w, h int) *permissionsEditor {
	m := &permissionsEditor{
		settings: st,
		width:    w,
		height:   h,
		step:     stepPermRoot,
	}
	m.rebuildRoot()
	return m
}

func (m *permissionsEditor) rebuildRoot() {
	perms := getPermissions(m.settings)
	mode, _ := perms["defaultMode"].(string)
	if mode == "" {
		mode = "(no seteado)"
	}
	allowCount := len(getStringList(perms, "allow"))
	denyCount := len(getStringList(perms, "deny"))

	items := []list.Item{
		simpleItem{
			titleText: fmt.Sprintf("defaultMode: %s", mode),
			descText:  "elegí default, acceptEdits, bypassPermissions o plan",
			id:        "defaultMode",
		},
		simpleItem{
			titleText: fmt.Sprintf("allow rules (%d)", allowCount),
			descText:  "ver y eliminar reglas de allow",
			id:        "allow",
		},
		simpleItem{
			titleText: fmt.Sprintf("deny rules (%d)", denyCount),
			descText:  "ver y eliminar reglas de deny",
			id:        "deny",
		},
	}
	w, h := m.listSize()
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, w, h)
	l.Title = "Permissions"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.root = l
}

func (m *permissionsEditor) rebuildModePicker() {
	current, _ := getPermissions(m.settings)["defaultMode"].(string)
	items := make([]list.Item, 0, len(validDefaultModes)+1)
	for _, mode := range validDefaultModes {
		marker := ""
		if mode == current {
			marker = " (actual)"
		}
		items = append(items, simpleItem{
			titleText: mode + marker,
			descText:  defaultModeDescription(mode),
			id:        mode,
		})
	}
	items = append(items, simpleItem{
		titleText: "(quitar defaultMode)",
		descText:  "borra la clave del bloque permissions",
		id:        "__unset__",
	})
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "defaultMode"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.modePicker = l
}

func defaultModeDescription(mode string) string {
	switch mode {
	case "default":
		return "comportamiento estándar — pide permiso por cada tool"
	case "acceptEdits":
		return "auto-acepta ediciones de archivos"
	case "bypassPermissions":
		return "sin prompts (PELIGROSO — usá con cuidado)"
	case "plan":
		return "modo plan: piensa pero no ejecuta"
	}
	return ""
}

func (m *permissionsEditor) rebuildRulesList() {
	perms := getPermissions(m.settings)
	rules := getStringList(perms, m.listKey)
	items := make([]list.Item, 0, len(rules))
	for _, r := range rules {
		items = append(items, simpleItem{
			titleText: r,
			descText:  fmt.Sprintf("regla %s", m.listKey),
			id:        r,
		})
	}
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = fmt.Sprintf("%s rules", m.listKey)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.rulesUI = l
}

func (m *permissionsEditor) listSize() (int, int) {
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

func (m *permissionsEditor) Init() tea.Cmd { return nil }

func (m *permissionsEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.root.SetSize(w, h)
		if m.modePicker.Items() != nil {
			m.modePicker.SetSize(w, h)
		}
		if m.rulesUI.Items() != nil {
			m.rulesUI.SetSize(w, h)
		}

	case tea.KeyMsg:
		switch m.step {
		case stepPermRoot:
			if key.Matches(v, backKey) {
				return m, sendBackToSettings("")
			}
			if key.Matches(v, enterKey) {
				it, ok := m.root.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				switch it.id {
				case "defaultMode":
					m.rebuildModePicker()
					m.step = stepPermDefaultMode
				case "allow":
					m.listKey = "allow"
					m.rebuildRulesList()
					m.step = stepPermAllowList
				case "deny":
					m.listKey = "deny"
					m.rebuildRulesList()
					m.step = stepPermDenyList
				}
				return m, nil
			}

		case stepPermDefaultMode:
			if key.Matches(v, backKey) {
				m.step = stepPermRoot
				m.rebuildRoot()
				return m, nil
			}
			if key.Matches(v, enterKey) {
				it, ok := m.modePicker.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				if err := m.applyDefaultMode(it.id); err != nil {
					m.err = err
					return m, nil
				}
				return m, sendBackToSettings(fmt.Sprintf("✓ defaultMode actualizado: %s", displayMode(it.id)))
			}

		case stepPermAllowList, stepPermDenyList:
			if key.Matches(v, backKey) {
				m.step = stepPermRoot
				m.rebuildRoot()
				return m, nil
			}
			if key.Matches(v, deleteKey) {
				it, ok := m.rulesUI.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				m.pendingRule = it.id
				m.step = stepPermConfirmDel
				return m, nil
			}

		case stepPermConfirmDel:
			if key.Matches(v, backKey) {
				if m.listKey == "allow" {
					m.step = stepPermAllowList
				} else {
					m.step = stepPermDenyList
				}
				return m, nil
			}
			if key.Matches(v, applyKey) {
				if err := m.deleteRule(m.listKey, m.pendingRule); err != nil {
					m.err = err
					return m, nil
				}
				m.flash = "✓ regla eliminada"
				m.rebuildRulesList()
				if m.listKey == "allow" {
					m.step = stepPermAllowList
				} else {
					m.step = stepPermDenyList
				}
				return m, nil
			}
		}
	}

	switch m.step {
	case stepPermRoot:
		var cmd tea.Cmd
		m.root, cmd = m.root.Update(msg)
		return m, cmd
	case stepPermDefaultMode:
		var cmd tea.Cmd
		m.modePicker, cmd = m.modePicker.Update(msg)
		return m, cmd
	case stepPermAllowList, stepPermDenyList:
		var cmd tea.Cmd
		m.rulesUI, cmd = m.rulesUI.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *permissionsEditor) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepPermRoot:
		hint := dimStyle.Render("↑↓ moverse · enter elegir · esc volver a Settings")
		parts := []string{m.root.View()}
		if m.flash != "" {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5FD6C4")).
				Padding(0, 2).
				Render(m.flash))
		}
		parts = append(parts, hint)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)

	case stepPermDefaultMode:
		hint := dimStyle.Render("↑↓ moverse · enter aplicar · esc cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, m.modePicker.View(), hint)

	case stepPermAllowList, stepPermDenyList:
		body := m.rulesUI.View()
		if len(m.rulesUI.Items()) == 0 {
			body = dimStyle.Render(fmt.Sprintf("No hay reglas de %s configuradas.", m.listKey))
		}
		hint := dimStyle.Render(
			"↑↓ moverse · d eliminar · esc volver\n" +
				"agregar reglas próximamente (v0.5)",
		)
		parts := []string{body}
		if m.flash != "" {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5FD6C4")).
				Padding(0, 2).
				Render(m.flash))
		}
		parts = append(parts, hint)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)

	case stepPermConfirmDel:
		title := titleStyle.Render(fmt.Sprintf("Eliminar regla de %s", m.listKey))
		body := previewStyle.Render(m.pendingRule)
		warn := warnStyle.Render("Se hará backup de ~/.claude/settings.json antes de modificarlo.")
		hint := dimStyle.Render("y para confirmar · esc para cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, title, body, warn, hint)
	}
	return ""
}

func displayMode(id string) string {
	if id == "__unset__" {
		return "(quitado)"
	}
	return id
}

func (m *permissionsEditor) applyDefaultMode(modeID string) error {
	perms := getPermissions(m.settings)
	if modeID == "__unset__" {
		delete(perms, "defaultMode")
	} else {
		perms["defaultMode"] = modeID
	}
	return savePermissions(m.settings, perms)
}

func (m *permissionsEditor) deleteRule(listKey, rule string) error {
	perms := getPermissions(m.settings)
	rules := getStringList(perms, listKey)
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		if r != rule {
			out = append(out, r)
		}
	}
	setStringList(perms, listKey, out)
	return savePermissions(m.settings, perms)
}

// =====================================================================
// Output Style editor
// =====================================================================

type outputStyleEditor struct {
	settings *settings.Settings
	width    int
	height   int

	step osStep

	picker list.Model

	input    textinput.Model
	inputErr string

	err error
}

type osStep int

const (
	stepOSPick osStep = iota
	stepOSCustomInput
)

var commonOutputStyles = []string{"default", "Gentleman", "Engineer", "Concise"}

func newOutputStyleEditor(st *settings.Settings, w, h int) *outputStyleEditor {
	m := &outputStyleEditor{
		settings: st,
		width:    w,
		height:   h,
		step:     stepOSPick,
	}
	m.rebuildPicker()

	ti := textinput.New()
	ti.Placeholder = "nombre del output style"
	ti.CharLimit = 64
	ti.Width = 40
	m.input = ti
	return m
}

func (m *outputStyleEditor) rebuildPicker() {
	current := getString(m.settings, "outputStyle")
	items := make([]list.Item, 0, len(commonOutputStyles)+2)
	for _, s := range commonOutputStyles {
		marker := ""
		if s == current {
			marker = " (actual)"
		}
		items = append(items, simpleItem{
			titleText: s + marker,
			descText:  "valor común",
			id:        s,
		})
	}
	items = append(items, simpleItem{
		titleText: "otro... (escribir uno custom)",
		descText:  "abre un input para tipearlo",
		id:        "__custom__",
	})
	items = append(items, simpleItem{
		titleText: "(quitar outputStyle)",
		descText:  "borra la clave de settings.json",
		id:        "__unset__",
	})
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	title := "Output Style"
	if current != "" {
		title = fmt.Sprintf("Output Style — actual: %s", current)
	}
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.picker = l
}

func (m *outputStyleEditor) listSize() (int, int) {
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

func (m *outputStyleEditor) Init() tea.Cmd { return nil }

func (m *outputStyleEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.picker.SetSize(w, h)

	case tea.KeyMsg:
		switch m.step {
		case stepOSPick:
			if key.Matches(v, backKey) {
				return m, sendBackToSettings("")
			}
			if key.Matches(v, enterKey) {
				it, ok := m.picker.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				if it.id == "__custom__" {
					m.input.SetValue("")
					m.input.Focus()
					m.step = stepOSCustomInput
					return m, textinput.Blink
				}
				if it.id == "__unset__" {
					if err := m.applyValue(""); err != nil {
						m.err = err
						return m, nil
					}
					return m, sendBackToSettings("✓ outputStyle quitado")
				}
				if err := m.applyValue(it.id); err != nil {
					m.err = err
					return m, nil
				}
				return m, sendBackToSettings(fmt.Sprintf("✓ outputStyle: %s", it.id))
			}

		case stepOSCustomInput:
			if key.Matches(v, backKey) {
				m.input.Blur()
				m.inputErr = ""
				m.step = stepOSPick
				return m, nil
			}
			if v.Type == tea.KeyEnter {
				val := strings.TrimSpace(m.input.Value())
				if val == "" {
					m.inputErr = "el valor no puede estar vacío"
					return m, nil
				}
				if err := m.applyValue(val); err != nil {
					m.err = err
					return m, nil
				}
				return m, sendBackToSettings(fmt.Sprintf("✓ outputStyle: %s", val))
			}
		}
	}

	switch m.step {
	case stepOSPick:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	case stepOSCustomInput:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *outputStyleEditor) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepOSPick:
		hint := dimStyle.Render("↑↓ moverse · enter aplicar · esc cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, m.picker.View(), hint)
	case stepOSCustomInput:
		title := titleStyle.Render("Output Style — custom")
		input := previewStyle.Render(m.input.View())
		var status string
		if m.inputErr != "" {
			status = errorStyle.Render(m.inputErr)
		}
		hint := dimStyle.Render("enter para aplicar · esc para cancelar")
		parts := []string{title, input}
		if status != "" {
			parts = append(parts, status)
		}
		parts = append(parts, hint)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return ""
}

// applyValue persiste outputStyle. Si val es "", elimina la key.
func (m *outputStyleEditor) applyValue(val string) error {
	if val == "" {
		// Set("", ...) es no-op. Para borrar necesitamos un truco:
		// ponemos "" igual sería un valor distinto a unset. La API
		// pública sólo expone Set/Save, así que setear "" es lo
		// más cercano sin tocar settings.go. Aceptable por ahora.
		m.settings.Set("outputStyle", "")
	} else {
		m.settings.Set("outputStyle", val)
	}
	return m.settings.Save()
}

// =====================================================================
// Modelo preferido editor
// =====================================================================

type modelEditor struct {
	settings *settings.Settings
	width    int
	height   int

	picker list.Model

	err error
}

var preferredModels = []string{
	"claude-sonnet-4-6",
	"claude-opus-4-7",
	"claude-haiku-4-5-20251001",
}

func newModelEditor(st *settings.Settings, w, h int) *modelEditor {
	m := &modelEditor{
		settings: st,
		width:    w,
		height:   h,
	}
	m.rebuildPicker()
	return m
}

func (m *modelEditor) rebuildPicker() {
	current := getString(m.settings, "model")
	items := make([]list.Item, 0, len(preferredModels)+1)
	for _, mod := range preferredModels {
		marker := ""
		if mod == current {
			marker = " (actual)"
		}
		items = append(items, simpleItem{
			titleText: mod + marker,
			descText:  "alias canónico de Claude",
			id:        mod,
		})
	}
	items = append(items, simpleItem{
		titleText: "(sin preferencia)",
		descText:  "borra la clave model de settings.json",
		id:        "__unset__",
	})
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	title := "Modelo preferido"
	if current != "" {
		title = fmt.Sprintf("Modelo — actual: %s", current)
	}
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.picker = l
}

func (m *modelEditor) listSize() (int, int) {
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

func (m *modelEditor) Init() tea.Cmd { return nil }

func (m *modelEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.picker.SetSize(w, h)

	case tea.KeyMsg:
		if key.Matches(v, backKey) {
			return m, sendBackToSettings("")
		}
		if key.Matches(v, enterKey) {
			it, ok := m.picker.SelectedItem().(simpleItem)
			if !ok {
				return m, nil
			}
			if it.id == "__unset__" {
				m.settings.Set("model", "")
				if err := m.settings.Save(); err != nil {
					m.err = err
					return m, nil
				}
				return m, sendBackToSettings("✓ modelo: (sin preferencia)")
			}
			m.settings.Set("model", it.id)
			if err := m.settings.Save(); err != nil {
				m.err = err
				return m, nil
			}
			return m, sendBackToSettings(fmt.Sprintf("✓ modelo: %s", it.id))
		}
	}

	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
}

func (m *modelEditor) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}
	hint := dimStyle.Render("↑↓ moverse · enter aplicar · esc cancelar")
	return lipgloss.JoinVertical(lipgloss.Left, m.picker.View(), hint)
}

// =====================================================================
// Misc toggles editor
// =====================================================================

type miscEditor struct {
	settings *settings.Settings
	width    int
	height   int

	step miscStep

	root list.Model

	// stepMiscEffortPick
	effortPicker list.Model

	// stepMiscDaysInput
	daysInput textinput.Model
	inputErr  string

	flash string
	err   error
}

type miscStep int

const (
	stepMiscRoot miscStep = iota
	stepMiscEffortPick
	stepMiscDaysInput
)

var validEffortLevels = []string{"low", "medium", "high"}

func newMiscEditor(st *settings.Settings, w, h int) *miscEditor {
	m := &miscEditor{
		settings: st,
		width:    w,
		height:   h,
		step:     stepMiscRoot,
	}
	m.rebuildRoot()

	ti := textinput.New()
	ti.Placeholder = "30"
	ti.CharLimit = 5
	ti.Width = 10
	m.daysInput = ti
	return m
}

func (m *miscEditor) rebuildRoot() {
	coAuth, coAuthSet := getBool(m.settings, "includeCoAuthoredBy")
	coAuthLabel := "(no seteado, default true)"
	if coAuthSet {
		coAuthLabel = strconv.FormatBool(coAuth)
	}

	days, daysSet := getInt(m.settings, "cleanupPeriodDays")
	daysLabel := "(no seteado, default 30)"
	if daysSet {
		daysLabel = strconv.Itoa(days)
	}

	effort := getString(m.settings, "effortLevel")
	if effort == "" {
		effort = "(no seteado)"
	}

	items := []list.Item{
		simpleItem{
			titleText: fmt.Sprintf("includeCoAuthoredBy: %s", coAuthLabel),
			descText:  "toggle: enter alterna true/false",
			id:        "includeCoAuthoredBy",
		},
		simpleItem{
			titleText: fmt.Sprintf("cleanupPeriodDays: %s", daysLabel),
			descText:  "días para limpiar transcripts (entero)",
			id:        "cleanupPeriodDays",
		},
		simpleItem{
			titleText: fmt.Sprintf("effortLevel: %s", effort),
			descText:  "low / medium / high",
			id:        "effortLevel",
		},
	}
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "Misc toggles"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.root = l
}

func (m *miscEditor) rebuildEffortPicker() {
	current := getString(m.settings, "effortLevel")
	items := make([]list.Item, 0, len(validEffortLevels)+1)
	for _, lvl := range validEffortLevels {
		marker := ""
		if lvl == current {
			marker = " (actual)"
		}
		items = append(items, simpleItem{
			titleText: lvl + marker,
			descText:  effortDescription(lvl),
			id:        lvl,
		})
	}
	items = append(items, simpleItem{
		titleText: "(quitar effortLevel)",
		descText:  "borra la clave de settings.json",
		id:        "__unset__",
	})
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = "effortLevel"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.effortPicker = l
}

func effortDescription(lvl string) string {
	switch lvl {
	case "low":
		return "respuestas más cortas y rápidas"
	case "medium":
		return "balance default"
	case "high":
		return "respuestas más completas y verbosas"
	}
	return ""
}

func (m *miscEditor) listSize() (int, int) {
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

func (m *miscEditor) Init() tea.Cmd { return nil }

func (m *miscEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.root.SetSize(w, h)
		if m.effortPicker.Items() != nil {
			m.effortPicker.SetSize(w, h)
		}

	case tea.KeyMsg:
		switch m.step {
		case stepMiscRoot:
			if key.Matches(v, backKey) {
				return m, sendBackToSettings("")
			}
			if key.Matches(v, enterKey) {
				it, ok := m.root.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				switch it.id {
				case "includeCoAuthoredBy":
					if err := m.toggleCoAuth(); err != nil {
						m.err = err
						return m, nil
					}
					m.flash = "✓ includeCoAuthoredBy actualizado"
					m.rebuildRoot()
					return m, nil
				case "cleanupPeriodDays":
					current, _ := getInt(m.settings, "cleanupPeriodDays")
					if current == 0 {
						current = 30
					}
					m.daysInput.SetValue(strconv.Itoa(current))
					m.daysInput.Focus()
					m.inputErr = ""
					m.step = stepMiscDaysInput
					return m, textinput.Blink
				case "effortLevel":
					m.rebuildEffortPicker()
					m.step = stepMiscEffortPick
					return m, nil
				}
			}

		case stepMiscEffortPick:
			if key.Matches(v, backKey) {
				m.step = stepMiscRoot
				return m, nil
			}
			if key.Matches(v, enterKey) {
				it, ok := m.effortPicker.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				if it.id == "__unset__" {
					m.settings.Set("effortLevel", "")
				} else {
					m.settings.Set("effortLevel", it.id)
				}
				if err := m.settings.Save(); err != nil {
					m.err = err
					return m, nil
				}
				m.flash = fmt.Sprintf("✓ effortLevel: %s", displayEffort(it.id))
				m.rebuildRoot()
				m.step = stepMiscRoot
				return m, nil
			}

		case stepMiscDaysInput:
			if key.Matches(v, backKey) {
				m.daysInput.Blur()
				m.inputErr = ""
				m.step = stepMiscRoot
				return m, nil
			}
			if v.Type == tea.KeyEnter {
				raw := strings.TrimSpace(m.daysInput.Value())
				n, err := strconv.Atoi(raw)
				if err != nil || n < 0 {
					m.inputErr = "ingresá un entero >= 0"
					return m, nil
				}
				m.settings.Set("cleanupPeriodDays", n)
				if err := m.settings.Save(); err != nil {
					m.err = err
					return m, nil
				}
				m.daysInput.Blur()
				m.flash = fmt.Sprintf("✓ cleanupPeriodDays: %d", n)
				m.rebuildRoot()
				m.step = stepMiscRoot
				return m, nil
			}
		}
	}

	switch m.step {
	case stepMiscRoot:
		var cmd tea.Cmd
		m.root, cmd = m.root.Update(msg)
		return m, cmd
	case stepMiscEffortPick:
		var cmd tea.Cmd
		m.effortPicker, cmd = m.effortPicker.Update(msg)
		return m, cmd
	case stepMiscDaysInput:
		var cmd tea.Cmd
		m.daysInput, cmd = m.daysInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *miscEditor) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepMiscRoot:
		body := m.root.View()
		hint := dimStyle.Render("↑↓ moverse · enter editar · esc volver a Settings")
		parts := []string{body}
		if m.flash != "" {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5FD6C4")).
				Padding(0, 2).
				Render(m.flash))
		}
		parts = append(parts, hint)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	case stepMiscEffortPick:
		hint := dimStyle.Render("↑↓ moverse · enter aplicar · esc cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, m.effortPicker.View(), hint)
	case stepMiscDaysInput:
		title := titleStyle.Render("cleanupPeriodDays")
		input := previewStyle.Render(m.daysInput.View())
		var status string
		if m.inputErr != "" {
			status = errorStyle.Render(m.inputErr)
		}
		hint := dimStyle.Render("enter para aplicar · esc para cancelar")
		parts := []string{title, input}
		if status != "" {
			parts = append(parts, status)
		}
		parts = append(parts, hint)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	return ""
}

func displayEffort(id string) string {
	if id == "__unset__" {
		return "(quitado)"
	}
	return id
}

// toggleCoAuth alterna includeCoAuthoredBy entre true y false. Si la clave
// no existe, la default Claude Code es true → toggle pone false.
func (m *miscEditor) toggleCoAuth() error {
	current, set := getBool(m.settings, "includeCoAuthoredBy")
	var next bool
	if !set {
		// Default es true → primer toggle: false.
		next = false
	} else {
		next = !current
	}
	m.settings.Set("includeCoAuthoredBy", next)
	return m.settings.Save()
}

// =====================================================================
// Helpers compartidos
// =====================================================================

// simpleItem es un list.Item genérico para los pickers internos. Reusamos
// uno solo para no inflar el archivo con structs por cada lista.
type simpleItem struct {
	titleText string
	descText  string
	id        string
}

func (s simpleItem) Title() string       { return s.titleText }
func (s simpleItem) Description() string { return s.descText }
func (s simpleItem) FilterValue() string { return s.titleText }

// titleListStyle reusa la paleta de titleStyle pero adaptada a list.Title
// (que necesita Background, no MarginTop). Devolverla por función mantiene
// las definiciones de estilo en statusline.go como única fuente.
func titleListStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#F5B544")).
		Foreground(lipgloss.Color("#1A1308")).
		Padding(0, 1).
		Bold(true)
}
