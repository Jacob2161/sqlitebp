# sqlitebp - SQLite Best Practices for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/jacob2161/sqlitebp.svg)](https://pkg.go.dev/github.com/jacob2161/sqlitebp)
[![Coverage](https://img.shields.io/badge/coverage-94%25-brightgreen.svg)](https://github.com/jacob2161/sqlitebp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue.svg)](https://go.dev/)

`sqlitebp` provides an opinionated, production-ready configuration for SQLite databases in Go applications. It implements SQLite best practices with sensible defaults focused on safety, performance, and reliability.

## Installation

```bash
go get github.com/jacob2161/sqlitebp
```

## Examples

### Open for read/write (create if required)

```go
package main

import (
    "fmt"
    "log"

    "github.com/jacob2161/sqlitebp"
)

func main() {
    // Creates or opens the database with best-practice defaults
    // (WAL, foreign keys, busy timeout, NORMAL synchronous, private cache, etc.)
    db, err := sqlitebp.OpenReadWriteCreate("app.db")
    if err != nil { log.Fatal(err) }
    defer db.Close()

    // Create a table (STRICT for stronger type enforcement)
    if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT NOT NULL) STRICT`); err != nil {
        log.Fatal(err)
    }
    // Insert a row
    if _, err := db.Exec(`INSERT INTO users (name) VALUES (?)`, "Alice"); err != nil {
        log.Fatal(err)
    }
    // Query a value
    var count int
    if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
        log.Fatal(err)
    }
    fmt.Println("User rows:", count)
}
```

### Open existing read/write

```go
db, err := sqlitebp.OpenReadWrite("app.db")
if err != nil {
    log.Fatal(err)
}
// e.g. read previously inserted rows
var n int
if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
    log.Fatal(err)
}
```

### Read-only

```go
db, err := sqlitebp.OpenReadOnly("app.db")
if err != nil {
    log.Fatal(err)
}
var n int
if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
    log.Fatal(err)
}
```

### With Options

```go
// Increase busy timeout (seconds), enlarge cache, enforce FULL synchronous
db, err := sqlitebp.OpenReadWriteCreate("app.db",
    sqlitebp.WithBusyTimeoutSeconds(30),
    sqlitebp.WithCacheSizeMiB(64),
    sqlitebp.WithSynchronous("FULL"),
)
if err != nil {
    log.Fatal(err)
}
```

### Adjust Journaling Mode

```go
// Use DELETE journal instead of WAL
db, err := sqlitebp.OpenReadWriteCreate("app.db",
    sqlitebp.WithJournalMode("DELETE"),
)
if err != nil {
    log.Fatal(err)
}
```

### Override temp_store

```go
// Force temporary tables to disk instead of memory (trade performance for lower RAM)
db, err := sqlitebp.OpenReadWriteCreate("app.db",
    sqlitebp.WithTempStore("FILE"),
)
if err != nil {
    log.Fatal(err)
}
```

### Disable PRAGMA optimize

```go
// Disable automatic PRAGMA optimize on new connections
// (Enabled by default unless explicitly disabled)
db, err := sqlitebp.OpenReadWriteCreate("app.db",
    sqlitebp.WithOptimize(false),
)
if err != nil {
    log.Fatal(err)
}
```

(Shared cache is intentionally not supported; private cache is enforced to avoid shared-cache pitfalls.)

## Features & Best Practices

### Default Configuration

The package applies these SQLite best practices automatically:

1. WAL Mode (`_journal_mode=WAL`) except in read-only mode (journal not forced when read-only)
2. Foreign Keys Enabled (`_foreign_keys=true`)
3. Busy Timeout (`_busy_timeout=10000` ms)
4. Private Cache enforced (`cache=private`) - not user configurable
5. Synchronous NORMAL (`_synchronous=NORMAL`)
6. Page Cache 32 MiB (`_cache_size=-32768` KB)
7. Smart Connection Pool (2-8 connections based on GOMAXPROCS)
8. PRAGMA optimize on each connection (disable via `WithOptimize(false)`)
9. Temp Storage in Memory by default (`PRAGMA temp_store=MEMORY`) - overridable via `WithTempStore`

## Platform Support

Optimized and tested for Linux. Other platforms may work but are not a focus.

## Memory Considerations

- Base page cache: ~32 MiB (configurable via `WithCacheSizeMiB`)
- Temp tables & sorts: additional RAM depending on workload (switch to FILE via `WithTempStore("FILE")` if needed)

## Connection Modes

### OpenReadWriteCreate

- Creates database if it doesn't exist
- Full read/write, all optimizations

### OpenReadWrite

- Database must exist
- Full read/write

### OpenReadOnly

- Database must exist
- No writes
- Existing journal mode respected (WAL not forced)
- Other optimizations still applied (foreign keys, busy timeout unaffected)

## Testing

Run tests:

```bash
go test -v
```

## License

MIT. See [LICENSE](LICENSE)

## Contributing

PRs welcome that improve safety, correctness, or performance.
