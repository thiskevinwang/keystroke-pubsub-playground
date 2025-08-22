package main

import (
	"database/sql"
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
	db, err := sql.Open("sqlite", "file:messages.db?_pragma=journal_mode(WAL)&cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS messages (id TEXT PRIMARY KEY, key TEXT, created_at TIMESTAMP)"); err != nil {
		log.Fatalf("creating table: %v", err)
	}

	queue := make(chan Message, 100)
	var wg sync.WaitGroup

	worker := func(id int) {
		defer wg.Done()
		for msg := range queue {
			if _, err := db.Exec("INSERT INTO messages (id, key, created_at) VALUES (?, ?, ?)", msg.ID, msg.Key, msg.CreatedAt); err != nil {
				log.Printf("worker %d nack %s: %v", id, msg.ID, err)
			} else {
				log.Printf("worker %d ack %s", id, msg.ID)
			}
		}
	}

	workers := 2
	wg.Add(workers)
	for i := 0; i < workers; i++ {
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
