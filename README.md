DiskTree TUI (Go)
===================

A small terminal user interface (TUI) written in Go (requires Go 1.25 or later) that scans a directory and shows immediate children sorted by size. It provides quick navigation (drill down/up), sorting, rescanning, and CSV export of the current view.

Features
- Scan a directory and display immediate children with Size, Files, Dirs, % of parent, and a small bar graph
- Navigate into directories with Enter and go up with Backspace
- Toggle sort: by size (default) with `s`, or by name with `n`
- Rescan current directory with `r` (clears cache for that directory)
- Export the current view to CSV with `e` (writes to `du-YYYYMMDD-HHMMSS.csv`)
- Quit with `q` or Ctrl+C

How it works (brief)
- The core scanner walks directory trees to compute sizes and counts. It computes a subtree total for directories without building the full tree for every nested directory (worker-limited concurrency).
- Scanning is cached per-directory to speed up navigation back to already scanned paths (in-memory cache using `sync.Map`).
- Symlinks are skipped by default to avoid cycles; enable following with the `-follow-symlinks` flag.
- The TUI is implemented with Bubble Tea and shows immediate children of the current node in a table.

Files of interest
- `main.go` — application source: scanner, data model (`Node`), TUI model, and CLI flags

Command-line flags
- `-root <path>`
  Root path to scan (default: `.`)
- `-threads <n>`
  Worker concurrency for size calculations (default: `GOMAXPROCS * 4`)
- `-follow-symlinks`
  Follow symbolic links (off by default; may cause cycles)

Build and run
Run from the project root (requires Go module support):

```powershell
# fetch dependencies and run
go mod tidy
go run . -root "." -threads 8

# or build a binary
go build -o disktree .
./disktree -root "C:\path\to\scan" -threads 16
```

Usage notes
- While scanning a directory, the status line shows a spinner and a message like `Scanning /path ...`.
- Press `Enter` on a directory row to drill into it (only directories with subtree data are opened).
- Press `Backspace` to go up one level.
- Press `e` to export the current table to CSV. The CSV is created in the current working directory and named like `du-20250801-153045.csv`.

CSV columns
- Name, Path, SizeBytes, SizeHuman, Files, Dirs, ParentShare%

Limitations & caveats
- The program reports logical file sizes (total bytes in files). On Windows, "size on disk" (allocated size) depends on filesystem cluster size and is not implemented here.
- Symlink handling: symlinks are skipped by default; enabling `-follow-symlinks` can cause cycles if the filesystem contains loops. Use with caution.
- Large trees may be slow or memory-intensive depending on `-threads`. The scanner uses goroutines with a semaphore to bound concurrency.
- Caching is in-memory for the lifetime of the process; there is no persistent cache.
- Errors reading directories are shown in the status line but do not stop the UI.

Troubleshooting
- Permission errors: run with appropriate permissions or choose a different `-root` path.
- If the UI freezes, try reducing `-threads` or scanning a narrower subtree.

Notes for contributors
- The code uses `bubbletea`, `bubbles`, and `lipgloss` for the TUI. Keep UI and scanning concerns reasonably separated when adding features.

License
- No license file is included in this repository; add a LICENSE if you want to publish under a specific license.

Contact
- For questions about the code, open an issue in the repository or inspect `main.go` for implementation details.
