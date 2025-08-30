// DiskTree TUI in Go 1.25 using Bubble Tea

package main

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// --------------------------- Data model ---------------------------

type Node struct {
	Name     string
	Path     string
	Size     int64
	Files    int64
	Dirs     int64
	Children []*Node // only immediate children of this node
	Err      error
	Scanned  bool
}

// TrashItem describes a trashed file's metadata stored next to the trashed item.
type TrashItem struct {
	Name      string    `json:"name"`
	TrashPath string    `json:"trash_path"`
	OrigPath  string    `json:"orig_path"`
	DeletedAt time.Time `json:"deleted_at"`
	IsDir     bool      `json:"is_dir"`
}

// Cache scanned directories to avoid recomputing when navigating back
var cache sync.Map // map[string]*Node

// --------------------------- Scanner -----------------------------

type Scanner struct {
	threads        int
	followSymlinks bool
}

type dirSum struct {
	size  int64
	files int64
	dirs  int64
	err   error
}

func (s *Scanner) scanDir(ctx context.Context, path string) *Node {
	if v, ok := cache.Load(path); ok {
		return v.(*Node)
	}

	name := filepath.Base(path)
	if name == "/" || name == "." || name == "" {
		name = path
	}

	n := &Node{Name: name, Path: path}

	// list immediate children
	entries, err := os.ReadDir(path)
	if err != nil {
		n.Err = err
		cache.Store(path, n)
		return n
	}

	// worker semaphore
	sem := make(chan struct{}, maxvalue(1, s.threads))
	var wg sync.WaitGroup
	children := make([]*Node, 0, len(entries))
	mu := sync.Mutex{}

	for _, e := range entries {
		// skip symlinks unless asked
		if e.Type()&fs.ModeSymlink != 0 && !s.followSymlinks {
			continue
		}

		childPath := filepath.Join(path, e.Name())
		child := &Node{Name: e.Name(), Path: childPath}
		children = append(children, child)

		if e.IsDir() {
			wg.Add(1)
			go func(nd *Node) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					// proceed
				case <-ctx.Done():
					return
				}
				defer func() { <-sem }()
				res := s.sumDir(ctx, nd.Path)
				mu.Lock()
				nd.Size, nd.Files, nd.Dirs, nd.Err = res.size, res.files, res.dirs, res.err
				mu.Unlock()
			}(child)
		} else {
			fi, err := e.Info()
			if err == nil {
				child.Size = fi.Size()
				child.Files = 1
			}
		}
	}

	wg.Wait()

	// aggregate
	var total int64
	for _, c := range children {
		total += c.Size
		if c.Dirs > 0 || c.Files > 0 {
			// counts already include nested totals for dirs
			n.Dirs += c.Dirs
			n.Files += c.Files
		}
		if c.Err != nil {
			n.Err = c.Err // keep last error; informational only
		}
	}
	n.Size = total
	n.Children = children
	n.Scanned = true
	cache.Store(path, n)
	return n
}

// sumDir computes totals for an entire subtree without building its full tree
func (s *Scanner) sumDir(ctx context.Context, path string) (res dirSum) {
	// BFS/DFS with semaphore-limited goroutines for subdirectories
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxvalue(1, s.threads))
	errs := make(chan error, 1)

	var mu sync.Mutex
	var files, dirs, size int64

	var walk func(string)
	walk = func(p string) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ents, err := os.ReadDir(p)
		if err != nil {
			select {
			case errs <- err:
			default:
			}
			return
		}
		for _, e := range ents {
			if e.Type()&fs.ModeSymlink != 0 && !s.followSymlinks {
				continue
			}
			child := filepath.Join(p, e.Name())
			if e.IsDir() {
				mu.Lock()
				dirs++
				mu.Unlock()
				wg.Add(1)
				go func(cp string) {
					defer wg.Done()
					select {
					case sem <- struct{}{}:
						// ok
					case <-ctx.Done():
						return
					}
					defer func() { <-sem }()
					walk(cp)
				}(child)
			} else {
				fi, err := e.Info()
				if err == nil {
					mu.Lock()
					size += fi.Size()
					files++
					mu.Unlock()
				}
			}
		}
	}

	walk(path)
	wg.Wait()
	var err error
	select {
	case err = <-errs:
	default:
	}
	return dirSum{size: size, files: files, dirs: dirs, err: err}
}

// --------------------------- TUI ------------------------------

type sortMode int

const (
	sortBySize sortMode = iota
	sortByName
)

type model struct {
	// config
	rootPath       string
	threads        int
	followSymlinks bool

	// ui state
	width  int
	height int

	breadcrumbs []string // stack of paths
	current     *Node
	loading     bool
	status      string

	tbl     table.Model
	spin    spinner.Model
	sort    sortMode
	scanner *Scanner

	ctx    context.Context
	cancel context.CancelFunc
	// delete confirmation
	confirmDelete bool
	deletePath    string
	confirmFocus  int // 0 = yes, 1 = no
	loadingFrame  int
	// incremental scan channel (delivers childUpdateMsg and final scanDoneMsg)
	scanCh chan tea.Msg
	// debounce control for frequent updates
	pendingUpdates bool
	debounceActive bool
	debounceDur    time.Duration
	// behavior options
	autoRescanAfterDelete bool
	// undo history (most recent appended at end)
	trashHistory []*TrashItem
	// time window during which undo is allowed
	undoWindow time.Duration
	// active scan token to match messages to the currently-viewed scan
	scanToken string
	// minimum overlay display time to prevent flicker
	loadingStartTime time.Time
	minLoadingTime   time.Duration
	// track ongoing scans to prevent premature loading state clearing
	ongoingScans   int
	ongoingScansMu sync.Mutex
	// ensure loading state is visible for at least this duration
	loadingMinDuration time.Duration
	// flag to ensure loading state persists during scans
	scanInProgress bool
}

