package ui

import (
	"path/filepath"
	"testing"

	"github.com/CogniDevAI/monocle/internal/settings"
)

// TestPermissionsEditor_DefaultModeAndDelete cubre el camino feliz:
// arrancar con permissions vacío, setear defaultMode, agregar reglas a
// allow vía Set directo y eliminarlas con deleteRule.
func TestPermissionsEditor_DefaultModeAndDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	st, err := settings.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Sembrar reglas de allow para poder eliminar después.
	st.Set("permissions", map[string]any{
		"allow": []any{"Bash(ls)", "Bash(pwd)"},
	})
	if err := st.Save(); err != nil {
		t.Fatalf("save seed: %v", err)
	}

	ed := newPermissionsEditor(st, 80, 24)

	// applyDefaultMode con un modo válido.
	if err := ed.applyDefaultMode("acceptEdits"); err != nil {
		t.Fatalf("applyDefaultMode: %v", err)
	}
	perms := getPermissions(st)
	if got, _ := perms["defaultMode"].(string); got != "acceptEdits" {
		t.Errorf("defaultMode = %q, want acceptEdits", got)
	}

	// Borrar la primera regla y verificar que sobrevive solo la otra.
	if err := ed.deleteRule("allow", "Bash(ls)"); err != nil {
		t.Fatalf("deleteRule: %v", err)
	}
	perms = getPermissions(st)
	rules := getStringList(perms, "allow")
	if len(rules) != 1 || rules[0] != "Bash(pwd)" {
		t.Errorf("rules = %v, want [Bash(pwd)]", rules)
	}

	// Quitar defaultMode y verificar que la clave queda fuera del map.
	if err := ed.applyDefaultMode("__unset__"); err != nil {
		t.Fatalf("applyDefaultMode unset: %v", err)
	}
	perms = getPermissions(st)
	if _, present := perms["defaultMode"]; present {
		t.Errorf("defaultMode debería estar quitado, sigue presente")
	}
}

// TestMiscEditor_ToggleCoAuth verifica que la primera vez setea false (porque
// la default de Claude Code es true) y la siguiente alterna.
func TestMiscEditor_ToggleCoAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := settings.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ed := newMiscEditor(st, 80, 24)

	// Primer toggle desde unset → false.
	if err := ed.toggleCoAuth(); err != nil {
		t.Fatalf("toggleCoAuth 1: %v", err)
	}
	v, set := getBool(st, "includeCoAuthoredBy")
	if !set || v != false {
		t.Errorf("after first toggle: set=%v val=%v, want set=true val=false", set, v)
	}

	// Segundo toggle: false → true.
	if err := ed.toggleCoAuth(); err != nil {
		t.Fatalf("toggleCoAuth 2: %v", err)
	}
	v, set = getBool(st, "includeCoAuthoredBy")
	if !set || v != true {
		t.Errorf("after second toggle: set=%v val=%v, want set=true val=true", set, v)
	}
}

// TestPermissionsEditor_AddRule cubre el ADD: regla nueva, duplicada, y vacía.
func TestPermissionsEditor_AddRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := settings.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ed := newPermissionsEditor(st, 80, 24)
	ed.listKey = "allow"

	// Caso feliz: agregar una regla nueva.
	ed.openAddRule()
	ed.addInput.SetValue("Bash(npm install)")
	if err := ed.saveAddRule(); err != nil {
		t.Fatalf("saveAddRule 1: %v", err)
	}
	rules := getStringList(getPermissions(st), "allow")
	if len(rules) != 1 || rules[0] != "Bash(npm install)" {
		t.Errorf("rules = %v, want [Bash(npm install)]", rules)
	}

	// Duplicado: debería fallar.
	ed.addInput.SetValue("Bash(npm install)")
	if err := ed.saveAddRule(); err == nil {
		t.Error("saveAddRule duplicado debería fallar")
	}

	// Vacío: debería fallar.
	ed.addInput.SetValue("   ")
	if err := ed.saveAddRule(); err == nil {
		t.Error("saveAddRule vacío debería fallar")
	}
}

