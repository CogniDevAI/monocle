package presets

import (
	"strings"
	"testing"
)

func TestAllSegments_HasExpectedIDs(t *testing.T) {
	got := AllSegments()
	want := []string{
		"folder", "branch", "model", "contextBar", "contextPercent",
		"tokens", "outputStyle", "permissionMode", "time",
	}
	if len(got) != len(want) {
		t.Fatalf("AllSegments() devolvió %d, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("segments[%d].ID = %q, want %q", i, got[i].ID, id)
		}
		if got[i].Label == "" {
			t.Errorf("segments[%d].Label vacío", i)
		}
	}
}

func TestFindSegment(t *testing.T) {
	if s := FindSegment("model"); s == nil || s.ID != "model" {
		t.Errorf("FindSegment(model) = %+v", s)
	}
	if s := FindSegment("no-existe"); s != nil {
		t.Errorf("FindSegment(no-existe) = %+v, want nil", s)
	}
}

func TestBuildCustomScript_EmptyError(t *testing.T) {
	if _, err := BuildCustomScript(nil); err == nil {
		t.Error("BuildCustomScript(nil) debería retornar error")
	}
	if _, err := BuildCustomScript([]string{}); err == nil {
		t.Error("BuildCustomScript([]) debería retornar error")
	}
	if _, err := BuildCustomScript([]string{"no-existe"}); err == nil {
		t.Error("BuildCustomScript con IDs inválidos debería retornar error")
	}
}

func TestBuildCustomScript_HasShebangAndStdin(t *testing.T) {
	script, err := BuildCustomScript([]string{"folder", "branch"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	if !strings.HasPrefix(script, "#!/usr/bin/env bash\n") {
		t.Errorf("script no empieza con shebang. primer línea: %q", firstLine([]byte(script)))
	}
	if !strings.Contains(script, `input=$(cat)`) {
		t.Error("script no lee stdin con input=$(cat)")
	}
}

func TestBuildCustomScript_OnlyExtractsNeededVars(t *testing.T) {
	// solo "model" → no debería extraer cwd ni used_pct ni tokens.
	script, err := BuildCustomScript([]string{"model"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	if strings.Contains(script, "current_dir") {
		t.Error("script con solo 'model' no debería extraer cwd")
	}
	if strings.Contains(script, "used_percentage") {
		t.Error("script con solo 'model' no debería extraer used_pct")
	}
	if strings.Contains(script, "total_input_tokens") {
		t.Error("script con solo 'model' no debería extraer tokens")
	}
	if !strings.Contains(script, "model.display_name") {
		t.Error("script con 'model' debería extraer model.display_name")
	}
}

func TestBuildCustomScript_FolderNeedsCwd(t *testing.T) {
	script, err := BuildCustomScript([]string{"folder"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	if !strings.Contains(script, "current_dir") {
		t.Error("script con 'folder' debería extraer cwd")
	}
	if !strings.Contains(script, `folder=$(basename "$cwd")`) {
		t.Error("script con 'folder' debería computar basename de cwd")
	}
}

func TestBuildCustomScript_DeduplicatesExtracts(t *testing.T) {
	// contextBar y contextPercent ambos usan used_pct.
	script, err := BuildCustomScript([]string{"contextBar", "contextPercent"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	count := strings.Count(script, `used_pct=$(echo "$input" | jq -r '.context_window.used_percentage`)
	if count != 1 {
		t.Errorf("used_pct debería extraerse una sola vez, got %d", count)
	}
}

func TestBuildCustomScript_DeduplicatesIDs(t *testing.T) {
	script, err := BuildCustomScript([]string{"model", "model", "model"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	// seg_model debería computarse una sola vez.
	count := strings.Count(script, `seg_model=`)
	if count != 1 {
		t.Errorf("seg_model debería aparecer una sola vez, got %d", count)
	}
}

func TestBuildCustomScript_PreservesOrder(t *testing.T) {
	script, err := BuildCustomScript([]string{"tokens", "folder", "model"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	// Buscar las líneas de composición final que referencian seg_<id>.
	idxTokens := strings.Index(script, `if [ -n "$seg_tokens" ]`)
	idxFolder := strings.Index(script, `if [ -n "$seg_folder" ]`)
	idxModel := strings.Index(script, `if [ -n "$seg_model" ]`)
	if idxTokens < 0 || idxFolder < 0 || idxModel < 0 {
		t.Fatal("no se encontraron las composiciones esperadas")
	}
	if !(idxTokens < idxFolder && idxFolder < idxModel) {
		t.Errorf("orden no preservado: tokens=%d folder=%d model=%d",
			idxTokens, idxFolder, idxModel)
	}
}

func TestBuildCustomScript_ContainsSeparator(t *testing.T) {
	script, err := BuildCustomScript([]string{"folder", "model"})
	if err != nil {
		t.Fatalf("BuildCustomScript: %v", err)
	}
	if !strings.Contains(script, `sep="${DIM} │ ${RESET}"`) {
		t.Error("script debería definir separador dimmed │")
	}
	if !strings.Contains(script, `printf "%b\n" "$out"`) {
		t.Error("script debería terminar imprimiendo $out")
	}
}
