# DiskTree - Terminal Directory Scanner

DiskTree is a Go-based terminal user interface (TUI) application that scans directories and displays immediate children sorted by size. It provides quick navigation, sorting, rescanning, and CSV export functionality using the Bubble Tea framework.

**ALWAYS reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.**

## Working Effectively

### Initial Setup and Dependencies
- Ensure Go 1.25 or later is installed: `go version` (required for this project)
- Install dependencies: `go mod tidy` (takes ~10-15 seconds with dependency downloads)
- Build the application: `go build -o disktree .` (takes ~0.5-9 seconds depending on cache, NEVER CANCEL)
- Run tests: `go test ./... -v` (takes ~0.3-2.5 seconds depending on cache, NEVER CANCEL)

### Building and Testing
- **Build command**: `go build -o disktree .`
  - Build time: ~0.5-9 seconds (depends on cache state)
  - NEVER CANCEL: Set timeout to 60+ seconds minimum
  - Output: `disktree` binary in project root
- **Test command**: `go test ./... -v`
  - Test time: ~0.3-2.5 seconds (depends on cache state)
  - NEVER CANCEL: Set timeout to 30+ seconds minimum
  - All tests should pass (6 tests: unit tests, integration tests for export and scanner)
- **Dependencies**: `go mod tidy`
  - Downloads Bubble Tea framework and dependencies
  - Required before first build
  - Time: ~10-15 seconds on first run

### Running the Application
- Basic usage: `./disktree` (scans current directory)
- With options: `./disktree -root "/path/to/scan" -threads 8`
- Help: `./disktree --help` (shows all available flags)
- **Command-line flags**:
  - `-root <path>`: Root path to scan (default: ".")
  - `-threads <n>`: Worker concurrency for size calculations (default: GOMAXPROCS * 4)
  - `-follow-symlinks`: Follow symbolic links (off by default; may cause cycles)
  - `-rescan-after-delete`: Automatically rescan parent after deleting an item

### Application Controls (when running)
- `Enter`: Navigate into selected directory
- `Backspace`: Go up one level in directory tree
- `s`: Sort by size (default)
- `n`: Sort by name
- `r`: Rescan current directory (clears cache)
- `e`: Export current view to CSV (creates `du-YYYYMMDD-HHMMSS.csv`)
- `q` or `Ctrl+C`: Quit application

## Validation and Testing

### Manual Testing Scenarios
**ALWAYS manually validate any changes by running through these scenarios:**

1. **Basic Functionality Test**:
   - Build: `go build -o disktree .`
   - Run: `./disktree -root /tmp -threads 2`
   - Verify: TUI loads, shows directory listing, loading spinner works
   - Navigate: Use Enter to drill into a directory, Backspace to go up
   - Quit: Press 'q' to exit cleanly

2. **Command Line Validation**:
   - Test help: `./disktree --help` (should show all flags)
   - Test with flags: `./disktree -root . -threads 4 -follow-symlinks`
   - Verify different root paths work correctly

3. **Export Feature Test**:
   - Run application: `./disktree`
   - Press 'e' to export CSV
   - Verify CSV file is created with name pattern `du-YYYYMMDD-HHMMSS.csv`
   - Check CSV contains proper headers: Name, Path, SizeBytes, SizeHuman, Files, Dirs, ParentShare%

### Automated Testing
- **Unit tests**: Test utility functions (humanBytes, bar, max)
- **Integration tests**: Test scanner functionality and CSV export
- **Test files**: `main_test.go`, `scanner_integration_test.go`, `export_integration_test.go`, `restore_integration_test.go`
- **Always run** `go test ./... -v` before committing changes
- **Expected**: All 6 tests should pass consistently

## Code Navigation and Architecture

### Files of Interest
- **`main.go`** — Complete application source: scanner, data model (`Node`), TUI model, CLI flags, and main function
- **`main_test.go`** — Unit tests for utility functions
- **`scanner_integration_test.go`** — Integration tests for directory scanning functionality
- **`export_integration_test.go`** — Integration tests for CSV export feature
- **`restore_integration_test.go`** — Integration tests for file restore functionality
- **`go.mod`** — Module definition and dependencies
- **`.github/workflows/ci.yml`** — CI pipeline (tests on Go 1.24 and 1.25, builds cross-platform releases)