// TestEnvEditor_AddEditDelete cubre el flujo agregar → editar → borrar y que
// el bloque env quede borrado del map cuando se vacía.
func TestEnvEditor_AddEditDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := settings.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ed := newEnvEditor(st, 80, 24)

	// Agregar FOO=bar.
	ed.openForm("", "")
	ed.keyInput.SetValue("FOO")
	ed.valueInput.SetValue("bar")
	if err := ed.saveForm(); err != nil {
		t.Fatalf("saveForm add: %v", err)
	}
	if v := getEnv(st)["FOO"]; v != "bar" {
		t.Errorf("env[FOO] = %q, want bar", v)
	}

	// Validación: KEY con '=' → error.
	ed.openForm("", "")
	ed.keyInput.SetValue("BAD=KEY")
	ed.valueInput.SetValue("x")
	if err := ed.saveForm(); err == nil {
		t.Error("KEY con '=' debería fallar")
	}

	// Editar FOO → renombrarlo a FOO2 con valor nuevo.
	ed.openForm("FOO", "bar")
	ed.keyInput.SetValue("FOO2")
	ed.valueInput.SetValue("baz")
	if err := ed.saveForm(); err != nil {
		t.Fatalf("saveForm edit: %v", err)
	}
	env := getEnv(st)
	if _, ok := env["FOO"]; ok {
		t.Error("FOO no se borró al renombrar")
	}
	if env["FOO2"] != "baz" {
		t.Errorf("env[FOO2] = %q, want baz", env["FOO2"])
	}

	// Borrar el último elemento → la key env debe desaparecer del map.
	ed.pendingKey = "FOO2"
	if err := ed.deleteCurrent(); err != nil {
		t.Fatalf("deleteCurrent: %v", err)
	}
	if st.Get("env") != nil {
		t.Errorf("env debería estar borrado del map cuando queda vacío, got %v", st.Get("env"))
	}
}

// TestParseEnvLine cubre el parser de "K1=V1 K2=V2".
func TestParseEnvLine(t *testing.T) {
	cases := []struct {
		in      string
		want    map[string]string
		wantErr bool
	}{
		{"", nil, false},
		{"  ", nil, false},
		{"K=V", map[string]string{"K": "V"}, false},
		{"A=1 B=2", map[string]string{"A": "1", "B": "2"}, false},
		{"NOEQUAL", nil, true},
		{"=BADKEY", nil, true},
		{"GOOD=ok BAD KEY=2", nil, true},
	}
	for _, c := range cases {
		got, err := parseEnvLine(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseEnvLine(%q): want error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseEnvLine(%q): unexpected error: %v", c.in, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseEnvLine(%q): len = %d, want %d (got %v)", c.in, len(got), len(c.want), got)
			continue
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("parseEnvLine(%q)[%s] = %q, want %q", c.in, k, got[k], v)
			}
		}
	}
}

// TestMCPServersEditor_AddArgsAndEnv valida que ARGS se splitea y ENV se
// parsea, y que el bloque mcpServers se borra cuando queda vacío.
func TestMCPServersEditor_AddArgsAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	st, err := settings.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ed := newMCPServersEditor(st, 80, 24)
	ed.openForm("", mcpServer{})
	ed.nameInput.SetValue("github")
	ed.commandInput.SetValue("npx")
	ed.argsInput.SetValue("-y @modelcontextprotocol/server-github")
	ed.envInput.SetValue("GITHUB_TOKEN=ghp_xxx FOO=bar")
	if err := ed.saveForm(); err != nil {
		t.Fatalf("saveForm: %v", err)
	}

	servers := getMCPServers(st)
	gh, ok := servers["github"]
	if !ok {
		t.Fatal("github server no fue guardado")
	}
	if gh.Command != "npx" {
		t.Errorf("Command = %q, want npx", gh.Command)
	}
	wantArgs := []string{"-y", "@modelcontextprotocol/server-github"}
	if len(gh.Args) != len(wantArgs) {
		t.Errorf("Args = %v, want %v", gh.Args, wantArgs)
	}
	if gh.Env["GITHUB_TOKEN"] != "ghp_xxx" || gh.Env["FOO"] != "bar" {
		t.Errorf("Env = %v, want GITHUB_TOKEN=ghp_xxx FOO=bar", gh.Env)
	}

	// Borrar → mcpServers debe desaparecer del map top-level.
	ed.pendingName = "github"
	if err := ed.deleteCurrent(); err != nil {
		t.Fatalf("deleteCurrent: %v", err)
	}
	if st.Get("mcpServers") != nil {
		t.Errorf("mcpServers debería estar borrado, got %v", st.Get("mcpServers"))
	}
}

// TestStringListHelpers cubre setStringList con vacío (debe borrar la key).
func TestStringListHelpers(t *testing.T) {
	perms := map[string]any{
		"allow": []any{"a", "b"},
	}

	got := getStringList(perms, "allow")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("getStringList = %v, want [a b]", got)
	}

	setStringList(perms, "allow", []string{"x"})
	if got := getStringList(perms, "allow"); len(got) != 1 || got[0] != "x" {
		t.Errorf("after set: %v, want [x]", got)
	}

	setStringList(perms, "allow", nil)
	if _, present := perms["allow"]; present {
		t.Errorf("setStringList(nil) debería borrar la key")
	}
}
