# DiskChart

![CI](https://github.com/wzshiming/DiskChart/actions/workflows/ci.yml/badge.svg)

DiskChart is a cross-platform (macOS / Windows / Linux) desktop disk usage analyzer: scan disk volumes or folders, use charts to find the folders, files, and file types taking up the most space, and clean them up right inside the app.

Disk usage analyzer for macOS, Windows and Linux.

## Features

- Scan a disk volume or any folder; cancel at any time
- Does not cross filesystem boundaries (skips other mount points)
- Hard links counted only once; symbolic links not followed
- Sizes are on-disk usage (allocated blocks, like `du`), not apparent length; sparse, compressed and cloud-placeholder files count only what they occupy; the detail panel also shows the apparent size
- Treemap / Sunburst charts with click-to-drill-down and breadcrumbs to navigate back
- Top 100 largest files
- Top 50 largest folders with dominance filtering: when ≥ 90% of a directory's size comes from a single subdirectory, only that subdirectory is listed
- Usage distribution by file type (extension)
- Per file/folder: reveal in file manager, copy path, permanent delete (with confirmation)
- Chinese / English UI, switchable at any time (follows the system language by default)
- Light/dark theme follows the system
- Capacity units switchable between decimal (1 KB = 1000 B) and binary (1 KiB = 1024 B); defaults to decimal on macOS and binary elsewhere

## Tech Stack

- [Wails](https://wails.io) v2.16 (Go backend + native WebView)
- Svelte 5 + TypeScript + Vite
- Tailwind CSS 4
- ECharts
- Go ≥ 1.26
- Node 26 (see `.nvmrc`)
- pnpm 12
- Platform dependencies:
  - macOS: Xcode Command Line Tools
  - Linux: `libgtk-3-dev libwebkit2gtk-4.1-dev`; build and test with `-tags webkit2_41`
  - Windows: WebView2 Runtime

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/DiskChart/blob/master/LICENSE) for the full license text.
