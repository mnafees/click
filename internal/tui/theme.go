package tui

import "github.com/charmbracelet/lipgloss"

// ClickHouse brand colors.
var (
	Yellow    = lipgloss.Color("#FADB4F")
	DarkGray  = lipgloss.Color("#1C1C1C")
	LightGray = lipgloss.Color("#A0A0A0")
	White     = lipgloss.Color("#FFFFFF")
	Red       = lipgloss.Color("#FF5555")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(DarkGray).
			Background(Yellow).
			Padding(0, 1)

	StatusStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Yellow).
			Padding(0, 1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(DarkGray).
			Background(Yellow).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().
			Foreground(White)

	DimStyle = lipgloss.NewStyle().
			Foreground(LightGray)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(Yellow).
			Bold(true).
			Underline(true)
)
