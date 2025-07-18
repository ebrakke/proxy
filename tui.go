package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/evertras/bubble-table/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type model struct {
	proxyManager *ProxyManager
	table        table.Model
	width        int
	height       int
}

func initialModel(pm *ProxyManager) model {
	columns := []table.Column{
		table.NewColumn("port", "Port", 6),
		table.NewColumn("description", "Description", 20),
		table.NewColumn("status", "Status", 10),
		table.NewColumn("active", "Active", 6),
		table.NewColumn("total", "Total", 6),
		table.NewColumn("data", "Data", 8),
		table.NewColumn("last_activity", "Last Activity", 13),
	}

	t := table.New(columns).
		WithRows([]table.Row{}).
		HeaderStyle(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)).
		WithBaseStyle(lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("238")).
			Foreground(lipgloss.Color("252"))).
		Focused(true)

	return model{
		proxyManager: pm,
		table:        t,
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table = m.table.WithTargetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tickMsg:
		m.table = m.updateTableData()
		return m, tickCmd()
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		MarginBottom(1)
	
	header := headerStyle.Render("TCP Proxy Dashboard")
	
	// Stats table
	stats := m.proxyManager.GetStats()
	
	if len(stats) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(2).
			MarginBottom(2)
		
		emptyMsg := emptyStyle.Render("No active proxies found.\nMake sure you have a .proxy.conf file and services running.")
		
		// Footer
		footerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			MarginTop(2)
		
		footer := footerStyle.Render("Press 'q' or Ctrl+C to quit • Updates every 2 seconds")
		
		return lipgloss.JoinVertical(lipgloss.Left, header, emptyMsg, footer)
	}
	
	tableView := m.table.View()
	
	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true).
		MarginTop(1)
	
	footer := footerStyle.Render("Press 'q' or Ctrl+C to quit • Updates every 2 seconds")
	
	return lipgloss.JoinVertical(lipgloss.Left, header, tableView, footer)
}

func (m model) updateTableData() table.Model {
	stats := m.proxyManager.GetStats()
	
	// Sort by activity priority: active connections first, then by last activity
	type sortableStat struct {
		*ProxyStats
	}
	var sortedStats []sortableStat
	for _, stat := range stats {
		sortedStats = append(sortedStats, sortableStat{stat})
	}
	
	sort.Slice(sortedStats, func(i, j int) bool {
		// Put active connections first, then sort by last activity
		if sortedStats[i].ActiveConnections > 0 && sortedStats[j].ActiveConnections == 0 {
			return true
		}
		if sortedStats[i].ActiveConnections == 0 && sortedStats[j].ActiveConnections > 0 {
			return false
		}
		return sortedStats[i].LastActivity.After(sortedStats[j].LastActivity)
	})

	var rows []table.Row
	for _, stat := range sortedStats {
		row := table.NewRow(table.RowData{
			"port":          m.coloredPort(stat.Port),
			"description":   stat.Description,
			"status":        m.coloredStatus(stat.Status),
			"active":        m.coloredActive(stat.ActiveConnections),
			"total":         fmt.Sprintf("%d", stat.TotalConnections),
			"data":          formatBytes(stat.BytesTransferred),
			"last_activity": formatTime(stat.LastActivity),
		})
		rows = append(rows, row)
	}

	return m.table.WithRows(rows)
}

func (m model) coloredPort(port string) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)
	return style.Render(port)
}

func (m model) coloredStatus(status string) string {
	var style lipgloss.Style
	switch status {
	case "Active":
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true)
	case "Starting":
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)
	default:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
	}
	return style.Render(status)
}

func (m model) coloredActive(active int64) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226"))
	if active > 0 {
		style = style.Bold(true)
	}
	return style.Render(fmt.Sprintf("%d", active))
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1fGB", float64(bytes)/(1024*1024*1024))
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	
	diff := time.Since(t)
	if diff < time.Minute {
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	return t.Format("Jan 2 15:04")
}