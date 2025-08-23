# keystroke-pubsub-playground

## Quickstart

In 3 terminals:

```console
user@~: $ gcloud beta emulators pubsub start --project=local
```

```console
user@~: $ go run ./cmd/pubsub/publisher.go


  ▱▱▱ Batch processor visualizer

   • This is a visualization for a pub/sub poller, and batch processor.
     The processor will flush every (5s) or when it reaches (10 'messages').
     Fake disruptions will nack() messages or let messages timeout.

  Batch:                                  1% full
  Timer: ███████████░░░░░░░░░░░░░░░░░░░  38% done
  - Last Key Press: a
  - Last Message ID: 25

  - Last Batch Size: 14
  - Last Ack Count: 14
  - Last Nack Count: 0
  - Last No Response Count: 0
```

```console
user@~: $ go run ./cmd/viewer/main.go

   ID     Key    Published                  Inserted (DESC)            Diff         
  ──────────────────────────────────────────────────────────────────────────────────
   14     s      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       2.754136s    
   12     d      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       2.860135s    
   13     a      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       2.767135s    
   11     s      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       2.982133s    
   9      d      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.081132s    
   10     a      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.017132s    
   8      s      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.189131s    
   7      a      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.20113s     
   5      s      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.356128s    
   6      d      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.295128s    
   3      d      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.473126s    
   4      a      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.401126s    
   2      s      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.577125s    
   1      a      2025-08-23T11:57:43Z       2025-08-23T11:57:46Z       3.609119s    
   1589   a      2025-08-23T11:32:01Z       2025-08-23T11:32:02Z       166.166ms    
   1587   a      2025-08-23T11:32:01Z       2025-08-23T11:32:02Z       235.165ms    
   1588   a      2025-08-23T11:32:01Z       2025-08-23T11:32:02Z       207.165ms    
   1585   a      2025-08-23T11:32:01Z       2025-08-23T11:32:02Z       307.164ms    
   1586   a      2025-08-23T11:32:01Z       2025-08-23T11:32:02Z       273.164ms    
   1582   a      2025-08-23T11:32:01Z       2025-08-23T11:32:02Z       408.163ms
```

## Dependencies

- [Pub/Sub Emulator](https://cloud.google.com/pubsub/docs/emulator) for local development
- [modernc.org/sqlite](https://modernc.org/sqlite) for SQLite database
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) for the
  terminal UI
