# keystroke-pubsub-playground

This repository contains a simple Go playground that simulates a pub/sub
system where keystrokes are published to an in-memory queue and consumed by
worker goroutines. Messages are persisted to a SQLite database and can be
viewed in a Bubble Tea table.

## Running

In one terminal, start the publisher and workers:

```
go run ./cmd/pubsub
```

Press keys to enqueue messages. Press ESC or Ctrl+C to exit.

In another terminal, view the stored messages:

```
go run ./cmd/viewer
```

The viewer refreshes every second and shows the most recent rows.

## Dependencies

- [modernc.org/sqlite](https://modernc.org/sqlite) for SQLite database
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) for the
  terminal UI
