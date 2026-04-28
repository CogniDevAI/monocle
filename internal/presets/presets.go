// Package presets contiene los scripts de statusline embebidos en el binario.
// Cada preset se materializa al disco bajo ~/.claude/statusline.sh cuando el
// usuario lo selecciona desde la TUI.
package presets

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed scripts/*.sh
var scripts embed.FS

// Preset describe una variante de statusline aplicable.
type Preset struct {
	ID          string // identificador estable (slug)
	Name        string // nombre legible para mostrar
	Description string // descripción corta para la lista
	Script      string // ruta dentro del FS embebido
}

// All retorna los presets disponibles, en el orden en que se muestran.
func All() []Preset {
	return []Preset{
		{
			ID:          "minimal",
			Name:        "Minimal",
			Description: "carpeta │ branch — lo justo y necesario",
			Script:      "scripts/minimal.sh",
		},
		{
			ID:          "compact",
			Name:        "Compact",
			Description: "carpeta │ branch │ modelo │ % de contexto",
			Script:      "scripts/compact.sh",
		},
		{
			ID:          "full",
			Name:        "Full (Gentleman)",
			Description: "todo: carpeta, branch, modelo, barra de contexto y tokens",
			Script:      "scripts/full.sh",
		},
	}
}

// FindByID busca un preset por su ID (case-sensitive). Retorna nil si no existe.
func FindByID(id string) *Preset {
	for _, p := range All() {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

// Content retorna el contenido del script embebido del preset.
func (p Preset) Content() ([]byte, error) {
	return scripts.ReadFile(p.Script)
}

// Install escribe el script del preset al disco y devuelve el path final.
// Por defecto va a ~/.claude/statusline.sh — el caller puede pasar otro path.
func (p Preset) Install(targetPath string) (string, error) {
	if targetPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		targetPath = filepath.Join(home, ".claude", "statusline.sh")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	body, err := p.Content()
	if err != nil {
		return "", fmt.Errorf("leyendo preset %s: %w", p.ID, err)
	}
	if err := os.WriteFile(targetPath, body, 0o755); err != nil {
		return "", fmt.Errorf("escribiendo %s: %w", targetPath, err)
	}
	return targetPath, nil
}
