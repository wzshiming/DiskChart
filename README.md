# DiskChart

![CI](https://github.com/wzshiming/diskchart/actions/workflows/ci.yml/badge.svg)

DiskChart is a cross-platform (macOS / Windows / Linux) desktop disk usage analyzer: scan disk volumes or folders, use charts to find the folders, files, and file types taking up the most space, and clean them up right inside the app.

Disk usage analyzer for macOS, Windows and Linux.

## Features

- Scan a disk volume or any folder; cancel at any time
- Does not cross filesystem boundaries (skips other mount points)
- Hard links counted only once; symbolic links not followed
- Treemap / Sunburst charts with click-to-drill-down and breadcrumbs to navigate back
- Top 100 largest files
- Top 50 largest folders with dominance filtering: when ≥ 90% of a directory's size comes from a single subdirectory, only that subdirectory is listed
- Usage distribution by file type (extension)
- Per file/folder: reveal in file manager, copy path, permanent delete (with confirmation)
- Chinese / English bilingual UI
- Light/dark theme follows the system
- Capacity units: decimal on macOS (1 KB = 1000 B), binary on other platforms (1 KiB = 1024 B)

## Screenshots

TODO (will be placed in `docs/screenshots/`).

## Tech Stack & Architecture

- [Wails](https://wails.io) v2.16 (Go backend + native WebView)
- Svelte 5 + TypeScript + Vite
- Tailwind CSS 4
- ECharts

| Directory | Responsibility |
| --- | --- |
| `main.go` | Wails application assembly |
| `internal/scan` | Concurrent filesystem scanning; builds the scan tree |
| `internal/stats` | Statistics over the scan tree: subtree views, top files/folders, type distribution |
| `internal/volumes` | Lists scannable disk volumes and mount points |
| `internal/fsops` | System operations: reveal in file manager, guarded deletion |
| `internal/app` | Service layer bound to the frontend (scan lifecycle, queries, events) |
| `frontend/src/lib` | Frontend logic: API wrappers, state, chart config, formatting, i18n |
| `frontend/src/components` | Frontend UI components |
| `frontend/wailsjs` | Wails-generated frontend bindings |

## Development

### Requirements

- Go ≥ 1.26
- Node 26 (see `.nvmrc`)
- pnpm 12
- Wails CLI v2.16.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`
- Platform dependencies:
  - macOS: Xcode Command Line Tools
  - Linux: `libgtk-3-dev libwebkit2gtk-4.1-dev`; build and test with `-tags webkit2_41`
  - Windows: WebView2 Runtime

Run `wails doctor` to check your environment.

### Run & Build

```sh
wails dev                       # development mode (hot reload)
wails build                     # build; output goes to build/bin/
wails build -tags webkit2_41    # Linux
```

### Make Targets

| Command | Description |
| --- | --- |
| `make dev` | `wails dev` |
| `make build` | `wails build` |
| `make test` | Go tests (defaults to `-race`; override with `GO_TEST_FLAGS`) |
| `make vet` | `go vet` |
| `make fmt` | Check Go code formatting |
| `make ui-install` | Install frontend dependencies |
| `make ui-check` | Frontend type checking (svelte-check) |
| `make ui-test` | Frontend tests (vitest) |
| `make check` | fmt + vet + test + ui-check + ui-test |
| `make clean` | Clean build artifacts |

### CI

GitHub Actions (`.github/workflows/ci.yml`) runs Go vet/test and frontend check/test/build on all three platforms, and builds the app artifacts (`diskchart-darwin-universal`, `diskchart-windows-amd64`, `diskchart-linux-amd64`).

## License

[MIT](LICENSE)