type scanDoneMsg struct {
	node  *Node
	token string
}

type errMsg struct{ err error }

type rescanMsg struct{}

type loadingTickMsg time.Time

type childUpdateMsg struct {
	parent string
	child  *Node
	token  string
}

type flushUpdatesMsg struct{}

type exportDoneMsg struct {
	path string
	err  error
}

func initialModel(root string, threads int, follow bool) *model {
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	cols := []table.Column{
		{Title: "Name", Width: 40},
		{Title: "Size", Width: 12},
		{Title: "Files", Width: 8},
		{Title: "Dirs", Width: 6},
		{Title: "% of Parent", Width: 12},
		{Title: "Graph", Width: 20},
	}

	t := table.New(table.WithColumns(cols), table.WithFocused(true))
	t.SetStyles(tableStyles())

	m := model{
		rootPath:       root,
		threads:        threads,
		followSymlinks: follow,
		breadcrumbs:    []string{root},
		spin:           sp,
		tbl:            t,
		sort:           sortBySize,
		scanner:        &Scanner{threads: threads, followSymlinks: follow},
		ctx:            ctx,
		cancel:         cancel,
		// default undo window 30s
		undoWindow: 30 * time.Second,
		// minimum loading display time to prevent flicker
		minLoadingTime: 200 * time.Millisecond,
		// ensure the loading state is visible for at least this duration
		loadingMinDuration: 500 * time.Millisecond,
	}

	return &m
}

func (m *model) Init() tea.Cmd {
	cache.Delete(m.rootPath)
	m.loading = true
	m.loadingStartTime = time.Now()
	m.status = fmt.Sprintf("Scanning %s ...", m.rootPath)
	return tea.Batch(m.spin.Tick, loadingTicker(), m.startIncrementalScan(m.rootPath))
}

// scanCmd is retained for reference but unused after incremental scanning refactor.
// Keeping it commented to avoid dead-code warnings.
// func (m model) scanCmd(path string) tea.Cmd {
//     return func() tea.Msg {
//         n := m.scanner.scanDir(m.ctx, path)
//         return scanDoneMsg{node: n}
//     }
// }

func loadingTicker() tea.Cmd {
	return tea.Tick(time.Millisecond*120, func(t time.Time) tea.Msg {
		return loadingTickMsg(t)
	})
}

func scanReaderCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		// read one message from the scan channel
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// startIncrementalScan launches an incremental scan in a background goroutine
// and returns a command that will deliver the first message. Subsequent
// messages are delivered by reusing scanReaderCmd repeatedly from Update.
func (m *model) startIncrementalScan(path string) tea.Cmd {
	useFastCache := !m.loading // capture at call time to avoid race conditions
	ch := make(chan tea.Msg, 64)
	m.scanCh = ch
	// generate scan token and store it on the model so updates can match
	token := uniqueSuffix()
	m.scanToken = token
	// increment ongoing scans counter
	m.ongoingScansMu.Lock()
	m.ongoingScans++
	m.ongoingScansMu.Unlock()
	m.scanInProgress = true

	go func(useFastCache bool) {
		defer func() {
			close(ch)
			// decrement ongoing scans counter when scan completes
			m.ongoingScansMu.Lock()
			m.ongoingScans--
			if m.ongoingScans <= 0 {
				m.scanInProgress = false
			}
			m.ongoingScansMu.Unlock()
		}()
		// Use cache if available, fully scanned, and fast cache is enabled
		if useFastCache {
			if v, ok := cache.Load(path); ok {
				if n, ok2 := v.(*Node); ok2 && n.Scanned {
					ch <- scanDoneMsg{node: n, token: token}
					return
				}
			}
		}

		// list immediate children
		ents, err := os.ReadDir(path)
		if err != nil {
			n := &Node{Name: filepath.Base(path), Path: path, Err: err, Scanned: true}
			ch <- scanDoneMsg{node: n, token: token}
			return
		}

		// prepare children slice while launching size workers for directories
		var wg sync.WaitGroup
		var mu sync.Mutex
		childs := make([]*Node, 0, len(ents))

		for _, e := range ents {
			// skip symlinks unless configured
			if e.Type()&fs.ModeSymlink != 0 && !m.followSymlinks {
				continue
			}
			childPath := filepath.Join(path, e.Name())
			child := &Node{Name: e.Name(), Path: childPath}

			if e.IsDir() {
				// append placeholder and compute size asynchronously
				mu.Lock()
				childs = append(childs, child)
				mu.Unlock()

				// send an immediate placeholder update so the UI shows the directory
				child.Size = -1 // sentinel for "scanning"
				ch <- childUpdateMsg{parent: path, child: child, token: token}

				wg.Add(1)
				go func(nd *Node) {
					defer wg.Done()
					res := m.scanner.sumDir(m.ctx, nd.Path)
					nd.Size, nd.Files, nd.Dirs, nd.Err = res.size, res.files, res.dirs, res.err
					// send update for this child with computed totals
					ch <- childUpdateMsg{parent: path, child: nd, token: token}
				}(child)
			} else {
				fi, err := e.Info()
				if err == nil {
					child.Size = fi.Size()
					child.Files = 1
				}
				mu.Lock()
				childs = append(childs, child)
				mu.Unlock()
				// immediate update for files
				ch <- childUpdateMsg{parent: path, child: child, token: token}
			}
		}

		wg.Wait()

		// aggregate totals
		var total, files, dirs int64
		var lastErr error
		for _, c := range childs {
			total += c.Size
			files += c.Files
			dirs += c.Dirs
			if c.Err != nil {
				lastErr = c.Err
			}
		}
		n := &Node{Name: filepath.Base(path), Path: path, Children: childs, Size: total, Files: files, Dirs: dirs, Err: lastErr, Scanned: true}
		cache.Store(path, n)
		ch <- scanDoneMsg{node: n, token: token}
	}(useFastCache)

	return scanReaderCmd(ch)
}

func debounceCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return flushUpdatesMsg{} })
}

func (m *model) setTableRowsFromNode(n *Node) {
	rows := make([]table.Row, 0, len(n.Children))
	// If there are no children yet and the folder is still being scanned,
	// show a subtle placeholder row so the user sees the state.
	if len(n.Children) == 0 && (!n.Scanned || m.loading) {
		ph := lipgloss.NewStyle().Faint(true).Render(".. scanning ..")
		rows = append(rows, table.Row{ph, "", "", "", "", ""})
		m.tbl.SetRows(rows)
		if len(rows) > 0 {
			m.tbl.SetCursor(0)
		}
		return
	}
	// sort
	switch m.sort {
	case sortByName:
		sort.Slice(n.Children, func(i, j int) bool { return strings.ToLower(n.Children[i].Name) < strings.ToLower(n.Children[j].Name) })
	default: // size desc
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Size > n.Children[j].Size })
	}
	var total int64
	// sort directories with unknown size (Size<0) to the bottom
	sort.SliceStable(n.Children, func(i, j int) bool {
		ai, aj := n.Children[i], n.Children[j]
		// unknown sizes go last
		if ai.Size < 0 && aj.Size >= 0 {
			return false
		}
		if aj.Size < 0 && ai.Size >= 0 {
			return true
		}
		// otherwise apply configured sort
		if m.sort == sortByName {
			return strings.ToLower(ai.Name) < strings.ToLower(aj.Name)
		}
		return ai.Size > aj.Size
	})

	for _, c := range n.Children {
		total += c.Size
	}
	for _, c := range n.Children {
		pct := 0.0
		// Treat unknown sizes as zero for percent calculations
		sz := c.Size
		if sz < 0 {
			sz = 0
		}
		if total > 0 {
			pct = float64(sz) / float64(maxInt64(total, 1))
		}
		// detect if child is a directory by stat (handles empty dirs)
		isDir := false
		if fi, err := os.Stat(c.Path); err == nil {
			isDir = fi.IsDir()
		}

		displayName := fmt.Sprintf("%s %s", iconFor(c.Name, isDir), c.Name)
		sizeStr := ""
		if c.Size < 0 {
			// per-row spinner frame while scanning
			if len(spinnerFrames) > 0 {
				sizeStr = spinnerFrames[m.loadingFrame%len(spinnerFrames)]
			} else {
				sizeStr = "scanning"
			}
		} else {
			sizeStr = humanBytes(c.Size)
		}

		rows = append(rows, table.Row{
			displayName,
			sizeStr,
			fmt.Sprintf("%d", c.Files),
			fmt.Sprintf("%d", c.Dirs),
			fmt.Sprintf("%5.1f%%", pct*100),
			bar(pct, 18),
		})
	}
	// preserve cursor position across updates to avoid jumping to top
	prev := m.tbl.Cursor()
	m.tbl.SetRows(rows)
	if len(rows) > 0 {
		if prev < 0 {
			prev = 0
		}
		if prev >= len(rows) {
			prev = len(rows) - 1
		}
		m.tbl.SetCursor(prev)
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case childUpdateMsg:
		// Ignore child updates from stale scans
		if msg.token != m.scanToken {
			return m, scanReaderCmd(m.scanCh)
		}
		// If current is nil or different path, ensure we have a node placeholder
		curPath := m.breadcrumbs[len(m.breadcrumbs)-1]
		if m.current == nil || m.current.Path != curPath {
			m.current = &Node{Name: filepath.Base(curPath), Path: curPath, Children: []*Node{}, Scanned: false}
		}

		// merge or append child
		merged := false
		for i, c := range m.current.Children {
			if c.Path == msg.child.Path {
				m.current.Children[i] = msg.child
				merged = true
				break
			}
		}
		if !merged {
			m.current.Children = append(m.current.Children, msg.child)
		}

		// recompute totals treating unknown sizes as zero
		var total, files, dirs int64
		for _, c := range m.current.Children {
			sz := c.Size
			if sz > 0 {
				total += sz
			}
			files += c.Files
			dirs += c.Dirs
		}
		m.current.Size = total
		m.current.Files = files
		m.current.Dirs = dirs

		// update cache partially (store current snapshot)
		cache.Store(curPath, m.current)

		// mark pending updates and start debounce timer if not active
		m.pendingUpdates = true
		if !m.debounceActive {
			m.debounceActive = true
			// start debounce timer (use model duration if set, else 100ms)
			d := m.debounceDur
			if d == 0 {
				d = 100 * time.Millisecond
			}
			return m, tea.Batch(scanReaderCmd(m.scanCh), debounceCmd(d))
		}
		return m, scanReaderCmd(m.scanCh)

	case flushUpdatesMsg:
		if m.pendingUpdates {
			m.setTableRowsFromNode(m.current)
			m.pendingUpdates = false
		}
		m.debounceActive = false
		return m, scanReaderCmd(m.scanCh)

	case loadingTickMsg:
		// advance per-row spinner frame
		if len(spinnerFrames) > 0 {
			m.loadingFrame = (m.loadingFrame + 1) % len(spinnerFrames)
		}
		// if no pending updates, refresh rows so spinner frames update in the table
		if !m.pendingUpdates && m.current != nil {
			m.setTableRowsFromNode(m.current)
		}
		return m, loadingTicker()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.reflowColumns()
		// adjust table height to fill remaining space (reserve lines for header/status/footer)
		// header ~1, status ~1, footer ~1, plus some padding
		tableHeight := maxvalue(3, m.height-6)
		m.tbl.SetHeight(tableHeight)
		return m, nil

	case tea.KeyMsg:
		// If a confirmation modal is open, handle modal keys first
		if m.confirmDelete {
			switch msg.String() {
			case "left", "h":
				m.confirmFocus = 0
				return m, nil
			case "right", "l":
				m.confirmFocus = 1
				return m, nil
			case "tab":
				m.confirmFocus = (m.confirmFocus + 1) % 2
				return m, nil
			case "enter":
				if m.confirmFocus == 0 {
					// yes: delete
					if m.deletePath != "" {
						ti, err := moveToTrash(m.deletePath)
						m.confirmDelete = false
						if err != nil {
							m.deletePath = ""
							m.status = "⚠ " + err.Error()
							return m, nil
						}
						// append to trash history for undo/restore
						m.trashHistory = append(m.trashHistory, ti)
						basename := filepath.Base(m.deletePath)
						// Remove the deleted child from the current view without doing a full rescan.
						parent := m.breadcrumbs[len(m.breadcrumbs)-1]
						if m.current != nil && m.current.Path == parent {
							newChildren := make([]*Node, 0, len(m.current.Children))
							for _, c := range m.current.Children {
								if c.Path == m.deletePath {
									continue
								}
								newChildren = append(newChildren, c)
							}
							m.current.Children = newChildren
							// recompute totals
							var total, files, dirs int64
							for _, c := range m.current.Children {
								if c.Size > 0 {
									total += c.Size
								}
								files += c.Files
								dirs += c.Dirs
							}
							m.current.Size = total
							m.current.Files = files
							m.current.Dirs = dirs
							// update cache and refresh table
							cache.Store(parent, m.current)
							m.setTableRowsFromNode(m.current)
							m.deletePath = ""
							m.status = fmt.Sprintf("Deleted %s", basename)
							return m, nil
						}
						// fallback: if current isn't the parent, just clear deletePath and note status
						m.deletePath = ""
						m.status = fmt.Sprintf("Deleted (refresh available for %s)", parent)
						return m, nil
					}
				} else {
					// no: cancel
					m.confirmDelete = false
					m.deletePath = ""
					m.status = "Canceled"
				}
				return m, nil
			case "esc":
				m.confirmDelete = false
				m.deletePath = ""
				m.status = ""
				return m, nil
			default:
				// swallow all other keys while modal is open (modal behavior)
				return m, nil
			}
		}

		// While loading, allow lightweight read-only navigation (arrow keys etc.)
		// but prevent actions that change state (enter, delete, rescan, export, sort).
		if m.loading {
			switch msg.String() {
			case "ctrl+c", "q":
				m.cancel()
				return m, tea.Quit
			case "up", "down", "left", "right", "pgup", "pgdown", "home", "end", "tab":
				// forward navigation keys to the table
				var cmd tea.Cmd
				m.tbl, cmd = m.tbl.Update(msg)
				return m, tea.Batch(cmd, m.spin.Tick)
			default:
				// swallow any other key while loading
				return m, m.spin.Tick
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "enter":
			if m.current == nil || len(m.current.Children) == 0 {
				return m, nil
			}
			idx := m.tbl.Cursor()
			if idx < 0 || idx >= len(m.current.Children) {
				return m, nil
			}
			child := m.current.Children[idx]
			if child == nil {
				return m, nil
			}
			// Only drill into directories (heuristic: has dirs or files from a subtree)
			// If it's a plain file, ignore
			if child.Files == 1 && child.Dirs == 0 && len(child.Children) == 0 {
				return m, nil
			}
			// navigate into folder immediately (show placeholder) then start scan
			m.breadcrumbs = append(m.breadcrumbs, child.Path)
			m.current = &Node{Name: filepath.Base(child.Path), Path: child.Path, Children: []*Node{}, Scanned: false}
			m.setTableRowsFromNode(m.current)
			m.status = fmt.Sprintf("Scanning %s ...", child.Path)
			m.loading = true
			m.loadingStartTime = time.Now()
			return m, tea.Batch(m.spin.Tick, loadingTicker(), m.startIncrementalScan(child.Path))
		case "backspace":
			if len(m.breadcrumbs) > 1 {
				m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
				up := m.breadcrumbs[len(m.breadcrumbs)-1]
				m.current = &Node{Name: filepath.Base(up), Path: up, Children: []*Node{}, Scanned: false}
				m.setTableRowsFromNode(m.current)
				m.status = fmt.Sprintf("Scanning %s ...", up)
				m.loading = true
				m.loadingStartTime = time.Now()
				return m, tea.Batch(m.spin.Tick, loadingTicker(), m.startIncrementalScan(up))
			}
		case "r":
			// rescan current
			cur := m.breadcrumbs[len(m.breadcrumbs)-1]
			// drop from cache so we actually rescan
			cache.Delete(cur)
			m.current = &Node{Name: filepath.Base(cur), Path: cur, Children: []*Node{}, Scanned: false}
			m.setTableRowsFromNode(m.current)
			m.status = fmt.Sprintf("Rescanning %s ...", cur)
			m.loading = true
			m.loadingStartTime = time.Now()
			return m, tea.Batch(m.spin.Tick, loadingTicker(), m.startIncrementalScan(cur))
		case "s":
			m.sort = sortBySize
			if m.current != nil {
				m.setTableRowsFromNode(m.current)
			}
			return m, nil
		case "n":
			m.sort = sortByName
			if m.current != nil {
				m.setTableRowsFromNode(m.current)
			}
			return m, nil
		case "e":
			return m, m.exportCSV()
		case "d":
			// prompt delete for current selection
			if m.current == nil || len(m.current.Children) == 0 {
				return m, nil
			}
			idx := m.tbl.Cursor()
			if idx < 0 || idx >= len(m.current.Children) {
				return m, nil
			}
			sel := m.current.Children[idx]
			m.confirmDelete = true
			m.deletePath = sel.Path
			m.status = fmt.Sprintf("Delete %s?", sel.Name)
			return m, nil
		case "u":
			// undo last delete / restore using trashHistory (LIFO)
			if len(m.trashHistory) == 0 {
				m.status = "Nothing to restore"
				return m, nil
			}
			// peek last
			ti := m.trashHistory[len(m.trashHistory)-1]
			// check undo window
			if m.undoWindow > 0 && time.Since(ti.DeletedAt) > m.undoWindow {
				m.status = "Undo window expired"
				// drop expired item from history
				m.trashHistory = m.trashHistory[:len(m.trashHistory)-1]
				return m, nil
			}
			if err := restoreFromTrash(ti); err != nil {
				m.status = fmt.Sprintf("Restore failed: %v", err)
				return m, nil
			}
			restored := ti.OrigPath
			// pop
			m.trashHistory = m.trashHistory[:len(m.trashHistory)-1]
			m.status = fmt.Sprintf("Restored %s", filepath.Base(restored))
			// if current view is the parent of restored item, rescan it to show restored entry
			if m.current != nil {
				parent := m.current.Path
				if filepath.Dir(restored) == parent {
					cache.Delete(parent)
					m.status += " — refreshing view"
					m.loading = true
					return m, tea.Batch(m.spin.Tick, loadingTicker(), m.startIncrementalScan(parent))
				}
			}
			return m, nil
		case "c", "esc":
			// cancel delete
			if m.confirmDelete {
				m.confirmDelete = false
				m.deletePath = ""
				m.status = "Canceled"
			}
			return m, nil
		}
		// forward other key messages (arrow keys, page up/down) to the table for navigation
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd

	case scanDoneMsg:
		// Ignore completion from stale scans; keep loading state
		if msg.token != m.scanToken {
			cache.Store(msg.node.Path, msg.node)
			return m, nil
		}
		// Only apply the completed scan to the UI if it matches the current breadcrumb path.
		cur := m.breadcrumbs[len(m.breadcrumbs)-1]
		if msg.node.Path == cur {
			m.current = msg.node

			// Always enforce minimum display time to prevent flicker
			elapsed := time.Since(m.loadingStartTime)
			if elapsed < m.loadingMinDuration {
				// Delay clearing the loading state - store the completed scan but keep loading
				remaining := m.loadingMinDuration - elapsed
				return m, tea.Tick(remaining, func(t time.Time) tea.Msg {
					// Create a special completion message that bypasses the minimum time check
					return struct {
						scanDoneMsg
						forceComplete bool
					}{scanDoneMsg: scanDoneMsg{node: msg.node, token: msg.token}, forceComplete: true}
				})
			}

			// Only clear loading state if no other scans are ongoing
			m.ongoingScansMu.Lock()
			ongoing := m.ongoingScans
			scanInProgress := m.scanInProgress
			m.ongoingScansMu.Unlock()

			if ongoing <= 1 && !scanInProgress {
				m.loading = false
				if msg.node.Err != nil {
					m.status = "⚠ " + msg.node.Err.Error()
				} else {
					m.status = fmt.Sprintf("%s — %s (%d files, %d dirs)", msg.node.Path, humanBytes(msg.node.Size), msg.node.Files, msg.node.Dirs)
				}
			} else {
				// Keep loading state and show debug info
				m.status = fmt.Sprintf("Scanning... (ongoing: %d, inProgress: %v)", ongoing, scanInProgress)
			}
			m.setTableRowsFromNode(msg.node)
			return m, nil
		}
		// otherwise cache the result for later; don't clear loading (it may be for another view)
		cache.Store(msg.node.Path, msg.node)
		return m, nil

	case struct {
		scanDoneMsg
		forceComplete bool
	}:
		// Handle forced completion after minimum display time
		if msg.forceComplete && msg.token == m.scanToken {
			cur := m.breadcrumbs[len(m.breadcrumbs)-1]
			if msg.node.Path == cur && m.current != nil {
				// Only clear loading state if no other scans are ongoing
				m.ongoingScansMu.Lock()
				ongoing := m.ongoingScans
				scanInProgress := m.scanInProgress
				m.ongoingScansMu.Unlock()

				if ongoing <= 1 && !scanInProgress {
					m.loading = false
					if msg.node.Err != nil {
						m.status = "⚠ " + msg.node.Err.Error()
					} else {
						m.status = fmt.Sprintf("%s — %s (%d files, %d dirs)", msg.node.Path, humanBytes(msg.node.Size), msg.node.Files, msg.node.Dirs)
					}
				} else {
					// Keep loading state and show debug info
					m.status = fmt.Sprintf("Scanning... (ongoing: %d, inProgress: %v)", ongoing, scanInProgress)
				}
				m.setTableRowsFromNode(msg.node)
				return m, nil
			}
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.status = "⚠ " + msg.err.Error()
		return m, nil

	case rescanMsg:
		cur := m.breadcrumbs[len(m.breadcrumbs)-1]
		m.status = fmt.Sprintf("Rescanning %s ...", cur)
		m.loading = true
		return m, tea.Batch(m.spin.Tick, loadingTicker(), m.startIncrementalScan(cur))

	default:
		// spinner & table updates
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
}

func (m *model) reflowColumns() {
	if m.width <= 0 {
		return
	}
	// Dedicate space: keep numeric columns readable, expand Name & Graph
	// Increase Dirs minInts width so larger directory counts aren't truncated,
	// and slightly reduce the Name minimum to make room on narrower terminals.
	minInts := []int{8, 10, 6, 8, 12, 10} // Name unused index 0, Size=10, Files=6, Dirs=8, %parent=12, Graph=10
	
	// Reserve more space for table formatting (borders, separators, padding)
	// Bubble Tea table adds separators between columns and may have borders
	avail := m.width - 10  // more conservative padding for table formatting

	// Base widths
	nameW := maxvalue(20, avail-(minInts[1]+minInts[2]+minInts[3]+minInts[4]+minInts[5]))
	graphW := maxvalue(12, minInts[5]+(avail-(nameW+minInts[1]+minInts[2]+minInts[3]+minInts[4]+minInts[5])))

	cols := []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Size", Width: minInts[1]},
		{Title: "Files", Width: minInts[2]},
		{Title: "Dirs", Width: minInts[3]},
		{Title: "% of Parent", Width: minInts[4]},
		{Title: "Graph", Width: graphW},
	}
	m.tbl.SetColumns(cols)
}

func (m *model) View() string {
	head := lipgloss.NewStyle().Bold(true).Render("DiskTree TUI — " + m.breadcrumb())
	status := m.status
	if m.loading {
		status = m.spin.View() + " " + status
	}
	foot := lipgloss.NewStyle().Faint(true).Render("↑/↓ move  Enter open  Backspace up  s=size  n=name  r=rescan  e=export CSV  d=delete  u=undo  q=quit")
	body := lipgloss.JoinVertical(lipgloss.Left,
		head,
		m.tbl.View(),
		status,
		foot,
	)

	if m.confirmDelete {
		// Build the modal popup — width clamped to terminal to avoid wrap/clipping
		popupW := 60
		if m.width > 0 {
			popupW = minvalue(popupW, maxvalue(10, m.width-4))
		}
		modalStyle := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2).Width(popupW).Align(lipgloss.Center).Background(lipgloss.Color("0"))
		// buttons
		btnYes := lipgloss.NewStyle().Padding(0, 2)
		btnNo := lipgloss.NewStyle().Padding(0, 2)
		if m.confirmFocus == 0 {
			btnYes = btnYes.Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
		} else {
			btnNo = btnNo.Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
		}
		yes := btnYes.Render(" Yes ")
		no := btnNo.Render(" No ")
		content := lipgloss.JoinHorizontal(lipgloss.Center, m.status)
		footer := lipgloss.JoinHorizontal(lipgloss.Center, yes, " ", no)
		popup := modalStyle.Render(lipgloss.JoinVertical(lipgloss.Center, content, "", footer))

		// If we don't yet know terminal size, fall back to simple body+popup
		if m.width == 0 || m.height == 0 {
			// Use conservative defaults to render a true overlay even before WindowSize
			ow, oh := m.width, m.height
			if ow <= 0 {
				if c := os.Getenv("COLUMNS"); c != "" {
					if v, err := strconv.Atoi(c); err == nil {
						ow = v
					}
				}
				if ow <= 0 {
					ow = 80
				}
			}
			if oh <= 0 {
				if l := os.Getenv("LINES"); l != "" {
					if v, err := strconv.Atoi(l); err == nil {
						oh = v
					}
				}
				if oh <= 0 {
					oh = 24
				}
			}
			return renderOverlay(body, popup, ow, oh)
		}
		return renderOverlay(body, popup, m.width, m.height)
	}

	// show a centered loading overlay while scanning
	if m.loading {
		popupW := 50
		if m.width > 0 {
			popupW = minvalue(popupW, maxvalue(10, m.width-4))
		}
		modalStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2).Width(popupW).Align(lipgloss.Center).Background(lipgloss.Color("0"))
		content := lipgloss.JoinHorizontal(lipgloss.Center, m.spin.View(), " ", m.status)
		popup := modalStyle.Render(content)
		if m.width == 0 || m.height == 0 {
			// Use conservative defaults to render a true overlay even before WindowSize
			ow, oh := m.width, m.height
			if ow <= 0 {
				if c := os.Getenv("COLUMNS"); c != "" {
					if v, err := strconv.Atoi(c); err == nil {
						ow = v
					}
				}
				if ow <= 0 {
					ow = 80
				}
			}
			if oh <= 0 {
				if l := os.Getenv("LINES"); l != "" {
					if v, err := strconv.Atoi(l); err == nil {
						oh = v
					}
				}
				if oh <= 0 {
					oh = 24
				}
			}
			return renderOverlay(body, popup, ow, oh)
		}
		return renderOverlay(body, popup, m.width, m.height)
	}
	// Always return a fixed-size base screen to prevent layout shifts
	{
		ow, oh := m.width, m.height
		if ow <= 0 {
			if c := os.Getenv("COLUMNS"); c != "" {
				if v, err := strconv.Atoi(c); err == nil {
					ow = v
				}
			}
			if ow <= 0 {
				ow = 80
			}
		}
		if oh <= 0 {
			if l := os.Getenv("LINES"); l != "" {
				if v, err := strconv.Atoi(l); err == nil {
					oh = v
				}
			}
			if oh <= 0 {
				oh = 24
			}
		}
		base := lipgloss.Place(maxvalue(1, ow), maxvalue(1, oh), lipgloss.Left, lipgloss.Top, body, lipgloss.WithWhitespaceChars(" "), lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
		return base
	}
}

