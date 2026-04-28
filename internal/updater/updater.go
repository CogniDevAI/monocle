// Package updater consulta GitHub Releases para detectar versiones nuevas
// del binario y expone helpers para comparar versiones y resolver la URL
// del script de instalación.
//
// El flujo es deliberadamente conservador: ante cualquier error de red el
// caller decide si lo ignora silenciosamente (caso del banner en la TUI) o
// lo propaga (caso del comando `monocle update`).
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// releaseURL apunta al endpoint público de GitHub para la última release.
	releaseURL = "https://api.github.com/repos/CogniDevAI/monocle/releases/latest"
	// installScriptURL es el script de instalación oficial.
	installScriptURL = "https://raw.githubusercontent.com/CogniDevAI/monocle/main/install.sh"
)

// httpClient comparte timeouts entre LatestVersion y futuros consumidores.
// 3s para conectar, 5s total — suficiente para no bloquear el arranque del TUI.
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).DialContext,
	},
}

// releaseResponse es el subset del payload de GitHub que nos interesa.
type releaseResponse struct {
	TagName string `json:"tag_name"`
}

// LatestVersion devuelve el tag normalizado (sin la "v" prefijo) de la
// release más reciente publicada en GitHub. Si la red falla o el JSON es
// inválido, retorna error y el caller decide qué hacer.
func LatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github releases: HTTP %d", resp.StatusCode)
	}

	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("decode release: %w", err)
	}

	return strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v"), nil
}

// InstallURL retorna la URL del script de instalación oficial.
// Se expone como función para mantener consistencia con LatestVersion
// y permitir override en tests si hiciera falta.
func InstallURL() string { return installScriptURL }

// IsNewer reporta si latest > current usando comparación semver simple X.Y.Z.
// Reglas:
//   - current == "dev"  → false (build de desarrollo, nunca propone update)
//   - current == ""     → false (sin versión, no podemos comparar)
//   - cualquiera de los dos malformado → false
//   - X.Y se trata como X.Y.0
func IsNewer(current, latest string) bool {
	if current == "" || current == "dev" {
		return false
	}
	cur, ok := parseSemver(current)
	if !ok {
		return false
	}
	lat, ok := parseSemver(latest)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if lat[i] > cur[i] {
			return true
		}
		if lat[i] < cur[i] {
			return false
		}
	}
	return false
}

// parseSemver parsea X.Y.Z (o X.Y) en un array de 3 ints. Tolera prefijo "v".
// Devuelve ok=false si algún componente no es entero o hay menos de 2.
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return [3]int{}, false
	}
	// Cortamos pre-release / build metadata para no contaminar el parse.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
