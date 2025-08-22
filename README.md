# keystroke-pubsub-playground

This repository contains a simple Go playground that simulates a pub/sub
system where keystrokes are published to an in-memory queue and consumed by
worker goroutines. Messages are persisted to a SQLite database and can be
viewed in a Bubble Tea table.

## Running

In one terminal, start the publisher and workers. Batching parameters can be
configured via flags:

```
go run ./cmd/pubsub -batch 10 -flush 1s -workers 2
```

Press keys to enqueue messages. Press ESC or Ctrl+C to exit. Messages include
their creation time and are saved with an additional inserted timestamp when
workers flush batches to the database.

In another terminal, view the stored messages:

```
go run ./cmd/viewer
```

The viewer refreshes every second and shows the most recent rows with both
message creation and insertion timestamps.

## Dependencies

- [modernc.org/sqlite](https://modernc.org/sqlite) for SQLite database
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) for the
  terminal UI
