// Package settings carga y persiste ~/.claude/settings.json preservando
// los campos que la TUI no conoce. Todas las escrituras hacen backup
// timestamped y son atómicas (write-temp + rename).
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Settings es el contenido completo de settings.json como mapa flexible.
// Mantenerlo como map[string]any preserva campos desconocidos al reescribir.
type Settings struct {
	path string
	data map[string]any
}

// DefaultPath retorna ~/.claude/settings.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// Load lee settings.json. Si no existe, retorna un Settings vacío
// listo para guardar.
func Load(path string) (*Settings, error) {
	s := &Settings{path: path, data: map[string]any{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", path, err)
	}
	return s, nil
}

// Path retorna el path absoluto del archivo gestionado.
func (s *Settings) Path() string { return s.path }

// Get devuelve el valor de la clave top-level (o nil si no existe).
func (s *Settings) Get(key string) any { return s.data[key] }

// Set sobrescribe la clave top-level.
func (s *Settings) Set(key string, value any) { s.data[key] = value }

// Save escribe el JSON con backup. Crea el directorio si hace falta.
// El backup queda en <path>.bak.<timestamp>.
func (s *Settings) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(s.path); err == nil {
		ts := time.Now().Format("20060102-150405")
		backup := s.path + ".bak." + ts
		if err := copyFile(s.path, backup); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}

	out, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, s.path, err)
	}
	return nil
}

// SetStatusLineCommand configura el bloque statusLine para apuntar a un
// script via "bash <path>". Es la forma estándar que documenta Claude Code.
func (s *Settings) SetStatusLineCommand(scriptPath string) {
	s.data["statusLine"] = map[string]any{
		"type":    "command",
		"command": "bash " + scriptPath,
	}
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}
