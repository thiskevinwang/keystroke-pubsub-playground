package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub/v2"
	pubsubpb "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/grpc/codes"
	_ "modernc.org/sqlite"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gogo/status"

	_ "modernc.org/sqlite"
)

// A simple example that shows how to send activity to Bubble Tea in real-time
// through a channel.

// A message used to indicate that activity has occurred. In the real world (for
// example, chat) this would contain actual data.
type (
	onBatchRead struct {
		BatchSize  string
		AckCount   string
		NackCount  string
		NoResCount string
	}

	incrBatchProgressMsg struct {
		Amount float64
	}
	resetBatchProgressMsg struct{}

	incrFlushProgressMsg struct {
		Amount float64
	}
	resetFlushProgressMsg struct{}
)

const (
	padding  = 2
	maxWidth = 80
)

// https://github.com/charmbracelet/bubbletea?tab=readme-ov-file#the-model
// So let's start by defining our model which will store our application's state.
// It can be any type, but a struct usually makes the most sense.
type model struct {
	spinner       spinner.Model
	batchProgress progress.Model
	flushProgress progress.Model

	lastKeyPress  string
	lastMessageID string

	lastBatchSize  string
	lastAckCount   string
	lastNackCount  string
	lastNoResCount string

	db         *sql.DB
	publisher  *pubsub.Publisher
	subscriber *pubsub.Subscriber
}

/////////////////// --------------------------------------------------------------
// MARK: INIT
/////////////////// --------------------------------------------------------------

// Init can return a Cmd that could perform some initial I/O.
func (m model) Init() tea.Cmd {
	// only need the spinner to tick; key events are handled in Update
	return m.spinner.Tick
}

///////////////////// --------------------------------------------------------------
// MARK: UPDATE
///////////////////// --------------------------------------------------------------

// The update function is called when ”things happen.”
// Its job is to look at what has happened and return an updated model in response.
// It can also return a Cmd to make more things happen, but for now don't worry about that part.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Increment the batch progress bar
	case incrBatchProgressMsg:
		cmd := m.batchProgress.IncrPercent(msg.Amount)
		return m, cmd
	case resetBatchProgressMsg:
		cmd := m.batchProgress.SetPercent(0)
		return m, cmd
	case incrFlushProgressMsg:
		cmd := m.flushProgress.IncrPercent(msg.Amount)
		return m, cmd
	case resetFlushProgressMsg:
		cmd := m.flushProgress.SetPercent(0)
		return m, cmd

		// Is it a subscriber.Receive message?
	case onBatchRead:
		m.lastBatchSize = msg.BatchSize
		m.lastAckCount = msg.AckCount
		m.lastNackCount = msg.NackCount
		m.lastNoResCount = msg.NoResCount
		return m, nil

	// Is it a key press?
	case tea.KeyMsg:
		// Cool, what was the actual key pressed?
		switch msg.String() {
		// These keys should exit the program.
		case "ctrl+c":
			return m, tea.Quit
			// Update the progress bar
		default:
			// Note that you can also use progress.Model.SetPercent to set the
			// percentage value explicitly, too.

			// publish message to pubsub
			ctx := context.Background()
			res := m.publisher.Publish(ctx, &pubsub.Message{
				Data: []byte(msg.String()),
				Attributes: map[string]string{
					"key": "value",
				},
			})

			serverGeneratedID, err := res.Get(ctx)
			if err != nil {
				log.Fatalf("pubsub publish error: %v", err)
				return m, nil
			}

			m.lastMessageID = serverGeneratedID
			m.lastKeyPress = msg.String()
			return m, nil
		}

	// FrameMsg is sent when the progress bar wants to animate itself
	// ie. from progress.IncrPercent
	case progress.FrameMsg:
		model2, cmd2 := m.flushProgress.Update(msg)
		if cmd2 != nil {
			p := model2.(progress.Model)
			m.flushProgress = p
		}

		model1, cmd1 := m.batchProgress.Update(msg)
		if cmd1 != nil {
			p := model1.(progress.Model)
			m.batchProgress = p
		}

		return m, tea.Batch(cmd1, cmd2)

	// Starting a Tea program with a spinner model will perpetually
	// send spinner.TickMsg messages at a regular interval.
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

/////////////////// --------------------------------------------------------------
// MARK: VIEW
/////////////////// --------------------------------------------------------------

