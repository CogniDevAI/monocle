package presets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAll_ReturnsExpectedPresets(t *testing.T) {
	got := All()

	wantIDs := []string{"minimal", "compact", "full"}
	if len(got) != len(wantIDs) {
		t.Fatalf("All() devolvió %d presets, want %d", len(got), len(wantIDs))
	}

	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Errorf("preset[%d].ID = %q, want %q", i, got[i].ID, want)
		}
		if got[i].Name == "" {
			t.Errorf("preset[%d].Name vacío", i)
		}
		if got[i].Description == "" {
			t.Errorf("preset[%d].Description vacío", i)
		}
		if got[i].Script == "" {
			t.Errorf("preset[%d].Script vacío", i)
		}
	}
}

func TestFindByID(t *testing.T) {
	t.Run("conocido", func(t *testing.T) {
		p := FindByID("compact")
		if p == nil {
			t.Fatal("FindByID(compact) devolvió nil para preset existente")
		}
		if p.ID != "compact" {
			t.Errorf("ID = %q, want %q", p.ID, "compact")
		}
	})

	t.Run("desconocido", func(t *testing.T) {
		if p := FindByID("no-existe"); p != nil {
			t.Errorf("FindByID(no-existe) = %+v, want nil", p)
		}
	})

	t.Run("case sensitive", func(t *testing.T) {
		// según el doc del código, FindByID es case-sensitive
		if p := FindByID("Compact"); p != nil {
			t.Errorf("FindByID(Compact) debería ser case-sensitive y devolver nil, got %+v", p)
		}
	})

	t.Run("string vacío", func(t *testing.T) {
		if p := FindByID(""); p != nil {
			t.Errorf("FindByID(\"\") = %+v, want nil", p)
		}
	})
}

func TestContent_ReturnsBytes(t *testing.T) {
	wantShebang := []byte("#!/usr/bin/env bash")

	for _, p := range All() {
		t.Run(p.ID, func(t *testing.T) {
			body, err := p.Content()
			if err != nil {
				t.Fatalf("Content() falló: %v", err)
			}
			if len(body) == 0 {
				t.Fatal("Content() devolvió bytes vacíos")
			}
			if !bytes.HasPrefix(body, wantShebang) {
				t.Errorf("script no empieza con shebang esperado.\nprimeros bytes: %q", firstLine(body))
			}
		})
	}
}

func firstLine(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out", "statusline.sh")

	p := FindByID("minimal")
	if p == nil {
		t.Fatal("preset minimal no encontrado")
	}

	finalPath, err := p.Install(target)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if finalPath != target {
		t.Errorf("Install devolvió %q, want %q", finalPath, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("archivo destino no existe: %v", err)
	}

	// permisos: el código pide 0755. En Unix verificamos los bits exactos.
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("permisos = %o, want %o", got, 0o755)
	}

	// contenido debe matchear Content()
	wantBody, err := p.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	gotBody, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("leyendo target: %v", err)
	}
	if !bytes.Equal(gotBody, wantBody) {
		t.Errorf("contenido instalado no coincide con Content() del preset")
	}
}

func TestInstall_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "statusline.sh")

	// dejo un archivo previo con contenido distinto
	if err := os.WriteFile(target, []byte("old-junk"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	p := FindByID("full")
	if p == nil {
		t.Fatal("preset full no encontrado")
	}
	if _, err := p.Install(target); err != nil {
		t.Fatalf("Install: %v", err)
	}

	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("leyendo: %v", err)
	}
	if bytes.Equal(body, []byte("old-junk")) {
		t.Error("Install no sobrescribió el archivo existente")
	}

	want, _ := p.Content()
	if !bytes.Equal(body, want) {
		t.Error("contenido tras overwrite no coincide con Content()")
	}
}