// renderOverlay composes an overlay popup centered over a full-screen renderings
// of base content, without shifting the layout. It returns a string with exactly
// height lines and width columns (padded as needed).
func renderOverlay(base, popup string, width, height int) string {
	// Create a fixed-size background surface
	screen := lipgloss.Place(
		maxvalue(1, width), maxvalue(1, height),
		lipgloss.Left, lipgloss.Top,
		base,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)

	bgLines := strings.Split(screen, "\n")
	popLines := strings.Split(popup, "\n")

	// Determine popup dimensions
	popW := 0
	for _, l := range popLines {
		if w := lipgloss.Width(l); w > popW {
			popW = w
		}
	}
	popH := len(popLines)

	// Centered 0-based placement
	startRow := 0
	startCol := 0
	if height > 0 {
		startRow = maxvalue(0, (height-popH)/2)
	}
	if width > 0 {
		startCol = maxvalue(0, (width-popW)/2)
	}

	// Compose output lines
	finalLines := make([]string, 0, len(bgLines))
	for i, line := range bgLines {
		if i >= startRow && i < startRow+popH {
			pi := i - startRow
			if pi >= 0 && pi < len(popLines) {
				// Overlay popup content on the background line
				bgLine := line
				popupLine := popLines[pi]
				
				// Ensure background line is at least as wide as needed
				bgWidth := lipgloss.Width(bgLine)
				if bgWidth < width {
					bgLine += strings.Repeat(" ", width-bgWidth)
				}
				
				// Convert to runes for proper character handling
				bgRunes := []rune(bgLine)
				popupRunes := []rune(popupLine)
				
				// Create result line by overlaying popup on background
				resultRunes := make([]rune, len(bgRunes))
				copy(resultRunes, bgRunes)
				
				// Overlay popup content at the calculated position
				endCol := minvalue(len(resultRunes), startCol+len(popupRunes))
				for j, pr := range popupRunes {
					if startCol+j < endCol {
						resultRunes[startCol+j] = pr
					}
				}
				
				ol := string(resultRunes)
				// Ensure line is exactly the right width
				actualWidth := lipgloss.Width(ol)
				if actualWidth < width {
					ol += strings.Repeat(" ", width-actualWidth)
				} else if actualWidth > width {
					// Truncate respecting visual width and Unicode boundaries
					ol = truncateToWidth(ol, width)
					// Add padding if needed after truncation
					actualWidth = lipgloss.Width(ol)
					if actualWidth < width {
						ol += strings.Repeat(" ", width-actualWidth)
					}
				}
				finalLines = append(finalLines, ol)
				continue
			}
		}
		// Keep background but ensure it's properly truncated and padded to width
		bgLine := line
		actualWidth := lipgloss.Width(bgLine)
		if actualWidth > width {
			// Truncate respecting visual width and Unicode boundaries
			bgLine = truncateToWidth(bgLine, width)
			actualWidth = lipgloss.Width(bgLine)
		}
		if actualWidth < width {
			bgLine += strings.Repeat(" ", width-actualWidth)
		}
		finalLines = append(finalLines, bgLine)
	}
	// Ensure we return exactly height lines
	for len(finalLines) < maxvalue(1, height) {
		finalLines = append(finalLines, strings.Repeat(" ", maxvalue(1, width)))
	}
	if len(finalLines) > maxvalue(1, height) {
		finalLines = finalLines[:maxvalue(1, height)]
	}
	return strings.Join(finalLines, "\n")
}

