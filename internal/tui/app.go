package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mnafees/click/internal/db"
)

type view int

const (
	viewTables view = iota
	viewQuery
	viewResults
)

type model struct {
	client *db.Client

	// state
	activeView view
	tables     []string
	cursor     int
	err        error

	// query editor
	editor textarea.Model

	// results
	result       *db.QueryResult
	resultHeader string
	resultLines  []string
	resultWidth  int
	viewport     viewport.Model
	hScroll      int
	expanded     bool
	utcMode      bool

	// dimensions
	width  int
	height int
}

// messages
type tablesMsg []string
type queryResultMsg *db.QueryResult
type errMsg error

func fetchTables(client *db.Client) tea.Cmd {
	return func() tea.Msg {
		tables, err := client.Tables(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return tablesMsg(tables)
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

func newModel(client *db.Client) model {
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

	return model{
		client:   client,
		editor:   ta,
		viewport: vp,
	}
}

func (m model) Init() tea.Cmd {
	return fetchTables(m.client)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.editor.SetWidth(msg.Width - 4)
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 16
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tablesMsg:
		m.tables = msg
		return m, nil

	case queryResultMsg:
		m.result = msg
		m.hScroll = 0
		m.rebuildResult()
		m.viewport.GotoTop()
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil
	}

	// pass to sub-components
	if m.activeView == viewQuery {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			return m, runQuery(m.client, query)
		}

	case "ctrl+r":
		if m.activeView == viewQuery {
			q := strings.TrimSpace(m.editor.Value())
			if q != "" {
				m.err = nil
				return m, runQuery(m.client, q)
			}
		}
		return m, nil

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

	title := TitleStyle.Render(" click ")
	expandedHint := "off"
	if m.expanded {
		expandedHint = "on"
	}
	tzHint := "local"
	if m.utcMode {
		tzHint = "UTC"
	}
	help := DimStyle.Render("tab: switch view • enter: select table • ctrl+r: run query • ctrl+x: expanded (" + expandedHint + ") • ctrl+u: tz (" + tzHint + ") • arrows: scroll • ctrl+c: quit")

	// Tables panel
	tableHeader := HeaderStyle.Render("Tables")
	var tableList strings.Builder
	for i, t := range m.tables {
		if i == m.cursor && m.activeView == viewTables {
			tableList.WriteString(SelectedStyle.Render(" ▸ " + t))
		} else {
			tableList.WriteString(NormalStyle.Render("   " + t))
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
	if m.activeView == viewTables {
		tableBorderStyle = tableBorderStyle.BorderForeground(Yellow)
	} else {
		tableBorderStyle = tableBorderStyle.BorderForeground(LightGray)
	}
	tablesPanel := tableBorderStyle.Render(tableHeader + "\n" + tableList.String())

	// Right panel: editor + results
	var rightParts []string
	rightParts = append(rightParts, m.editor.View())

	if m.err != nil {
		rightParts = append(rightParts, ErrorStyle.Render("Error: "+m.err.Error()))
	}

	if m.result != nil {
		timing := StatusStyle.Render(fmt.Sprintf("%d rows in %s", len(m.result.Rows), m.result.Duration))
		rightParts = append(rightParts, timing)

		// sticky header (only in table mode, not expanded)
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

// isDateTimeType reports whether a ClickHouse type name represents a date/time value.
func isDateTimeType(typeName string) bool {
	return strings.HasPrefix(typeName, "DateTime") ||
		typeName == "Date" ||
		typeName == "Date32"
}

// formatDateTimeCell parses a UTC RFC3339 datetime string and re-formats it in the
// requested timezone. Falls back to the raw string if parsing fails.
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

// columnHeader builds the display string for a column header (name + type).
func columnHeader(name, typeName string) string {
	return name + " (" + typeName + ")"
}

// renderTableLines returns a header string, data lines, and total width for tabular display.
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

// renderExpandedLines renders results vertically like psql \x mode.
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

// updateViewportContent applies horizontal scroll offset and styling to the result lines.
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

// hSlice extracts a horizontal window from a string.
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

func Run(client *db.Client) error {
	p := tea.NewProgram(newModel(client), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
