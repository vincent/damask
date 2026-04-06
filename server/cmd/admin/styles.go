package main

import "github.com/charmbracelet/lipgloss"

var (
	// Color palette
	ColorPrimary = lipgloss.Color("63")  // soft purple
	ColorMuted   = lipgloss.Color("241") // gray
	ColorSuccess = lipgloss.Color("78")  // green
	ColorWarning = lipgloss.Color("214") // amber
	ColorDanger  = lipgloss.Color("196") // red
	ColorAccent  = lipgloss.Color("86")  // teal

	// Title
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorMuted)

	// Tabs
	TabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(ColorPrimary).
			Padding(0, 2)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 2)

	// Stat boxes
	StatBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 2).
			Width(20)

	StatBoxDangerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorDanger).
				Padding(0, 2).
				Width(20)

	StatBoxWarningStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorWarning).
				Padding(0, 2).
				Width(20)

	StatValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Align(lipgloss.Center)

	StatValueDangerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorDanger).
				Align(lipgloss.Center)

	StatValueWarningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWarning).
				Align(lipgloss.Center)

	StatLabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Align(lipgloss.Center)

	// Table
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorMuted)

	TableSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(ColorPrimary)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Status / errors
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	ItalicMutedStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Modal / overlay
	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Width(64)

	ModalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	ModalKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorMuted).
			Width(12)

	ModalValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	DimColor = lipgloss.Color("238")

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	// Divider
	DividerStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Search input
	SearchPromptStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)
)
