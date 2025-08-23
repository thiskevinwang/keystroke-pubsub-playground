package main

import (
	"context"
	"database/sql"

	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "modernc.org/sqlite"
)

type tickMsg time.Time

type model struct {
	table table.Model
	db    *sql.DB
	ch    driver.Conn
}

func newModel(db *sql.DB, ch driver.Conn) model {
	columns := []table.Column{
		{Title: "ID", Width: 5},
		{Title: "Key", Width: 5},
		{Title: "Published", Width: 25},
		{Title: "Inserted (DESC)", Width: 25},
		{Title: "Diff", Width: 19},
	}
	t := table.New(
		table.WithColumns(columns),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)
	return model{
		table: t,
		db:    db,
		ch:    ch}
}

func (m model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.refresh()
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) refresh() {
	rows, err := m.ch.Query(context.Background(), "SELECT id, key, created_at, inserted_at FROM messages ORDER BY inserted_at DESC LIMIT 20")
	if err != nil {
		log.Printf("query: %v", err)
		return
	}
	defer rows.Close()
	var data []table.Row
	for rows.Next() {
		var id, key string
		var created, inserted time.Time
		if err := rows.Scan(&id, &key, &created, &inserted); err != nil {
			log.Printf("scan: %v", err)
			continue
		}
		data = append(data, table.Row{
			id, key,
			created.Format(time.RFC3339), inserted.Format(time.RFC3339),
			inserted.Sub(created).String(),
		})
	}
	m.table.SetRows(data)
}

func (m model) View() string {
	s := lipgloss.NewStyle().Margin(1, 2)
	return s.Render(m.table.View()) + "\nPress q to quit\n"
}

func main() {
	db := newSqlLiteConnection()
	defer db.Close()

	ch := newClickHouseConnection()
	defer ch.Close()

	m := newModel(db, ch)
	if err := tea.NewProgram(m).Start(); err != nil {
		log.Fatal(err)
	}
}

func newSqlLiteConnection() *sql.DB {
	db, err := sql.Open("sqlite", "file:messages.db?_pragma=journal_mode(WAL)&cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func newClickHouseConnection() driver.Conn {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "123456",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	return conn
}