func (m *model) breadcrumb() string {
	return strings.Join(m.breadcrumbs, string(os.PathSeparator))
}

// --------------------------- Helpers ------------------------------

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	d := float64(b)
	u := []string{"KB", "MB", "GB", "TB", "PB"}
	for i := 0; i < len(u); i++ {
		d /= unit
		if d < unit {
			return fmt.Sprintf("%.1f %s", d, u[i])
		}
	}
	return fmt.Sprintf("%.1f %s", d/unit, "EB")
}

var fileIcons = map[string]string{
	"folder":  "📁",
	".pdf":    "📄",
	".xls":    "📊",
	".xlsx":   "📊",
	".csv":    "📑",
	".txt":    "📄",
	".go":     "🟦",
	".md":     "📝",
	".png":    "🖼️",
	".jpg":    "🖼️",
	".zip":    "📦",
	"default": "📄",
}

func iconFor(name string, isDir bool) string {
	if isDir {
		return fileIcons["folder"]
	}
	if ext := strings.ToLower(filepath.Ext(name)); ext != "" {
		if ic, ok := fileIcons[ext]; ok {
			return ic
		}
	}
	return fileIcons["default"]
}

func bar(p float64, width int) string {
	if width <= 0 {
		width = 10
	}
	fill := int(p * float64(width))
	if fill > width {
		fill = width
	}
	return strings.Repeat("█", fill) + strings.Repeat("░", width-fill)
}

