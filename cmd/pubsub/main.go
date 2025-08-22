package main

import (
	"database/sql"
	"flag"
	"log"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
)

type Message struct {
	ID        string
	Key       string
	CreatedAt time.Time
}

func main() {
	var (
		workers    = flag.Int("workers", 2, "number of workers")
		batchSize  = flag.Int("batch", 10, "messages per batch")
		flushAfter = flag.Duration("flush", time.Second, "flush timeout")
	)
	flag.Parse()

	db, err := sql.Open("sqlite", "file:messages.db?_pragma=journal_mode(WAL)&cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS messages (id TEXT PRIMARY KEY, key TEXT, created_at TIMESTAMP, inserted_at TIMESTAMP)"); err != nil {
		log.Fatalf("creating table: %v", err)
	}

	queue := make(chan Message, 100)
	var wg sync.WaitGroup

	worker := func(id int) {
		defer wg.Done()
		batch := make([]Message, 0, *batchSize)
		timer := time.NewTimer(*flushAfter)
		defer timer.Stop()

		flush := func() {
			if len(batch) == 0 {
				return
			}
			now := time.Now()
			tx, err := db.Begin()
			if err != nil {
				log.Printf("worker %d begin tx: %v", id, err)
				return
			}
			stmt, err := tx.Prepare("INSERT INTO messages (id, key, created_at, inserted_at) VALUES (?, ?, ?, ?)")
			if err != nil {
				log.Printf("worker %d prepare: %v", id, err)
				_ = tx.Rollback()
				return
			}
			for _, msg := range batch {
				if _, err := stmt.Exec(msg.ID, msg.Key, msg.CreatedAt, now); err != nil {
					log.Printf("worker %d nack %s: %v", id, msg.ID, err)
				} else {
					log.Printf("worker %d ack %s", id, msg.ID)
				}
			}
			stmt.Close()
			if err := tx.Commit(); err != nil {
				log.Printf("worker %d commit: %v", id, err)
			}
			batch = batch[:0]
		}

		for {
			select {
			case msg, ok := <-queue:
				if !ok {
					flush()
					return
				}
				batch = append(batch, msg)
				if len(batch) >= *batchSize {
					flush()
					if !timer.Stop() {
						<-timer.C
					}
					timer.Reset(*flushAfter)
				}
			case <-timer.C:
				flush()
				timer.Reset(*flushAfter)
			}
		}
	}

	wg.Add(*workers)
	for i := 0; i < *workers; i++ {
		go worker(i + 1)
	}

	if err := keyboard.Open(); err != nil {
		log.Fatal(err)
	}
	defer keyboard.Close()

	log.Println("Press keys to publish messages. Press ESC or Ctrl+C to quit.")

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			log.Printf("keyboard error: %v", err)
			continue
		}
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			break
		}
		msg := Message{
			ID:        ulid.Make().String(),
			Key:       string(char),
			CreatedAt: time.Now(),
		}
		queue <- msg
	}

	close(queue)
	wg.Wait()
}
