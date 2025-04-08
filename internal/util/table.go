package util

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func RenderTable(headers []string, rows ...[]string) string {
	return table.New().
		Border(lipgloss.Border{}).
		BorderHeader(false).
		BorderTop(false).
		BorderBottom(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle()

			if col != 0 {
				s = s.Padding(0, 2)
			}

			if row == table.HeaderRow {
				s = s.Bold(true)
			}

			return s
		}).
		Headers(headers...).
		Rows(rows...).
		Render()
}
