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
	sectionEnv         settingsSection = "env"
	sectionMCPServers  settingsSection = "mcpServers"
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
		settingsRootItem{
			id:    sectionEnv,
			title: "Variables de entorno",
			desc:  "bloque env: pares KEY=VALUE",
		},
		settingsRootItem{
			id:    sectionMCPServers,
			title: "MCP Servers",
			desc:  "servidores MCP configurados (command, args, env)",
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
	case sectionEnv:
		return newEnvEditor(st, w, h)
	case sectionMCPServers:
		return newMCPServersEditor(st, w, h)
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

// savePermissions reescribe el bloque permissions y persiste. Si el bloque
// queda vacío, borra la clave para no dejar `"permissions": {}` colgado.
func savePermissions(st *settings.Settings, perms map[string]any) error {
	if len(perms) == 0 {
		st.Delete("permissions")
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

	// stepPermAddRule
	addInput textinput.Model
	addErr   string

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
	stepPermAddRule
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
			if key.Matches(v, addKey) {
				m.openAddRule()
				return m, textinput.Blink
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

		case stepPermAddRule:
			if key.Matches(v, backKey) {
				m.addInput.Blur()
				m.addErr = ""
				if m.listKey == "allow" {
					m.step = stepPermAllowList
				} else {
					m.step = stepPermDenyList
				}
				return m, nil
			}
			if key.Matches(v, formSaveKey) || v.Type == tea.KeyEnter {
				if err := m.saveAddRule(); err != nil {
					m.addErr = err.Error()
					return m, nil
				}
				m.flash = fmt.Sprintf("✓ regla %s agregada", m.listKey)
				m.addInput.Blur()
				m.addErr = ""
				m.rebuildRulesList()
				if m.listKey == "allow" {
					m.step = stepPermAllowList
				} else {
					m.step = stepPermDenyList
				}
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
	case stepPermAddRule:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
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
			"↑↓ moverse · a agregar · d eliminar · esc volver",
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

	case stepPermAddRule:
		title := titleStyle.Render(fmt.Sprintf("Agregar regla a %s", m.listKey))
		input := previewStyle.Render(m.addInput.View())
		examples := dimStyle.Render(
			"Ejemplos: Bash(npm install) · Read(*.env) · Edit(src/**) · WebFetch(*)",
		)
		parts := []string{title, input, examples}
		if m.addErr != "" {
			parts = append(parts, errorStyle.Render(m.addErr))
		}
		parts = append(parts, dimStyle.Render("enter o ctrl+s guardar · esc cancelar"))
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

// openAddRule prepara el textinput para tipear una regla nueva.
func (m *permissionsEditor) openAddRule() {
	m.addInput = newFormInput("", "Bash(npm install)")
	m.addInput.Focus()
	m.addErr = ""
	m.step = stepPermAddRule
}

// saveAddRule valida y persiste la regla nueva. No vacío y sin duplicados.
func (m *permissionsEditor) saveAddRule() error {
	rule := strings.TrimSpace(m.addInput.Value())
	if rule == "" {
		return fmt.Errorf("la regla no puede estar vacía")
	}
	perms := getPermissions(m.settings)
	rules := getStringList(perms, m.listKey)
	for _, r := range rules {
		if r == rule {
			return fmt.Errorf("la regla ya existe en %s", m.listKey)
		}
	}
	rules = append(rules, rule)
	setStringList(perms, m.listKey, rules)
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
		m.settings.Delete("outputStyle")
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
				m.settings.Delete("model")
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
					m.settings.Delete("effortLevel")
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

// truncate corta un string a max chars y agrega "…" si fue cortado.
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// =====================================================================
// Env editor — bloque "env" como pares KEY=VALUE
// =====================================================================

type envEditor struct {
	settings *settings.Settings
	width    int
	height   int

	step envStep

	list list.Model

	// stepEnvForm: dos textinputs (KEY, VALUE)
	keyInput   textinput.Model
	valueInput textinput.Model
	focusIdx   int // 0 = KEY, 1 = VALUE
	formErr    string
	editingKey string // si != "" estamos editando esa key. Vacío → add.
	originalKey string // key original cuando se editó (para detectar rename)

	// stepEnvConfirmDel
	pendingKey string

	flash string
	err   error
}

type envStep int

const (
	stepEnvList envStep = iota
	stepEnvForm
	stepEnvConfirmDel
)

func newEnvEditor(st *settings.Settings, w, h int) *envEditor {
	m := &envEditor{
		settings: st,
		width:    w,
		height:   h,
		step:     stepEnvList,
	}
	m.rebuildList()
	return m
}

// getEnv devuelve el bloque env como map de string→string. Tolera valores
// no-string (los descarta) — Claude Code documenta env como string→string.
func getEnv(st *settings.Settings) map[string]string {
	out := map[string]string{}
	raw, ok := st.Get("env").(map[string]any)
	if !ok {
		return out
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// saveEnv persiste el bloque env. Si queda vacío, lo borra.
func saveEnv(st *settings.Settings, env map[string]string) error {
	if len(env) == 0 {
		st.Delete("env")
		return st.Save()
	}
	out := make(map[string]any, len(env))
	for k, v := range env {
		out[k] = v
	}
	st.Set("env", out)
	return st.Save()
}

// sortedKeys devuelve las keys ordenadas alfabéticamente para listas estables.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func (m *envEditor) rebuildList() {
	env := getEnv(m.settings)
	keys := sortedKeys(env)
	items := make([]list.Item, 0, len(keys))
	for _, k := range keys {
		items = append(items, simpleItem{
			titleText: k,
			descText:  truncate(env[k], 60),
			id:        k,
		})
	}
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = fmt.Sprintf("Variables de entorno (%d)", len(keys))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.list = l
}

func (m *envEditor) listSize() (int, int) {
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

func (m *envEditor) Init() tea.Cmd { return nil }

func (m *envEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.list.SetSize(w, h)

	case tea.KeyMsg:
		switch m.step {
		case stepEnvList:
			if key.Matches(v, backKey) {
				return m, sendBackToSettings("")
			}
			if key.Matches(v, addKey) {
				m.openForm("", "")
				return m, textinput.Blink
			}
			if key.Matches(v, editKey) {
				it, ok := m.list.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				env := getEnv(m.settings)
				m.openForm(it.id, env[it.id])
				return m, textinput.Blink
			}
			if key.Matches(v, deleteKey) {
				it, ok := m.list.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				m.pendingKey = it.id
				m.step = stepEnvConfirmDel
				return m, nil
			}

		case stepEnvForm:
			if key.Matches(v, backKey) {
				m.closeForm()
				return m, nil
			}
			if key.Matches(v, formSaveKey) {
				if err := m.saveForm(); err != nil {
					m.formErr = err.Error()
					return m, nil
				}
				if m.editingKey != "" {
					m.flash = fmt.Sprintf("✓ env %s actualizada", m.keyInput.Value())
				} else {
					m.flash = fmt.Sprintf("✓ env %s agregada", m.keyInput.Value())
				}
				m.closeForm()
				m.rebuildList()
				return m, nil
			}
			if key.Matches(v, formNextKey) {
				m.toggleFocus(true)
				return m, nil
			}
			if key.Matches(v, formPrevKey) {
				m.toggleFocus(false)
				return m, nil
			}
			var cmd tea.Cmd
			if m.focusIdx == 0 {
				m.keyInput, cmd = m.keyInput.Update(msg)
			} else {
				m.valueInput, cmd = m.valueInput.Update(msg)
			}
			return m, cmd

		case stepEnvConfirmDel:
			if key.Matches(v, backKey) {
				m.step = stepEnvList
				return m, nil
			}
			if key.Matches(v, applyKey) {
				if err := m.deleteCurrent(); err != nil {
					m.err = err
					return m, nil
				}
				m.flash = fmt.Sprintf("✓ env %s eliminada", m.pendingKey)
				m.rebuildList()
				m.step = stepEnvList
				return m, nil
			}
		}
	}

	if m.step == stepEnvList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *envEditor) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepEnvList:
		body := m.list.View()
		if len(m.list.Items()) == 0 {
			body = dimStyle.Render("No hay variables de entorno configuradas.")
		}
		hint := dimStyle.Render(
			"↑↓ moverse · a agregar · e editar · d eliminar · esc volver",
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

	case stepEnvForm:
		titleText := "Agregar variable de entorno"
		if m.editingKey != "" {
			titleText = fmt.Sprintf("Editar variable de entorno (%s)", m.editingKey)
		}
		title := titleStyle.Render(titleText)
		keyLabel := "KEY"
		valLabel := "VALUE"
		if m.focusIdx == 0 {
			keyLabel = "▸ " + keyLabel
		} else {
			valLabel = "▸ " + valLabel
		}
		keyBlock := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F5B544")).Bold(true).Padding(0, 2).Render(keyLabel),
			lipgloss.NewStyle().Padding(0, 2).Render(m.keyInput.View()),
			dimStyle.Render("sin espacios ni '='. Ej: ANTHROPIC_API_KEY"),
		)
		valBlock := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#F5B544")).Bold(true).Padding(0, 2).Render(valLabel),
			lipgloss.NewStyle().Padding(0, 2).Render(m.valueInput.View()),
			dimStyle.Render("cualquier string"),
		)
		parts := []string{title, keyBlock, valBlock}
		if m.formErr != "" {
			parts = append(parts, errorStyle.Render(m.formErr))
		}
		parts = append(parts, dimStyle.Render("tab cambiar campo · ctrl+s guardar · esc cancelar"))
		return lipgloss.JoinVertical(lipgloss.Left, parts...)

	case stepEnvConfirmDel:
		title := titleStyle.Render(fmt.Sprintf("Eliminar variable %s", m.pendingKey))
		warn := warnStyle.Render("Se hará backup de ~/.claude/settings.json antes de modificarlo.")
		hint := dimStyle.Render("y para confirmar · esc para cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, title, warn, hint)
	}
	return ""
}

func (m *envEditor) openForm(k, v string) {
	m.keyInput = newFormInput(k, "ANTHROPIC_API_KEY")
	m.valueInput = newFormInput(v, "valor")
	m.keyInput.Focus()
	m.focusIdx = 0
	m.formErr = ""
	m.editingKey = k
	m.originalKey = k
	m.step = stepEnvForm
}

func (m *envEditor) closeForm() {
	m.keyInput.Blur()
	m.valueInput.Blur()
	m.formErr = ""
	m.editingKey = ""
	m.originalKey = ""
	m.step = stepEnvList
}

func (m *envEditor) toggleFocus(forward bool) {
	_ = forward
	if m.focusIdx == 0 {
		m.focusIdx = 1
		m.keyInput.Blur()
		m.valueInput.Focus()
	} else {
		m.focusIdx = 0
		m.valueInput.Blur()
		m.keyInput.Focus()
	}
}

// validateEnvKey rechaza vacío, espacios, '=' o tabs.
func validateEnvKey(k string) error {
	if k == "" {
		return fmt.Errorf("la key no puede estar vacía")
	}
	if strings.ContainsAny(k, "= \t\n\r") {
		return fmt.Errorf("la key no puede contener '=' ni espacios")
	}
	return nil
}

func (m *envEditor) saveForm() error {
	k := strings.TrimSpace(m.keyInput.Value())
	v := m.valueInput.Value()
	if err := validateEnvKey(k); err != nil {
		return err
	}
	env := getEnv(m.settings)

	// Detectar duplicado: si la key elegida existe pero NO es la que estamos
	// editando (es decir, rename hacia una key existente), abortar.
	if _, exists := env[k]; exists && k != m.originalKey {
		return fmt.Errorf("la key %s ya existe", k)
	}

	// Si estábamos editando y se renombró la key, borrar la vieja.
	if m.originalKey != "" && m.originalKey != k {
		delete(env, m.originalKey)
	}
	env[k] = v
	return saveEnv(m.settings, env)
}

func (m *envEditor) deleteCurrent() error {
	env := getEnv(m.settings)
	delete(env, m.pendingKey)
	return saveEnv(m.settings, env)
}

// =====================================================================
// MCP Servers editor — bloque "mcpServers"
// =====================================================================

type mcpServersEditor struct {
	settings *settings.Settings
	width    int
	height   int

	step mcpStep

	list list.Model

	// formulario
	nameInput    textinput.Model
	commandInput textinput.Model
	argsInput    textinput.Model
	envInput     textinput.Model
	focusIdx     int // 0..3
	formErr      string
	editingName  string // != "" si editando
	originalName string

	// confirm del
	pendingName string

	flash string
	err   error
}

type mcpStep int

const (
	stepMCPList mcpStep = iota
	stepMCPForm
	stepMCPConfirmDel
)

func newMCPServersEditor(st *settings.Settings, w, h int) *mcpServersEditor {
	m := &mcpServersEditor{
		settings: st,
		width:    w,
		height:   h,
		step:     stepMCPList,
	}
	m.rebuildList()
	return m
}

// mcpServer es la representación interna de un server. Mantenemos los
// tipos del JSON original (env como map[string]any → string→string filtrado).
type mcpServer struct {
	Command string
	Args    []string
	Env     map[string]string
}

func getMCPServers(st *settings.Settings) map[string]mcpServer {
	out := map[string]mcpServer{}
	raw, ok := st.Get("mcpServers").(map[string]any)
	if !ok {
		return out
	}
	for name, val := range raw {
		obj, ok := val.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := obj["command"].(string)
		args := []string{}
		if rawArgs, ok := obj["args"].([]any); ok {
			for _, a := range rawArgs {
				if s, ok := a.(string); ok {
					args = append(args, s)
				}
			}
		}
		env := map[string]string{}
		if rawEnv, ok := obj["env"].(map[string]any); ok {
			for k, v := range rawEnv {
				if s, ok := v.(string); ok {
					env[k] = s
				}
			}
		}
		out[name] = mcpServer{Command: cmd, Args: args, Env: env}
	}
	return out
}

// saveMCPServers reescribe el bloque mcpServers. Si queda vacío, lo borra.
// Para cada server omite "args" o "env" si están vacíos para no inflar el JSON.
func saveMCPServers(st *settings.Settings, servers map[string]mcpServer) error {
	if len(servers) == 0 {
		st.Delete("mcpServers")
		return st.Save()
	}
	out := map[string]any{}
	for name, s := range servers {
		entry := map[string]any{
			"command": s.Command,
		}
		if len(s.Args) > 0 {
			args := make([]any, len(s.Args))
			for i, a := range s.Args {
				args[i] = a
			}
			entry["args"] = args
		}
		if len(s.Env) > 0 {
			env := map[string]any{}
			for k, v := range s.Env {
				env[k] = v
			}
			entry["env"] = env
		}
		out[name] = entry
	}
	st.Set("mcpServers", out)
	return st.Save()
}

func (m *mcpServersEditor) rebuildList() {
	servers := getMCPServers(m.settings)
	names := make([]string, 0, len(servers))
	for n := range servers {
		names = append(names, n)
	}
	sortStrings(names)
	items := make([]list.Item, 0, len(names))
	for _, n := range names {
		s := servers[n]
		desc := truncate(s.Command, 50)
		if len(s.Args) > 0 {
			desc = truncate(s.Command+" "+strings.Join(s.Args, " "), 60)
		}
		items = append(items, simpleItem{
			titleText: n,
			descText:  desc,
			id:        n,
		})
	}
	w, h := m.listSize()
	l := list.New(items, list.NewDefaultDelegate(), w, h)
	l.Title = fmt.Sprintf("MCP Servers (%d)", len(names))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleListStyle()
	m.list = l
}

func (m *mcpServersEditor) listSize() (int, int) {
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

func (m *mcpServersEditor) Init() tea.Cmd { return nil }

func (m *mcpServersEditor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		w, h := m.listSize()
		m.list.SetSize(w, h)

	case tea.KeyMsg:
		switch m.step {
		case stepMCPList:
			if key.Matches(v, backKey) {
				return m, sendBackToSettings("")
			}
			if key.Matches(v, addKey) {
				m.openForm("", mcpServer{})
				return m, textinput.Blink
			}
			if key.Matches(v, editKey) {
				it, ok := m.list.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				servers := getMCPServers(m.settings)
				m.openForm(it.id, servers[it.id])
				return m, textinput.Blink
			}
			if key.Matches(v, deleteKey) {
				it, ok := m.list.SelectedItem().(simpleItem)
				if !ok {
					return m, nil
				}
				m.pendingName = it.id
				m.step = stepMCPConfirmDel
				return m, nil
			}

		case stepMCPForm:
			if key.Matches(v, backKey) {
				m.closeForm()
				return m, nil
			}
			if key.Matches(v, formSaveKey) {
				if err := m.saveForm(); err != nil {
					m.formErr = err.Error()
					return m, nil
				}
				if m.editingName != "" {
					m.flash = fmt.Sprintf("✓ MCP server %s actualizado", m.nameInput.Value())
				} else {
					m.flash = fmt.Sprintf("✓ MCP server %s agregado", m.nameInput.Value())
				}
				m.closeForm()
				m.rebuildList()
				return m, nil
			}
			if key.Matches(v, formNextKey) {
				m.cycleFocus(1)
				return m, nil
			}
			if key.Matches(v, formPrevKey) {
				m.cycleFocus(-1)
				return m, nil
			}
			var cmd tea.Cmd
			switch m.focusIdx {
			case 0:
				m.nameInput, cmd = m.nameInput.Update(msg)
			case 1:
				m.commandInput, cmd = m.commandInput.Update(msg)
			case 2:
				m.argsInput, cmd = m.argsInput.Update(msg)
			case 3:
				m.envInput, cmd = m.envInput.Update(msg)
			}
			return m, cmd

		case stepMCPConfirmDel:
			if key.Matches(v, backKey) {
				m.step = stepMCPList
				return m, nil
			}
			if key.Matches(v, applyKey) {
				if err := m.deleteCurrent(); err != nil {
					m.err = err
					return m, nil
				}
				m.flash = fmt.Sprintf("✓ MCP server %s eliminado", m.pendingName)
				m.rebuildList()
				m.step = stepMCPList
				return m, nil
			}
		}
	}

	if m.step == stepMCPList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *mcpServersEditor) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nesc para volver", m.err))
	}

	switch m.step {
	case stepMCPList:
		body := m.list.View()
		if len(m.list.Items()) == 0 {
			body = dimStyle.Render("No hay MCP servers configurados.")
		}
		hint := dimStyle.Render(
			"↑↓ moverse · a agregar · e editar · d eliminar · esc volver",
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

	case stepMCPForm:
		titleText := "Agregar MCP server"
		if m.editingName != "" {
			titleText = fmt.Sprintf("Editar MCP server (%s)", m.editingName)
		}
		title := titleStyle.Render(titleText)

		labels := []string{"NAME", "COMMAND", "ARGS", "ENV"}
		labels[m.focusIdx] = "▸ " + labels[m.focusIdx]
		hints := []string{
			"identificador del server (sin espacios). Ej: github",
			"binario a ejecutar. Ej: npx, /usr/local/bin/foo",
			"separados por espacio. Ej: -y @modelcontextprotocol/server-github",
			"opcional. KEY1=VAL1 KEY2=VAL2",
		}
		inputs := []textinput.Model{m.nameInput, m.commandInput, m.argsInput, m.envInput}
		blocks := make([]string, 0, len(labels))
		for i, label := range labels {
			block := lipgloss.JoinVertical(
				lipgloss.Left,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#F5B544")).Bold(true).Padding(0, 2).Render(label),
				lipgloss.NewStyle().Padding(0, 2).Render(inputs[i].View()),
				dimStyle.Render(hints[i]),
			)
			blocks = append(blocks, block)
		}

		parts := []string{title}
		parts = append(parts, blocks...)
		if m.formErr != "" {
			parts = append(parts, errorStyle.Render(m.formErr))
		}
		parts = append(parts, dimStyle.Render("tab/shift+tab cambiar campo · ctrl+s guardar · esc cancelar"))
		return lipgloss.JoinVertical(lipgloss.Left, parts...)

	case stepMCPConfirmDel:
		title := titleStyle.Render(fmt.Sprintf("Eliminar MCP server %s", m.pendingName))
		warn := warnStyle.Render("Se hará backup de ~/.claude/settings.json antes de modificarlo.")
		hint := dimStyle.Render("y para confirmar · esc para cancelar")
		return lipgloss.JoinVertical(lipgloss.Left, title, warn, hint)
	}
	return ""
}

func (m *mcpServersEditor) openForm(name string, s mcpServer) {
	m.nameInput = newFormInput(name, "github")
	m.commandInput = newFormInput(s.Command, "npx")
	m.argsInput = newFormInput(strings.Join(s.Args, " "), "-y @modelcontextprotocol/server-github")
	m.envInput = newFormInput(formatEnvLine(s.Env), "GITHUB_TOKEN=ghp_xxx")
	m.nameInput.Focus()
	m.focusIdx = 0
	m.formErr = ""
	m.editingName = name
	m.originalName = name
	m.step = stepMCPForm
}

func (m *mcpServersEditor) closeForm() {
	m.nameInput.Blur()
	m.commandInput.Blur()
	m.argsInput.Blur()
	m.envInput.Blur()
	m.formErr = ""
	m.editingName = ""
	m.originalName = ""
	m.step = stepMCPList
}

// cycleFocus mueve el foco entre los 4 inputs en forma circular.
func (m *mcpServersEditor) cycleFocus(delta int) {
	inputs := []*textinput.Model{&m.nameInput, &m.commandInput, &m.argsInput, &m.envInput}
	inputs[m.focusIdx].Blur()
	m.focusIdx = (m.focusIdx + delta + len(inputs)) % len(inputs)
	inputs[m.focusIdx].Focus()
}

func (m *mcpServersEditor) saveForm() error {
	name := strings.TrimSpace(m.nameInput.Value())
	cmd := strings.TrimSpace(m.commandInput.Value())
	if name == "" {
		return fmt.Errorf("el nombre no puede estar vacío")
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return fmt.Errorf("el nombre no puede contener espacios")
	}
	if cmd == "" {
		return fmt.Errorf("el command no puede estar vacío")
	}

	args := splitArgs(m.argsInput.Value())
	env, err := parseEnvLine(m.envInput.Value())
	if err != nil {
		return err
	}

	servers := getMCPServers(m.settings)
	// Si renombró hacia uno existente, abortar.
	if _, exists := servers[name]; exists && name != m.originalName {
		return fmt.Errorf("ya existe un server llamado %s", name)
	}
	// Renombre: borrar el viejo.
	if m.originalName != "" && m.originalName != name {
		delete(servers, m.originalName)
	}
	servers[name] = mcpServer{
		Command: cmd,
		Args:    args,
		Env:     env,
	}
	return saveMCPServers(m.settings, servers)
}

func (m *mcpServersEditor) deleteCurrent() error {
	servers := getMCPServers(m.settings)
	delete(servers, m.pendingName)
	return saveMCPServers(m.settings, servers)
}

// splitArgs parte el string por whitespace. No soporta quoting (los args con
// espacios no son representables en este formato simple — para casos así, el
// usuario puede editar settings.json a mano).
func splitArgs(raw string) []string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// parseEnvLine parsea "KEY1=VAL1 KEY2=VAL2" a un map. Tolera vacío. Falla si
// algún token no contiene '='.
func parseEnvLine(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	out := map[string]string{}
	for _, tok := range strings.Fields(raw) {
		idx := strings.IndexByte(tok, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("ENV mal formado: %q (esperado KEY=VALUE)", tok)
		}
		k := tok[:idx]
		v := tok[idx+1:]
		if err := validateEnvKey(k); err != nil {
			return nil, fmt.Errorf("ENV: %w", err)
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// formatEnvLine arma "K1=V1 K2=V2" para precarga del input al editar.
func formatEnvLine(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := sortedKeys(env)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+env[k])
	}
	return strings.Join(parts, " ")
}

// sortStrings ordena en place. Wrapper para no importar sort sólo por esto.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