// Because the view describes the entire UI of your application, you don’t have to worry
// about redrawing logic and stuff like that.Bubble Tea takes care of it for you.
func (m model) View() string {
	pad := strings.Repeat(" ", padding)

	return "\n" +
		pad + m.spinner.View() + " Batch processor visualizer" + "\n\n" +
		pad + helpStyle(" • This is a visualization for a pub/sub poller, and batch processor.") + "\n" +
		pad + helpStyle("   The processor will flush every (5s) or when it reaches (10 'messages').") + "\n" +
		pad + helpStyle("   Fake disruptions will nack() messages or let messages timeout.") + "\n\n" +
		pad + "Batch: " + m.batchProgress.View() + "\n" +
		pad + "Timer: " + m.flushProgress.View() + "\n" +
		pad + "- Last Key Press: " + helpStyle(m.lastKeyPress) + "\n" +
		pad + "- Last Message ID: " + helpStyle(m.lastMessageID) + "\n\n" +

		pad + "- Last Batch Size: " + helpStyle(m.lastBatchSize) + "\n" +
		pad + "- Last Ack Count: " + infoStyle(m.lastAckCount) + "\n" +
		pad + "- Last Nack Count: " + errorStyle(m.lastNackCount) + "\n" +
		pad + "- Last No Response Count: " + noResStyle(m.lastNoResCount) + "\n\n" +
		pad + helpStyle(" • Press any key to send a message.")

}

////////////////////////////////////////////////////////////////////////

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
var infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1586d6ff")).Render
var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#c71e1eff")).Render
var noResStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Render

var (
	pubsubHost   = "localhost:8085"
	localProject = "local-project"
	topicName    = "keystrokes"
)

