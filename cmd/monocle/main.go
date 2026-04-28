// Monocle — TUI elegante para configurar Claude Code.
//
// Lee y modifica ~/.claude/settings.json haciendo backup automático.
// Empezamos con la configuración del statusLine; en próximas versiones
// agregamos hooks, permisos y output styles.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CogniDevAI/monocle/internal/ui"
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
		}
	}

	app, err := ui.NewApp()
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

func printHelp() {
	fmt.Print(`monocle — configurador elegante de Claude Code

Uso:
  monocle              abre el TUI interactivo
  monocle version      imprime la versión
  monocle help         muestra esta ayuda

Edita ~/.claude/settings.json haciendo backup automático antes de cada
escritura. Se inicia con un menú de secciones (statusline, etc).
`)
}
