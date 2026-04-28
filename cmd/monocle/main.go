// Monocle — TUI elegante para configurar Claude Code.
//
// Lee y modifica ~/.claude/settings.json haciendo backup automático.
// Empezamos con la configuración del statusLine; en próximas versiones
// agregamos hooks, permisos y output styles.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CogniDevAI/monocle/internal/ui"
	"github.com/CogniDevAI/monocle/internal/updater"
)

// version se sobreescribe en build vía -ldflags="-X main.version=...".
// Para que goreleaser pueda inyectarla, debe ser var (no const).
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Printf("monocle %s\n", version)
			return
		case "-h", "--help", "help":
			printHelp()
			return
		case "update":
			if err := runUpdate(); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
			return
		}
	}

	app, err := ui.NewApp(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	prog := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runUpdate descarga el script de instalación oficial y lo ejecuta con sh,
// volcando el output al stdout/stderr del usuario. No verifica checksum ni
// hace rollback — eso queda fuera de scope para esta versión.
func runUpdate() error {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updater.InstallURL(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descarga del instalador: HTTP %d", resp.StatusCode)
	}

	cmd := exec.CommandContext(ctx, "sh")
	cmd.Stdin = resp.Body
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printHelp() {
	fmt.Print(`monocle — configurador elegante de Claude Code

Uso:
  monocle              abre el TUI interactivo
  monocle update       descarga e instala la última versión
  monocle version      imprime la versión
  monocle help         muestra esta ayuda

Edita ~/.claude/settings.json haciendo backup automático antes de cada
escritura. Se inicia con un menú de secciones (statusline, etc).
`)
}
