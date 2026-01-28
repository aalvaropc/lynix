package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Help     lipgloss.Style
	Card     lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		Title:    lipgloss.NewStyle().Bold(true),
		Subtitle: lipgloss.NewStyle().Faint(true),
		Help:     lipgloss.NewStyle().Faint(true),
		Card: lipgloss.NewStyle().
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")),
	}
}