// MARK: main
func main() {
	ctx := context.Background()

	// Use pubsub emulator
	os.Setenv("PUBSUB_EMULATOR_HOST", pubsubHost)
	topicFullName := fmt.Sprintf("projects/%s/topics/%s", localProject, topicName)
	subscriptionFullName := fmt.Sprintf("projects/%s/subscriptions/%s", localProject, topicName)

	// log.Println("creating pubsub client")
	// client must be created within 1 second
	cctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	pubSubClient, err := pubsub.NewClient(cctx, localProject)
	if err != nil {
		log.Fatalf("creating pubsub client: %v\nIs the pubsub emulator running?", err)
	}
	// log.Println("pubsub client created")
	defer pubSubClient.Close()

	// log.Println("creating topic...")

	// topic must be created within 3 seconds
	cctx, cancel = context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = pubSubClient.TopicAdminClient.CreateTopic(cctx, &pubsubpb.Topic{Name: topicFullName})
	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists:
			// log.Println("topic already exists")
		default:
			log.Fatalf("creating topic: %v\nIs the pubsub emulator running?", err)
		}
	}

	// log.Println("creating subscription...")

	// subscription must be created within 3 seconds
	_, err = pubSubClient.SubscriptionAdminClient.CreateSubscription(cctx, &pubsubpb.Subscription{
		Name:  subscriptionFullName,
		Topic: topicFullName,
		// https://stackoverflow.com/questions/57123053/cloud-pub-sub-missing-ack-nack-in-callback-not-causing-redelivery
		AckDeadlineSeconds: 1,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.AlreadyExists:
			// log.Println("subscription already exists")
		default:
			log.Fatalf("creating subscription: %v\nIs the pubsub emulator running?", err)
		}
	}

	const maxGoRoutines = 10
	const maxBatch = 100
	const maxDelaySeconds = 5
	const maxDelay = time.Duration(maxDelaySeconds) * time.Second

	const ackRate = 0
	const nackRate = 0

	publisher := pubSubClient.Publisher(topicFullName)
	subscriber := pubSubClient.Subscriber(subscriptionFullName)
	subscriber.ReceiveSettings.NumGoroutines = maxGoRoutines
	subscriber.ReceiveSettings.MaxOutstandingMessages = -1
	subscriber.ReceiveSettings.MaxExtension = -1

	db := newSqlLiteConnection()
	defer db.Close()
	ch := newClickHouseConnection()
	defer ch.Close()

	sqlite := NewSqlLiteDatastore()
	if err := sqlite.InitTable(db); err != nil {
		log.Fatalf("initializing sqlite table: %v", err)
	}

	// ClickHouse
	clickhouse := NewClickHouseDatastore()
	if err := clickhouse.InitTable(ctx, ch); err != nil {
		log.Fatalf("initializing clickhouse table: %v", err)
	}

	// Bubble Tea program
	bp := progress.New(progress.WithDefaultGradient())
	bp.PercentFormat = " %3.0f%% full"
	bp.Full = '0'
	bp.Empty = ' '

	fp := progress.New(progress.WithDefaultGradient())
	fp.PercentFormat = " %3.0f%% done"

	p := tea.NewProgram(model{
		spinner:       spinner.New(spinner.WithSpinner(spinner.Meter), spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5733")))),
		batchProgress: bp,
		flushProgress: fp,
		publisher:     publisher,
		subscriber:    subscriber,
		db:            db,
	})

	// Go routine for pubsub subscriber receiver
	go func() {
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()
		// fmt.Println("Starting subscriber...")

		// batching channel & worker
		msgChannel := make(chan *pubsub.Message, 200)

		// MARK: processBatch
		processBatch := func(batch []*pubsub.Message) {
			// Do your batch processing here (DB write, bulk publish, etc).
			// On success call Ack(); on failure call Nack() for each message.

			ackCount := 0
			nackCount := 0
			noResCount := 0
			batchSize := len(batch)

			placeholders := make([]string, 0, len(batch))
			args := make([]interface{}, 0, len(batch)*4)
			for _, msg := range batch {
				placeholders = append(placeholders, "(?, ?, ?, ?)")
				args = append(args,
					msg.ID,
					string(msg.Data),
					msg.PublishTime.UTC(),
					time.Now().UTC(),
				)
			}
			stmt := fmt.Sprintf("INSERT INTO messages (id, key, created_at, inserted_at) VALUES %s",
				strings.Join(placeholders, ","))
			tx, err := db.Begin()
			if err != nil {
				log.Println("begin tx error:", err)
				for _, m := range batch {
					nackCount++
					m.Nack()
				}
			} else {
				if _, err := tx.Exec(stmt, args...); err != nil {
					_ = tx.Rollback()
					// log.Println("insert batch error:", err)
					for _, m := range batch {
						nackCount++
						m.Nack()
					}
				} else {
					if err := tx.Commit(); err != nil {
						log.Println("commit error:", err)
						for _, m := range batch {
							nackCount++
							m.Nack()
						}
					}
					// on successful commit, fall through to ack/nack simulation below
				}
			}

			// simulate some fake success and failure to cause msgs to be redelivered
			for _, m := range batch {
				if rand.Float32() >= ackRate {
					ackCount++
					m.Ack()
				} else if rand.Float32() > nackRate {
					nackCount++
					m.Nack()
				}
			}
			noResCount += len(batch) - ackCount - nackCount

			p.Send(onBatchRead{
				BatchSize:  fmt.Sprintf("%d", batchSize),
				AckCount:   fmt.Sprintf("%d", ackCount),
				NackCount:  fmt.Sprintf("%d", nackCount),
				NoResCount: fmt.Sprintf("%d", noResCount),
			})
		}

		msgHandler := func(ctx context.Context, msg *pubsub.Message) {
			select {
			case msgChannel <- msg:
				// accepted for batching; ack/nack will happen in processBatch
			case <-ctx.Done():
				// handler context cancelled; nack to be safe
				msg.Nack()
			}
		}

		// MARK: batcher goroutine
		// flush by size or timeout
		go func() {
			batch := make([]*pubsub.Message, 0, maxBatch)

			ticker := time.NewTicker(1 * time.Second)
			timer := time.NewTimer(5 * time.Second)

			defer ticker.Stop()
			defer timer.Stop()

			flush := func() {
				toProcess := make([]*pubsub.Message, len(batch))
				copy(toProcess, batch)
				batch = batch[:0]
				processBatch(toProcess)

				// Clear progress indicators
				p.Send(resetFlushProgressMsg{})
				p.Send(resetBatchProgressMsg{})
			}

			for {
				// quick done check
				select {
				case <-cctx.Done():
					flush()
					return
				default:
				}

				// Main blocking select: prefer messages and ticks; flushTicker already checked above
				select {
				case <-cctx.Done():
					flush()
					return
				case m := <-msgChannel:
					batch = append(batch, m)

					// increment batch progress as a percentage
					amtBatch := 1 / float64(maxBatch)
					p.Send(incrBatchProgressMsg{Amount: amtBatch})

					if len(batch) >= maxBatch {
						// manual flush due to size; restart flush ticker
						flush()
					}
				// Timer wins: don't log the tick at 5s
				case <-timer.C:
					flush()
					if !timer.Stop() {
						// drain if necessary
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(5 * time.Second)
					continue
				case <-ticker.C:
					// add proportional percent each second until the flush time
					amtTick := 1 / float64(maxDelaySeconds)
					p.Send(incrFlushProgressMsg{Amount: amtTick})

				}
			}
		}()

		// MARK: subscriber.Receive
		err := subscriber.Receive(cctx, msgHandler)
		if err != nil {
			log.Fatalf("pubsub subscriber Receive error: %v", err)
		}
		// fmt.Println("Subscriber stopped")
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("could not start program:", err)
		os.Exit(1)
	}
}

func newSqlLiteConnection() *sql.DB {
	db, err := sql.Open("sqlite", "file:messages.db?_pragma=journal_mode(WAL)&cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	return db
}

// Should be a new file
type SqlLiteDatastore struct {
}

func NewSqlLiteDatastore() *SqlLiteDatastore {
	return &SqlLiteDatastore{}
}

func (ds *SqlLiteDatastore) InitTable(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS messages(
		id TEXT, 
		key TEXT, 
		created_at TIMESTAMP, 
		inserted_at TIMESTAMP)
	`); err != nil {
		log.Fatalf("creating table: %v", err)
	}
	return nil
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

type ClickHouseDatastore struct {
}

func NewClickHouseDatastore() *ClickHouseDatastore {
	return &ClickHouseDatastore{}
}

func (ds *ClickHouseDatastore) InitTable(ctx context.Context, conn driver.Conn) error {
	err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS messages(
		id String, 
		key String, 
		created_at DateTime, 
		inserted_at DateTime
	) ENGINE = ReplacingMergeTree()
	ORDER BY id
	`)
	return err
}