func maxvalue(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minvalue(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// truncateToWidth truncates a string to fit within the specified visual width,
// respecting Unicode character boundaries
func truncateToWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	
	runes := []rune(s)
	var result strings.Builder
	
	for _, r := range runes {
		// Check the visual width this rune would add
		testString := result.String() + string(r)
		testWidth := lipgloss.Width(testString)
		
		if testWidth > maxWidth {
			break
		}
		
		result.WriteRune(r)
	}
	
	return result.String()
}

// --------------------------- Trash helpers -----------------------

func getTrashDir() string {
	// Prefer XDG location on Unix-like systems, fallback to home
	if td := os.Getenv("XDG_DATA_HOME"); td != "" {
		return filepath.Join(td, "disktree", "trash")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".local", "share", "disktree", "trash")
	}
	// fallback to current dir ./trash
	return "./.disktree_trash"
}

func uniqueSuffix() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("-%d", time.Now().UnixNano())
	}
	return "-" + hex.EncodeToString(b)
}

// moveToTrash moves the provided path into the trash directory, preserving the basename
// and adding a short unique suffix if necessary.
func moveToTrash(src string) (*TrashItem, error) {
	td := getTrashDir()
	if err := os.MkdirAll(td, 0755); err != nil {
		return nil, err
	}
	base := filepath.Base(src)
	dst := filepath.Join(td, base)
	// if dst exists, add suffix
	if _, err := os.Stat(dst); err == nil {
		dst = dst + uniqueSuffix()
	}
	// try rename first
	if err := os.Rename(src, dst); err == nil {
		// write metadata
		ti := TrashItem{Name: base, TrashPath: dst, OrigPath: src, DeletedAt: time.Now(), IsDir: fiIsDir(src)}
		_ = writeTrashMeta(dst, ti)
		return &ti, nil
	}
	// fallback: copy recursively (for directories) then remove
	fi, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		// simple directory copy
		if err := copyDir(src, dst); err != nil {
			return nil, err
		}
		if err := os.RemoveAll(src); err != nil {
			return nil, err
		}
		ti := TrashItem{Name: base, TrashPath: dst, OrigPath: src, DeletedAt: time.Now(), IsDir: true}
		if err := writeTrashMeta(dst, ti); err != nil {
			return &ti, err
		}
		return &ti, nil
	}
	// file copy
	if err := copyFile(src, dst); err != nil {
		return nil, err
	}
	if err := os.Remove(src); err != nil {
		return nil, err
	}
	// write metadata
	ti := TrashItem{Name: base, TrashPath: dst, OrigPath: src, DeletedAt: time.Now(), IsDir: fi.IsDir()}
	if err := writeTrashMeta(dst, ti); err != nil {
		return &ti, err
	}
	return &ti, nil
}

func fiIsDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func writeTrashMeta(trashPath string, ti TrashItem) error {
	metaPath := trashPath + ".meta.json"
	b, err := json.Marshal(ti)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, b, 0644)
}

// restoreFromTrash moves a trashed item back to its original path. If a file exists at the
// destination, it will add a unique suffix to avoid overwriting.
func restoreFromTrash(ti *TrashItem) error {
	if ti == nil {
		return errors.New("no item to restore")
	}
	dst := ti.OrigPath
	// if dst exists, add suffix
	if _, err := os.Stat(dst); err == nil {
		dst = dst + uniqueSuffix()
	}
	// attempt rename back
	if err := os.Rename(ti.TrashPath, dst); err == nil {
		// remove meta file
		_ = os.Remove(ti.TrashPath + ".meta.json")
		return nil
	}
	// fallback: copy then remove
	fi, err := os.Stat(ti.TrashPath)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := copyDir(ti.TrashPath, dst); err != nil {
			return err
		}
		if err := os.RemoveAll(ti.TrashPath); err != nil {
			return err
		}
		_ = os.Remove(ti.TrashPath + ".meta.json")
		return nil
	}
	if err := copyFile(ti.TrashPath, dst); err != nil {
		return err
	}
	if err := os.Remove(ti.TrashPath); err != nil {
		return err
	}
	_ = os.Remove(ti.TrashPath + ".meta.json")
	return nil
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func(sf *os.File) {
		err := sf.Close()
		if err != nil {

		}
	}(sf)
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func(df *os.File) {
		err := df.Close()
		if err != nil {

		}
	}(df)
	_, err = io.Copy(df, sf)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// --------------------------- Export ------------------------------