### Key Components in main.go
- **Scanner**: Directory scanning with worker-limited concurrency
- **Node struct**: Data model for directory/file information
- **TUI Model**: Bubble Tea model implementing the user interface
- **CLI flags**: Command line argument parsing
- **Cache**: In-memory directory cache using `sync.Map`

### Dependencies
- **Bubble Tea**: `github.com/charmbracelet/bubbletea` - TUI framework
- **Bubbles**: `github.com/charmbracelet/bubbles` - TUI components
- **Lipgloss**: `github.com/charmbracelet/lipgloss` - Styling for TUI

## Development Workflow

### Making Changes
1. **Always run tests first** to establish baseline: `go test ./... -v`
2. **Build to verify** current state: `go build -o disktree .`
3. **Make minimal changes** to achieve the goal
4. **Test immediately** after each change: `go test ./... -v`
5. **Manual validation**: Run the application and test affected functionality
6. **No linting required**: No linters configured in CI or repository

### CI Expectations
- **Tests must pass** on Go 1.24 and 1.25
- **Build must succeed** for cross-platform targets (linux, windows, darwin on amd64, arm64)
- **No additional linting** or formatting requirements
- **Releases**: Binary artifacts are built automatically on tags

### Common Tasks and Timing Expectations
- `go mod tidy`: 10-15 seconds (with downloads), 1-2 seconds (cached)
- `go build -o disktree .`: ~0.5-9 seconds depending on cache (NEVER CANCEL)
- `go test ./... -v`: ~0.3-2.5 seconds depending on cache (NEVER CANCEL)  
- Application startup: Immediate (TUI displays while scanning)
- Directory scanning: Varies by size, shows progress spinner

### Performance Considerations
- **Scanning speed**: Depends on directory size and `-threads` parameter
- **Memory usage**: In-memory cache for scanned directories (lifetime of process)
- **Concurrency**: Uses goroutines with semaphore to bound worker threads
- **Error handling**: Permission errors shown in status line, do not stop UI

## Troubleshooting

### Common Issues
- **Permission errors**: Run with appropriate permissions or choose different `-root` path
- **UI freezes**: Reduce `-threads` parameter or scan narrower subtree
- **Build failures**: Ensure Go 1.25+ is installed and run `go mod tidy`
- **Test failures**: Check for file system permissions or existing processes

### Build Environment
- **Go version**: 1.25 or later required
- **Modules**: Project uses Go modules (`go.mod`)
- **Dependencies**: Downloaded automatically with `go mod tidy`
- **Platform**: Cross-platform (Linux, Windows, macOS on amd64/arm64)

## Reference Information

### Repository Structure
```
.
├── .github/
│   ├── workflows/
│   │   └── ci.yml                    # CI pipeline
│   └── copilot-instructions.md       # This file
├── export_integration_test.go        # CSV export tests
├── go.mod                           # Go module definition
├── go.sum                           # Dependency checksums
├── main.go                          # Complete application source
├── main_test.go                     # Unit tests
├── restore_integration_test.go      # File restore tests
├── scanner_integration_test.go      # Directory scanning tests
├── LICENSE                          # MIT license
└── README.md                        # Project documentation
```

### Key Dependencies (from go.mod)
```
github.com/charmbracelet/bubbles v0.21.0    # TUI components
github.com/charmbracelet/bubbletea v1.3.6   # TUI framework  
github.com/charmbracelet/lipgloss v1.1.0    # TUI styling
```

### CSV Export Format
When using the export feature ('e' key), CSV files are created with these columns:
- Name: File/directory name
- Path: Full path
- SizeBytes: Size in bytes
- SizeHuman: Human-readable size (e.g., "1.5 KB")
- Files: Number of files in subtree
- Dirs: Number of directories in subtree
- ParentShare%: Percentage of parent directory size