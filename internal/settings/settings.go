// Package settings carga y persiste ~/.claude/settings.json preservando
// los campos que la TUI no conoce. Todas las escrituras hacen backup
// timestamped y son atómicas (write-temp + rename).
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxBackups es la cantidad máxima de backups timestamped que se conservan
// por archivo. Cuando Save() crea uno nuevo y la cuenta total supera este
// número, se eliminan los más viejos.
const MaxBackups = 10

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

// Set sobrescribe la clave top-level. Una key vacía se ignora silenciosamente
// para evitar entradas raras como `"": "x"` en el JSON.
func (s *Settings) Set(key string, value any) {
	if key == "" {
		return
	}
	s.data[key] = value
}

// Save escribe el JSON con backup. Crea el directorio si hace falta.
//
// Orden de operaciones (importa para no dejar archivos huérfanos ni
// backups inútiles):
//  1. MarshalIndent — si falla acá, no se tocó nada en disco todavía.
//  2. Si ya existe el archivo: backup timestamped en <path>.bak.<ts>.
//  3. Rotación: borrar backups más viejos si la cuenta supera MaxBackups.
//  4. Escribir <path>.tmp y rename atómico → <path>.
//     Si el rename falla, se borra el .tmp para no dejar basura.
func (s *Settings) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	out, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if _, err := os.Stat(s.path); err == nil {
		ts := time.Now().Format("20060102-150405.000000")
		backup := s.path + ".bak." + ts
		if err := copyFile(s.path, backup); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		if err := pruneBackups(s.path, MaxBackups); err != nil {
			// Pruning failure no debe abortar el save — el dato está a salvo.
			// Lo dejamos pasar silencioso; podríamos loggear si tuviéramos logger.
			_ = err
		}
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		// Si el rename falla, no dejamos el .tmp colgando.
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, s.path, err)
	}
	return nil
}

// pruneBackups borra los backups más viejos si hay más de keep. Compara por
// nombre (orden lexicográfico ≈ orden temporal por el formato del timestamp).
func pruneBackups(path string, keep int) error {
	dir := filepath.Dir(path)
	prefix := filepath.Base(path) + ".bak."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var backups []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			backups = append(backups, e.Name())
		}
	}

	if len(backups) <= keep {
		return nil
	}

	// Orden ascendente: el primero es el más viejo.
	sort.Strings(backups)
	toDelete := backups[:len(backups)-keep]
	for _, name := range toDelete {
		_ = os.Remove(filepath.Join(dir, name))
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

// Hooks devuelve el bloque "hooks" como mapa de evento → lista de entradas.
// Si el bloque no existe o tiene tipos inesperados, devuelve un mapa vacío.
// Las entradas internas se devuelven como []any (cada item es típicamente un
// map[string]any con "matcher" y "hooks"). Mantenemos []any para no romper
// los tipos arbitrarios que pudieran venir del JSON.
func (s *Settings) Hooks() map[string][]any {
	out := map[string][]any{}
	raw, ok := s.data["hooks"].(map[string]any)
	if !ok {
		return out
	}
	for event, list := range raw {
		entries, ok := list.([]any)
		if !ok {
			continue
		}
		out[event] = entries
	}
	return out
}

// SetHooks reemplaza el bloque "hooks" con el mapa provisto. Si el mapa
// queda vacío, elimina la clave para no dejar `"hooks": {}` colgado.
func (s *Settings) SetHooks(hooks map[string][]any) {
	if len(hooks) == 0 {
		delete(s.data, "hooks")
		return
	}
	out := map[string]any{}
	for event, entries := range hooks {
		if len(entries) == 0 {
			continue
		}
		out[event] = entries
	}
	if len(out) == 0 {
		delete(s.data, "hooks")
		return
	}
	s.data["hooks"] = out
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}