func (m *model) exportCSV() tea.Cmd {
	if m.current == nil {
		return func() tea.Msg { return exportDoneMsg{err: errors.New("nothing to export")} }
	}
	path := fmt.Sprintf("du-%s.csv", time.Now().Format("20060102-150405"))
	return func() tea.Msg {
		f, err := os.Create(path)
		if err != nil {
			return exportDoneMsg{err: err}
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {

			}
		}(f)
		w := csv.NewWriter(f)
		defer w.Flush()
		err = w.Write([]string{"Name", "Path", "SizeBytes", "SizeHuman", "Files", "Dirs", "ParentShare%"})
		if err != nil {
			return nil
		}
		var total int64
		for _, c := range m.current.Children {
			total += c.Size
		}
		for _, c := range m.current.Children {
			pct := 0.0
			if total > 0 {
				pct = float64(c.Size) / float64(total) * 100
			}
			_ = w.Write([]string{
				c.Name,
				c.Path,
				fmt.Sprintf("%d", c.Size),
				humanBytes(c.Size),
				fmt.Sprintf("%d", c.Files),
				fmt.Sprintf("%d", c.Dirs),
				fmt.Sprintf("%.1f", pct),
			})
		}
		return exportDoneMsg{path: path}
	}
}

// --------------------------- Styles ------------------------------

func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.NoColor{}).
		Background(lipgloss.Color("57")).
		Bold(false)
	return styles
}

// --------------------------- main ------------------------------

func main() {
	var root string
	var threads int
	var follow bool
	flag.StringVar(&root, "root", ".", "Root path to scan")
	flag.IntVar(&threads, "threads", runtime.GOMAXPROCS(0)*4, "Worker concurrency for size calculation")
	flag.BoolVar(&follow, "follow-symlinks", false, "Follow symbolic links (may cause cycles)")
	var rescanAfterDelete bool
	flag.BoolVar(&rescanAfterDelete, "rescan-after-delete", false, "Automatically rescan parent after deleting an item")
	flag.Parse()

	// Normalize root
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}

	m := initialModel(root, threads, follow)
	m.autoRescanAfterDelete = rescanAfterDelete
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
