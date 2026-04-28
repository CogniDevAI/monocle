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
