package tui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mnafees/click/internal/db"
	"github.com/mnafees/click/internal/history"
)

type view int

const (
	viewTables view = iota
	viewQuery
	viewResults
)

// dangerous SQL patterns that need confirmation
var dangerousPatterns = []string{"DROP ", "TRUNCATE ", "ALTER ", "DELETE ", "DETACH "}

func isDangerous(query string) bool {
	upper := strings.ToUpper(strings.TrimSpace(query))
	for _, p := range dangerousPatterns {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

type model struct {
	client     *db.Client
	serverInfo db.ServerInfo
	history    *history.History

	// state
	activeView view
	tables     []string
	tableStats map[string]db.TableStats
	cursor     int
	err        error
	loading    bool
	spinner    spinner.Model

	// query editor
	editor    textarea.Model
	savedEdit string

	// results
	result       *db.QueryResult
	resultHeader string
	resultLines  []string
	resultWidth  int
	viewport     viewport.Model
	hScroll      int
	expanded     bool
	utcMode      bool
	explainMode  bool

	// confirm prompt
	confirmQuery string
	confirming   bool

	// database switcher
	databases []string
	dbPicking bool
	dbCursor  int

	// dimensions
	width  int
	height int
}

// messages
type tablesMsg []string
type tableStatsMsg map[string]db.TableStats
type queryResultMsg *db.QueryResult
type errMsg error
type databasesMsg []string
type dbSwitchedMsg struct{ tables []string }
type exportDoneMsg string

func fetchTables(client *db.Client) tea.Cmd {
	return func() tea.Msg {
		tables, err := client.Tables(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return tablesMsg(tables)
	}
}

func fetchTableStats(client *db.Client) tea.Cmd {
	return func() tea.Msg {
		stats, err := client.TableStatsBatch(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return tableStatsMsg(stats)
	}
}

func fetchDatabases(client *db.Client) tea.Cmd {
	return func() tea.Msg {
		dbs, err := client.Databases(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return databasesMsg(dbs)
	}
}

func runQuery(client *db.Client, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := client.Query(context.Background(), query)
		if err != nil {
			return errMsg(err)
		}
		return queryResultMsg(res)
	}
}

func switchDatabase(client *db.Client, name string) tea.Cmd {
	return func() tea.Msg {
		client.SwitchDatabase(name)
		tables, err := client.Tables(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return dbSwitchedMsg{tables: tables}
	}
}

func exportCSV(res *db.QueryResult) tea.Cmd {
	return func() tea.Msg {
		path := fmt.Sprintf("click_export_%d.csv", time.Now().Unix())
		f, err := os.Create(path)
		if err != nil {
			return errMsg(err)
		}
		defer f.Close()
		w := csv.NewWriter(f)
		w.Write(res.Columns)
		for _, row := range res.Rows {
			w.Write(row)
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return errMsg(err)
		}
		return exportDoneMsg(path)
	}
}

func exportJSON(res *db.QueryResult) tea.Cmd {
	return func() tea.Msg {
		path := fmt.Sprintf("click_export_%d.json", time.Now().Unix())
		var records []map[string]string
		for _, row := range res.Rows {
			rec := make(map[string]string, len(res.Columns))
			for i, col := range res.Columns {
				if i < len(row) {
					rec[col] = row[i]
				}
			}
			records = append(records, rec)
		}
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return errMsg(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return errMsg(err)
		}
		return exportDoneMsg(path)
	}
}

func newModel(client *db.Client, info db.ServerInfo) model {
	ta := textarea.New()
	ta.Placeholder = "SELECT * FROM ..."
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Yellow)
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(LightGray)
	ta.SetWidth(80)
	ta.SetHeight(4)

	vp := viewport.New(80, 10)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Yellow)

	return model{
		client:     client,
		serverInfo: info,
		history:    history.New(),
		editor:     ta,
		viewport:   vp,
		spinner:    s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchTables(m.client), fetchTableStats(m.client), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.editor.SetWidth(msg.Width - 4)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 18
		return m, nil

	case tea.MouseMsg:
		if m.activeView == viewResults {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tablesMsg:
		m.tables = msg
		return m, nil

	case tableStatsMsg:
		m.tableStats = msg
		return m, nil

	case queryResultMsg:
		m.result = msg
		m.loading = false
		m.hScroll = 0
		m.rebuildResult()
		m.viewport.GotoTop()
		return m, nil

	case databasesMsg:
		m.databases = msg
		m.dbPicking = true
		m.dbCursor = 0
		return m, nil

	case dbSwitchedMsg:
		m.tables = msg.tables
		m.cursor = 0
		m.dbPicking = false
		m.serverInfo.Database = m.client.Database()
		m.result = nil
		m.tableStats = nil
		return m, fetchTableStats(m.client)

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case exportDoneMsg:
		m.err = nil
		m.result.Duration = 0 // reuse the status line temporarily
		// show export path in error field (it's just a status message)
		m.err = fmt.Errorf("exported to %s", string(msg))
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// pass to sub-components
	if m.activeView == viewQuery && !m.confirming && !m.dbPicking {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// confirmation prompt intercepts all keys
	if m.confirming {
		switch msg.String() {
		case "y", "Y":
			m.confirming = false
			q := m.confirmQuery
			m.confirmQuery = ""
			m.loading = true
			m.history.Add(q)
			return m, runQuery(m.client, q)
		default:
			m.confirming = false
			m.confirmQuery = ""
			return m, nil
		}
	}

	// database picker intercepts all keys
	if m.dbPicking {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.dbPicking = false
			return m, nil
		case "up", "k":
			if m.dbCursor > 0 {
				m.dbCursor--
			}
			return m, nil
		case "down", "j":
			if m.dbCursor < len(m.databases)-1 {
				m.dbCursor++
			}
			return m, nil
		case "enter":
			if len(m.databases) > 0 {
				return m, switchDatabase(m.client, m.databases[m.dbCursor])
			}
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		if m.activeView != viewQuery {
			return m, tea.Quit
		}

	case "tab":
		switch m.activeView {
		case viewTables:
			m.activeView = viewQuery
			m.editor.Focus()
		case viewQuery:
			m.editor.Blur()
			if m.result != nil {
				m.activeView = viewResults
			} else {
				m.activeView = viewTables
			}
		case viewResults:
			m.activeView = viewTables
		}
		return m, nil

	case "up", "k":
		if m.activeView == viewTables {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}
		if m.activeView == viewResults {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case "down", "j":
		if m.activeView == viewTables {
			if m.cursor < len(m.tables)-1 {
				m.cursor++
			}
			return m, nil
		}
		if m.activeView == viewResults {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case "left", "h":
		if m.activeView == viewResults {
			if m.hScroll > 0 {
				m.hScroll -= 4
				if m.hScroll < 0 {
					m.hScroll = 0
				}
				m.updateViewportContent()
			}
			return m, nil
		}

	case "right", "l":
		if m.activeView == viewResults {
			vpWidth := m.width - m.width/4 - 6
			if m.hScroll+vpWidth < m.resultWidth {
				m.hScroll += 4
				m.updateViewportContent()
			}
			return m, nil
		}

	case "enter":
		if m.activeView == viewTables && len(m.tables) > 0 {
			query := fmt.Sprintf("SELECT * FROM %s LIMIT 100", m.tables[m.cursor])
			m.editor.SetValue(query)
			m.loading = true
			m.history.Add(query)
			return m, runQuery(m.client, query)
		}

	case "ctrl+r":
		if m.activeView == viewQuery {
			q := strings.TrimSpace(m.editor.Value())
			if q == "" {
				return m, nil
			}
			m.err = nil
			if isDangerous(q) {
				m.confirming = true
				m.confirmQuery = q
				return m, nil
			}
			actualQuery := q
			if m.explainMode {
				actualQuery = "EXPLAIN " + q
			}
			m.loading = true
			m.history.Add(q)
			m.history.Reset()
			m.savedEdit = ""
			return m, runQuery(m.client, actualQuery)
		}
		return m, nil

	case "ctrl+p":
		if m.activeView == viewQuery {
			if m.savedEdit == "" && m.history.Pos() == -1 {
				m.savedEdit = m.editor.Value()
			}
			if entry, ok := m.history.Prev(); ok {
				m.editor.SetValue(entry)
			}
			return m, nil
		}

	case "ctrl+n":
		if m.activeView == viewQuery {
			if entry, ok := m.history.Next(); ok {
				m.editor.SetValue(entry)
			} else {
				m.editor.SetValue(m.savedEdit)
				m.savedEdit = ""
			}
			return m, nil
		}

	case "ctrl+x":
		if m.result != nil {
			m.expanded = !m.expanded
			m.hScroll = 0
			m.rebuildResult()
			m.viewport.GotoTop()
		}
		return m, nil

	case "ctrl+u":
		if m.result != nil {
			m.utcMode = !m.utcMode
			m.hScroll = 0
			m.rebuildResult()
			m.viewport.GotoTop()
		}
		return m, nil

	case "ctrl+e":
		m.explainMode = !m.explainMode
		return m, nil

	case "ctrl+d":
		if m.activeView == viewTables && len(m.tables) > 0 {
			m.loading = true
			table := m.tables[m.cursor]
			return m, func() tea.Msg {
				res, err := m.client.DescribeTable(context.Background(), table)
				if err != nil {
					return errMsg(err)
				}
				return queryResultMsg(res)
			}
		}

	case "ctrl+b":
		return m, fetchDatabases(m.client)

	case "ctrl+s":
		if m.result != nil {
			return m, exportCSV(m.result)
		}
		return m, nil

	case "ctrl+j":
		if m.result != nil {
			return m, exportJSON(m.result)
		}
		return m, nil
	}

	// pass key to sub-components
	if m.activeView == viewQuery {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	// database picker overlay
	if m.dbPicking {
		return m.viewDBPicker()
	}

	title := TitleStyle.Render(" click ") + "  " +
		DimStyle.Render(fmt.Sprintf("ClickHouse %s | %s:%d/%s | up %s",
			m.serverInfo.Version,
			m.serverInfo.Host, m.serverInfo.Port, m.serverInfo.Database,
			m.serverInfo.Uptime))

	// status hints
	var hints []string
	if m.expanded {
		hints = append(hints, "expanded")
	}
	if m.utcMode {
		hints = append(hints, "UTC")
	}
	if m.explainMode {
		hints = append(hints, "EXPLAIN")
	}

	helpParts := []string{
		"tab: switch",
		"ctrl+r: run",
		"ctrl+d: describe",
		"ctrl+b: databases",
		"ctrl+x: expand",
		"ctrl+u: tz",
		"ctrl+e: explain",
		"ctrl+s: csv",
		"ctrl+j: json",
		"ctrl+p/n: history",
	}
	if len(hints) > 0 {
		helpParts = append(helpParts, "["+strings.Join(hints, ", ")+"]")
	}
	help := DimStyle.Render(strings.Join(helpParts, " • "))

	// confirmation prompt
	if m.confirming {
		prompt := ErrorStyle.Render("Dangerous query: "+m.confirmQuery) + "\n" +
			StatusStyle.Render("Press y to confirm, any other key to cancel")
		return lipgloss.JoinVertical(lipgloss.Left, title, "", prompt)
	}

	// Tables panel
	tableHeader := HeaderStyle.Render("Tables")
	var tableList strings.Builder
	for i, t := range m.tables {
		label := t
		if st, ok := m.tableStats[t]; ok {
			label = fmt.Sprintf("%s  %s  %s", t,
				DimStyle.Render(formatRowCount(st.Rows)),
				DimStyle.Render(formatBytes(st.DiskBytes)))
		}
		if i == m.cursor && m.activeView == viewTables {
			tableList.WriteString(SelectedStyle.Render(" ▸ " + label))
		} else {
			tableList.WriteString(NormalStyle.Render("   " + label))
		}
		tableList.WriteString("\n")
	}
	if len(m.tables) == 0 {
		tableList.WriteString(DimStyle.Render("   (no tables)"))
	}

	tableBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Yellow).
		Padding(0, 1).
		Width(m.width/4 - 2)
	if m.activeView != viewTables {
		tableBorderStyle = tableBorderStyle.BorderForeground(LightGray)
	}
	tablesPanel := tableBorderStyle.Render(tableHeader + "\n" + tableList.String())

	// Right panel: editor + results
	var rightParts []string
	rightParts = append(rightParts, m.editor.View())

	if m.err != nil {
		rightParts = append(rightParts, ErrorStyle.Render("Error: "+m.err.Error()))
	}

	if m.loading {
		rightParts = append(rightParts, m.spinner.View()+" Running query...")
	}

	if m.result != nil {
		timing := StatusStyle.Render(fmt.Sprintf("%d rows (%s) in %s",
			len(m.result.Rows), formatBytes(m.result.BytesRead), m.result.Duration))
		rightParts = append(rightParts, timing)

		if !m.expanded && m.resultHeader != "" {
			vpWidth := m.viewport.Width
			visible := hSlice(m.resultHeader, m.hScroll, vpWidth)
			rightParts = append(rightParts, HeaderStyle.Render(visible))
		}

		resultBorder := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(LightGray)
		if m.activeView == viewResults {
			resultBorder = resultBorder.BorderForeground(Yellow)
		}
		rightParts = append(rightParts, resultBorder.Render(m.viewport.View()))
	}

	rightPanel := lipgloss.JoinVertical(lipgloss.Left, rightParts...)

	body := lipgloss.JoinHorizontal(lipgloss.Top, tablesPanel, "  ", rightPanel)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		body,
		"",
		help,
	)
}

func (m model) viewDBPicker() string {
	title := TitleStyle.Render(" switch database ")
	var list strings.Builder
	for i, d := range m.databases {
		if i == m.dbCursor {
			list.WriteString(SelectedStyle.Render(" ▸ " + d))
		} else {
			list.WriteString(NormalStyle.Render("   " + d))
		}
		list.WriteString("\n")
	}
	help := DimStyle.Render("j/k: navigate • enter: select • esc: cancel")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", list.String(), help)
}

// rebuildResult recomputes resultHeader, resultLines, resultWidth from m.result and m.expanded.
func (m *model) rebuildResult() {
	if m.result == nil || len(m.result.Columns) == 0 {
		m.resultHeader = ""
		m.resultLines = nil
		m.resultWidth = 0
		m.updateViewportContent()
		return
	}
	if m.expanded {
		m.resultHeader = ""
		m.resultLines, m.resultWidth = renderExpandedLines(m.result, m.utcMode)
	} else {
		m.resultHeader, m.resultLines, m.resultWidth = renderTableLines(m.result, m.utcMode)
	}
	m.updateViewportContent()
}

func isDateTimeType(typeName string) bool {
	return strings.HasPrefix(typeName, "DateTime") ||
		typeName == "Date" ||
		typeName == "Date32"
}

func formatDateTimeCell(s string, utcMode bool) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	if utcMode {
		return t.UTC().Format("2006-01-02 15:04:05")
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func columnHeader(name, typeName string) string {
	return name + " (" + typeName + ")"
}

func renderTableLines(res *db.QueryResult, utcMode bool) (string, []string, int) {
	widths := make([]int, len(res.Columns))
	for i, col := range res.Columns {
		typeName := ""
		if i < len(res.ColumnTypes) {
			typeName = res.ColumnTypes[i]
		}
		widths[i] = len(columnHeader(col, typeName))
	}
	for _, row := range res.Rows {
		for i, cell := range row {
			display := cell
			if i < len(res.ColumnTypes) && isDateTimeType(res.ColumnTypes[i]) {
				display = formatDateTimeCell(cell, utcMode)
			}
			if len(display) > widths[i] {
				widths[i] = len(display)
			}
		}
	}

	var hdr strings.Builder
	totalWidth := 0
	for i, col := range res.Columns {
		typeName := ""
		if i < len(res.ColumnTypes) {
			typeName = res.ColumnTypes[i]
		}
		hdr.WriteString(pad(columnHeader(col, typeName), widths[i]))
		totalWidth += widths[i]
		if i < len(res.Columns)-1 {
			hdr.WriteString("  ")
			totalWidth += 2
		}
	}

	lines := make([]string, 0, len(res.Rows))
	for _, row := range res.Rows {
		var b strings.Builder
		for i, cell := range row {
			display := cell
			if i < len(res.ColumnTypes) && isDateTimeType(res.ColumnTypes[i]) {
				display = formatDateTimeCell(cell, utcMode)
			}
			b.WriteString(pad(display, widths[i]))
			if i < len(row)-1 {
				b.WriteString("  ")
			}
		}
		lines = append(lines, b.String())
	}

	return hdr.String(), lines, totalWidth
}

func renderExpandedLines(res *db.QueryResult, utcMode bool) ([]string, int) {
	maxColWidth := 0
	for i, col := range res.Columns {
		typeName := ""
		if i < len(res.ColumnTypes) {
			typeName = res.ColumnTypes[i]
		}
		if w := len(columnHeader(col, typeName)); w > maxColWidth {
			maxColWidth = w
		}
	}

	var lines []string
	maxLineWidth := 0
	for i, row := range res.Rows {
		sep := fmt.Sprintf("-- Record %d --", i+1)
		lines = append(lines, sep)
		if len(sep) > maxLineWidth {
			maxLineWidth = len(sep)
		}
		for j, cell := range row {
			typeName := ""
			if j < len(res.ColumnTypes) {
				typeName = res.ColumnTypes[j]
			}
			display := cell
			if isDateTimeType(typeName) {
				display = formatDateTimeCell(cell, utcMode)
			}
			line := pad(columnHeader(res.Columns[j], typeName), maxColWidth) + " | " + display
			lines = append(lines, line)
			if len(line) > maxLineWidth {
				maxLineWidth = len(line)
			}
		}
	}
	return lines, maxLineWidth
}

func (m *model) updateViewportContent() {
	if len(m.resultLines) == 0 {
		m.viewport.SetContent("")
		return
	}

	vpWidth := m.viewport.Width
	var b strings.Builder
	for i, line := range m.resultLines {
		visible := hSlice(line, m.hScroll, vpWidth)
		if m.expanded && strings.HasPrefix(line, "-- Record") {
			b.WriteString(StatusStyle.Render(visible))
		} else {
			b.WriteString(NormalStyle.Render(visible))
		}
		if i < len(m.resultLines)-1 {
			b.WriteString("\n")
		}
	}
	m.viewport.SetContent(b.String())
}

func hSlice(s string, offset, width int) string {
	runes := []rune(s)
	if offset >= len(runes) {
		return ""
	}
	end := offset + width
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[offset:end])
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatRowCount(n uint64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB rows", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM rows", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK rows", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d rows", n)
	}
}

func Run(client *db.Client) error {
	info, err := client.ServerInfo(context.Background())
	if err != nil {
		return fmt.Errorf("server info: %w", err)
	}
	p := tea.NewProgram(newModel(client, info), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
