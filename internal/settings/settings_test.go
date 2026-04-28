package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: escribe un settings.json en path con el map dado, formateado.
func writeJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal helper: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir helper: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func TestLoad_NotExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load no debería fallar si el archivo no existe, got: %v", err)
	}
	if s == nil {
		t.Fatal("Load debe retornar un Settings no-nil para archivo inexistente")
	}
	if got := s.Get("anything"); got != nil {
		t.Errorf("Settings vacío debe devolver nil para cualquier key, got %v", got)
	}
	if s.Path() != path {
		t.Errorf("Path() = %q, want %q", s.Path(), path)
	}
}

func TestLoad_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	writeJSON(t, path, map[string]any{
		"model":      "claude-opus",
		"theme":      "dark",
		"statusLine": map[string]any{"type": "command", "command": "bash /tmp/x.sh"},
	})

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load existing falló: %v", err)
	}

	if got := s.Get("model"); got != "claude-opus" {
		t.Errorf("Get(model) = %v, want %q", got, "claude-opus")
	}
	if got := s.Get("theme"); got != "dark" {
		t.Errorf("Get(theme) = %v, want %q", got, "dark")
	}

	sl, ok := s.Get("statusLine").(map[string]any)
	if !ok {
		t.Fatalf("Get(statusLine) no es map[string]any: %T", s.Get("statusLine"))
	}
	if sl["type"] != "command" {
		t.Errorf("statusLine.type = %v, want %q", sl["type"], "command")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load con JSON inválido debe retornar error, got nil")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load con archivo vacío no debería fallar: %v", err)
	}
	if got := s.Get("anything"); got != nil {
		t.Errorf("archivo vacío debe dar Settings vacío, got %v", got)
	}
}

// findBackup busca en el directorio el primer archivo que matchee el prefijo
// "<base>.bak.". Falla el test si encuentra cero o más de uno.
func findBackup(t *testing.T, dir, base string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	prefix := base + ".bak."
	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 0 {
		t.Fatalf("no se encontró backup con prefijo %q en %s", prefix, dir)
	}
	if len(matches) > 1 {
		t.Fatalf("se encontraron múltiples backups: %v", matches)
	}
	return filepath.Join(dir, matches[0])
}

func TestSave_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	original := map[string]any{"model": "old-value", "theme": "light"}
	writeJSON(t, path, original)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Set("model", "new-value")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	backupPath := findBackup(t, dir, "settings.json")

	backupRaw, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("leyendo backup: %v", err)
	}
	var backupData map[string]any
	if err := json.Unmarshal(backupRaw, &backupData); err != nil {
		t.Fatalf("backup JSON inválido: %v", err)
	}

	if backupData["model"] != "old-value" {
		t.Errorf("backup debe tener el valor ORIGINAL, got model=%v", backupData["model"])
	}
	if backupData["theme"] != "light" {
		t.Errorf("backup debe preservar todos los campos, theme=%v", backupData["theme"])
	}
}

func TestSave_NoBackupIfFileDoesntExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "settings.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Set("model", "fresh")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// no debería haber backup porque no había archivo previo
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			t.Errorf("no debería existir backup para archivo nuevo, encontrado: %s", e.Name())
		}
	}
}

func TestSave_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Set("model", "claude-sonnet")
	s.Set("nested", map[string]any{"a": float64(1), "b": "two"})

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// el archivo final debe existir
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leyendo settings tras save: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("contenido tras save no es JSON válido: %v", err)
	}
	if got["model"] != "claude-sonnet" {
		t.Errorf("model = %v, want %q", got["model"], "claude-sonnet")
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested no es map: %T", got["nested"])
	}
	if nested["a"] != float64(1) || nested["b"] != "two" {
		t.Errorf("nested mal serializado: %v", nested)
	}

	// no debería quedar el archivo .tmp
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp no fue limpiado tras save: err=%v", err)
	}
}

func TestSave_PreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	original := map[string]any{
		"model":     "original-model",
		"foo":       "bar",
		"customKey": map[string]any{"x": float64(42), "y": []any{"a", "b"}},
		"future":    "unsupported-field",
	}
	writeJSON(t, path, original)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// modifico un campo conocido
	s.Set("model", "new-model")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leyendo: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("JSON inválido tras save: %v", err)
	}

	if got["model"] != "new-model" {
		t.Errorf("model no se actualizó: %v", got["model"])
	}
	if got["foo"] != "bar" {
		t.Errorf("campo desconocido 'foo' se perdió: %v", got["foo"])
	}
	if got["future"] != "unsupported-field" {
		t.Errorf("campo 'future' se perdió: %v", got["future"])
	}
	custom, ok := got["customKey"].(map[string]any)
	if !ok {
		t.Fatalf("customKey perdió su shape: %T", got["customKey"])
	}
	if custom["x"] != float64(42) {
		t.Errorf("customKey.x = %v, want 42", custom["x"])
	}
}

func TestSetStatusLineCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	scriptPath := "/home/user/.claude/statusline.sh"
	s.SetStatusLineCommand(scriptPath)

	raw := s.Get("statusLine")
	sl, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("statusLine no es map[string]any: %T", raw)
	}

	if sl["type"] != "command" {
		t.Errorf("statusLine.type = %v, want %q", sl["type"], "command")
	}
	wantCmd := "bash " + scriptPath
	if sl["command"] != wantCmd {
		t.Errorf("statusLine.command = %v, want %q", sl["command"], wantCmd)
	}

	// debe persistirse al guardar
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	persisted, err := Load(path)
	if err != nil {
		t.Fatalf("re-Load: %v", err)
	}
	sl2, ok := persisted.Get("statusLine").(map[string]any)
	if !ok {
		t.Fatalf("statusLine tras reload no es map: %T", persisted.Get("statusLine"))
	}
	if sl2["command"] != wantCmd {
		t.Errorf("command no persistió: got %v want %q", sl2["command"], wantCmd)
	}
}

func TestSet_IgnoresEmptyKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Set("", "no-debería-entrar")

	if got := s.Get(""); got != nil {
		t.Errorf("Set con key vacía no debe registrar el valor, got %v", got)
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leyendo settings tras save: %v", err)
	}
	if strings.Contains(string(raw), `""`) && strings.Contains(string(raw), "no-debería-entrar") {
		t.Errorf("settings.json contiene la entry con key vacía: %s", raw)
	}
}

func TestSave_RotatesOldBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Sembrar el archivo + (MaxBackups + 3) backups antiguos con timestamps
	// crecientes para que el de menor lex sea el más viejo.
	writeJSON(t, path, map[string]any{"k": "v"})
	excess := 3
	for i := 0; i < MaxBackups+excess; i++ {
		fake := path + ".bak.20200101-00000" + string(rune('0'+i%10))
		if err := os.WriteFile(fake, []byte("{}"), 0o644); err != nil {
			t.Fatalf("seed backup %d: %v", i, err)
		}
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Set("k", "v2")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Tras el Save debe haber 1 backup nuevo + se borraron los excedentes.
	// Total esperado ≈ MaxBackups (los más nuevos sobreviven).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings.json.bak.") {
			count++
		}
	}
	if count > MaxBackups {
		t.Errorf("rotación falló: hay %d backups, máximo %d", count, MaxBackups)
	}
	if count < 1 {
		t.Errorf("se borraron todos los backups, debería quedar al menos el nuevo")
	}
}

func TestDefaultPath_ContainsClaudeSettings(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if !strings.HasSuffix(p, filepath.Join(".claude", "settings.json")) {
		t.Errorf("DefaultPath = %q, debe terminar en .claude/settings.json", p)
	}
}
