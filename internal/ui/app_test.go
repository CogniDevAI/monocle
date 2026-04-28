package ui_test

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/CogniDevAI/monocle/internal/ui"
)

// TestApp_RendersMenu valida que el menú raíz aparece con todas las
// secciones esperadas en el primer frame con term grande.
func TestApp_RendersMenu(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app, err := ui.NewApp("dev")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 60))

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool {
			for _, want := range []string{"Monocle", "Statusline", "Hooks", "Settings", "Salir"} {
				if !bytes.Contains(b, []byte(want)) {
					return false
				}
			}
			return true
		},
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestApp_QuitOnQ — desde el menú principal "q" emite tea.Quit y termina.
func TestApp_QuitOnQ(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app, err := ui.NewApp("dev")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 60))

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Monocle")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestApp_NavigatesToStatusline — enter sobre el primer item del menú activa
// el sub-modelo de statusline, que muestra los presets.
func TestApp_NavigatesToStatusline(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app, err := ui.NewApp("dev")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 60))

	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool { return bytes.Contains(b, []byte("Monocle")) },
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(3*time.Second),
	)

	// El primer item del menú es "Statusline". Enter lo activa.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// El editor de statusline tiene su propio título "Elegí un preset".
	teatest.WaitFor(t, tm.Output(),
		func(b []byte) bool {
			return bytes.Contains(b, []byte("Elegí un preset")) &&
				bytes.Contains(b, []byte("Minimal"))
		},
		teatest.WithCheckInterval(50*time.Millisecond),
		teatest.WithDuration(5*time.Second),
	)

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
