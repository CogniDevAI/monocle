package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/CogniDevAI/monocle/internal/ui"
)

// TestStatusline_ShowsAllPresets navega al editor de statusline y valida
// que los 4 items aparecen: Minimal, Compact, Full y Custom.
//
// El list de bubbles muestra varios items por pantalla cuando hay altura
// suficiente, así que con un term de 120x60 deberían entrar los 4.
func TestStatusline_ShowsAllPresets(t *testing.T) {
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

	// Statusline es el primer item del menú raíz.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Esperar a entrar al editor (frame que contiene "Elegí un preset").
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool {
			// Una vez visto el frame con el título del editor, los items
			// están renderizados. Validamos los 4 en el mismo buffer.
			if !bytes.Contains(b, []byte("Elegí un preset")) {
				return false
			}
			for _, want := range []string{"Minimal", "Compact", "Full", "Custom"} {
				if !bytes.Contains(b, []byte(want)) {
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

// TestStatusline_ApplyMinimalCreatesScript navega al preset Minimal,
// previsualiza, aplica con "y" y verifica que ~/.claude/statusline.sh
// existe y arranca con el shebang bash.
func TestStatusline_ApplyMinimalCreatesScript(t *testing.T) {
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

	// menu → enter en Statusline
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Minimal")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// El cursor del list ya está sobre "Minimal" (primer preset).
	// Enter dispara stepConfirm con preview.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Preview")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// "y" aplica. El apply escribe statusline.sh y settings.json.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})

	// Esperar a que el archivo se materialice. El cmd de apply corre
	// sincrónico dentro del Update del modelo, pero el dispatch va por
	// el program loop, así que damos un margen razonable.
	scriptPath := filepath.Join(home, ".claude", "statusline.sh")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(scriptPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	body, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("statusline.sh no se creó en %s: %v", scriptPath, err)
	}
	if !strings.HasPrefix(string(body), "#!/usr/bin/env bash") {
		t.Errorf("statusline.sh no arranca con shebang bash; head=%q", firstLine(body))
	}

	// El settings.json también debe haberse escrito con statusLine.command.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("settings.json no se creó: %v", err)
	}
}

func firstLine(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}
