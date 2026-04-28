# Monocle

> Configurador elegante de Claude Code — un TUI en Go para editar
> `~/.claude/settings.json` sin abrir el editor a mano.

## Instalación

```sh
curl -fsSL https://raw.githubusercontent.com/CogniDevAI/monocle/main/install.sh | sh
```

El script detecta tu OS/arquitectura, baja el binario de la última release
de GitHub y lo coloca en `~/.local/bin`. Asegurate de tener ese path en tu
`$PATH`.

Landing para compartir: https://cognidevai.github.io/monocle/

### Otras formas

Build local (requiere Go 1.22+):

```sh
git clone https://github.com/CogniDevAI/monocle.git
cd monocle
go install ./cmd/monocle
```

## Uso

```sh
monocle
```

Se abre un menú interactivo. Hoy permite configurar:

- **Statusline** — elegí entre presets (`minimal`, `compact`, `full`) que
  se materializan como script bash en `~/.claude/statusline.sh` y se
  apuntan desde `settings.json`.

Cada vez que Monocle escribe `settings.json` hace un backup
`settings.json.bak.<timestamp>` antes de tocar el archivo.

## Roadmap

- [ ] Editor de hooks (PreToolUse, PostToolUse, etc.)
- [ ] Editor de permissions (allow/deny rules)
- [ ] Editor de output styles
- [ ] Preview en vivo del statusline antes de aplicar
- [ ] Custom statusline (editor de comandos a mano)

## Desarrollo

```sh
go run ./cmd/monocle
go test ./...
goreleaser release --snapshot --clean   # build local de releases
```

## Licencia

MIT
