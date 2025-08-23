package main

import (
	"context"
	"database/sql"

	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "modernc.org/sqlite"
)

type tickMsg time.Time

type model struct {
	idInput string

	textInput textinput.Model
	table     table.Model
	db        *sql.DB
	ch        driver.Conn
}

func newModel(db *sql.DB, ch driver.Conn) model {
	ti := textinput.New()
	ti.Placeholder = "..."
	ti.Width = 20
	ti.Focus()

	columns := []table.Column{
		{Title: " ", Width: 3},
		{Title: "ID", Width: 5},
		{Title: "Key", Width: 5},
		{Title: "Published Ago", Width: 30},
		{Title: "Inserted Ago (DESC)", Width: 30},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)

	t.SetStyles(s)

	return model{
		textInput: ti,
		table:     t,
		db:        db,
		ch:        ch}
}

// MARK: Init
func (m model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// MARK: Update
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tickMsg:
		m.refresh()
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			var tiCmd tea.Cmd

			m.textInput, tiCmd = m.textInput.Update(msg)
			m.idInput = m.textInput.Value()

			// highlight rows whose ID matches the input
			rows := m.table.Rows()
			for i := range rows {
				if rows[i][1] == m.idInput {
					rows[i][0] = "->"
				}
			}
			m.table.SetRows(rows)

			return m, tea.Batch(tiCmd)
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) refresh() {
	rows, err := m.ch.Query(context.Background(), `SELECT
	id, key,
	formatReadableTimeDelta(timeDiff(created_at, now64())), 
	formatReadableTimeDelta(timeDiff(inserted_at, now64()))
	FROM messages
	ORDER BY inserted_at DESC
	LIMIT 20`)
	if err != nil {
		log.Printf("query: %v", err)
		return
	}
	defer rows.Close()
	var data []table.Row
	for rows.Next() {
		var id, key string
		var created, inserted string

		if err := rows.Scan(&id, &key, &created, &inserted); err != nil {
			log.Printf("scan: %v", err)
			continue
		}

		// if id matches input,
		matcher := ""
		if id == m.idInput {
			matcher = "->"
		}

		data = append(data, table.Row{
			matcher,
			id, key,
			created, inserted,
		})
	}
	m.table.SetRows(data)
}

// MARK: View
func (m model) View() string {
	return "Highlight an ID: " + m.textInput.View() + "\n\n" + m.table.View() + "\nPress q to quit\n"
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
