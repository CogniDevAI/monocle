package ui_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/CogniDevAI/monocle/internal/ui"
)

// hooksExpectedEvents replica la lista de eventos válidos del modelo
// (internal/ui/hooks.go). Si la app suma uno nuevo, este test lo señala.
var hooksExpectedEvents = []string{
	"PreToolUse",
	"PostToolUse",
	"UserPromptSubmit",
	"Notification",
	"Stop",
	"SubagentStop",
	"SessionStart",
	"SessionEnd",
	"PreCompact",
}

// TestHooks_ListsAllEvents valida que al entrar al sub-modelo de Hooks
// se listan los 9 eventos válidos según la doc de Claude Code, incluso
// con un settings vacío. Usamos un term grande para que entren todos
// en la misma vista del list.
func TestHooks_ListsAllEvents(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app, err := ui.NewApp("dev")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 60))

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Statusline")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// menu → bajar a "Hooks" (segundo item) → enter
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Esperar al frame que tiene los 9 eventos. Con term 120x60 entran
	// todos sin scroll.
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool {
			for _, ev := range hooksExpectedEvents {
				if !bytes.Contains(b, []byte(ev)) {
					return false
				}
			}
			return true
		},
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(5*time.Second),
	)

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestHooks_AddEntryFlow ejercita el flujo de agregar un hook a un evento:
// entrar al evento, "a" para abrir formulario, tipear matcher y command,
// ctrl+s para guardar y verificar que settings.json se actualizó.
func TestHooks_AddEntryFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, err := ui.NewApp("dev")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 60))

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Statusline")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// menu → Hooks (segundo item)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("PreToolUse")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// Cursor por default sobre el primer evento (PreToolUse). Entramos.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Cuando el evento no tiene hooks, la View muestra el placeholder
	// "No hay hooks configurados...".
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool {
			return bytes.Contains(b, []byte("No hay hooks configurados")) ||
				bytes.Contains(b, []byte("Hooks de PreToolUse"))
		},
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// "a" abre el formulario de agregar.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Agregar hook")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// Tipear matcher: "Bash"
	tm.Type("Bash")
	// Tab al campo command.
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	// Tipear command: "echo hola"
	tm.Type("echo hola")
	// ctrl+s guarda.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})

	// El save dispara settings.Save() sincrónico en el Update y luego
	// vuelve a stepHookEntries con flash. Esperamos a que el archivo se
	// materialice en disco.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(settingsPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	// Validar el contenido de settings.json en disco.
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json no existe: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("settings.json inválido: %v", err)
	}
	hooks, ok := data["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("data[hooks] no es map: %T", data["hooks"])
	}
	pre, ok := hooks["PreToolUse"].([]any)
	if !ok || len(pre) != 1 {
		t.Fatalf("hooks.PreToolUse = %v, want 1 entry", hooks["PreToolUse"])
	}
	entry, _ := pre[0].(map[string]any)
	if got, _ := entry["matcher"].(string); got != "Bash" {
		t.Errorf("matcher = %q, want Bash", got)
	}
	innerHooks, _ := entry["hooks"].([]any)
	if len(innerHooks) != 1 {
		t.Fatalf("inner hooks = %v, want 1", innerHooks)
	}
	first, _ := innerHooks[0].(map[string]any)
	if cmd, _ := first["command"].(string); cmd != "echo hola" {
		t.Errorf("command = %q, want 'echo hola'", cmd)
	}
}
