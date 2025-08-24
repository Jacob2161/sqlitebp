// Package sqlitebp provides opinionated best practices for opening SQLite databases
// with sensible defaults for connection pooling, pragmas, and configuration options.
package sqlitebp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var (
	// ErrEmptyFilename indicates an empty filename was supplied.
	ErrEmptyFilename = errors.New("sqlitebp: filename cannot be empty")
	// ErrInvalidMode indicates an internal invalid mode value.
	ErrInvalidMode = errors.New("sqlitebp: invalid mode")
	// ErrOpenFailed indicates the database could not be opened.
	ErrOpenFailed = errors.New("sqlitebp: open failed")
	// ErrPragmaExec indicates a PRAGMA failed during connection initialization.
	ErrPragmaExec = errors.New("sqlitebp: pragma execution failed")
	// ErrPingFailed indicates ping validation failed after opening.
	ErrPingFailed = errors.New("sqlitebp: ping failed")
	// ErrInvalidConfigOption indicates an invalid configuration option was supplied.
	ErrInvalidConfigOption = errors.New("sqlitebp: invalid config option")
)

var defaultOptions = map[string]string{
	// Use a private cache to avoid issues with multiple connections.
	// Shared cache is an obsolete feature that SQLite discourages using.
	// WAL mode provides better concurrent access without shared cache complexity.
	// See: https://www.sqlite.org/sharedcache.html
	"cache": "private",

	// Enable foreign key constraints by default.
	// This is not enabled by default in SQLite for backwards compatibility reasons.
	// See: https://www.sqlite.org/foreignkeys.html
	"_foreign_keys": "true",

	// Busy timeout of 10 seconds to avoid immediate "database is locked" errors.
	// The default is 0 (fail immediately), which is too aggressive for most applications.
	// 10 seconds allows complex queries and transactions to complete while preventing
	// indefinite hangs. Can be overridden based on application needs.
	// See: https://www.sqlite.org/pragma.html#pragma_busy_timeout
	"_busy_timeout": "10000",

	// WAL mode is almost always better than the default DELETE mode.
	// There are some use-cases where DELETE makes sense (e.g. mostly read-only databases),
	// but for most use-cases WAL is better.
	// See: https://www.sqlite.org/wal.html
	"_journal_mode": "WAL",

	// Normal synchronous mode is a good balance between performance and safety.
	// In WAL mode, NORMAL is safe from corruption and equivalent to FULL for
	// application crashes. Only a power loss can cause recently committed transactions
	// to roll back. For most applications using WAL mode, NORMAL is the best choice.
	// See: https://www.sqlite.org/pragma.html#pragma_synchronous
	"_synchronous": "NORMAL", // use uppercase for consistency with SQLite docs

	// Set a reasonable cache size to avoid using too much memory.
	// Negative value means size in KiB, positive means number of pages.
	// When the cache is full, SQLite will evict pages using an LRU algorithm.
	// See: https://www.sqlite.org/pragma.html#pragma_cache_size
	"_cache_size": "-32768", // -32768 means 32 MiB of cache.
}

// Internal symbolic modes.
type internalMode string

const (
	modeReadOnly        internalMode = "ro"
	modeReadWrite       internalMode = "rw"
	modeReadWriteCreate internalMode = "rwc"
)

// OpenReadOnly opens an existing database in read-only mode (journal mode not forced; no writes).
func OpenReadOnly(filename string, opts ...Option) (*sql.DB, error) {
	return openWithMode(filename, modeReadOnly, opts...)
}

// OpenReadWrite opens an existing database with read/write access (must exist).
func OpenReadWrite(filename string, opts ...Option) (*sql.DB, error) {
	return openWithMode(filename, modeReadWrite, opts...)
}

// OpenReadWriteCreate opens or creates a database with full read/write access.
func OpenReadWriteCreate(filename string, opts ...Option) (*sql.DB, error) {
	return openWithMode(filename, modeReadWriteCreate, opts...)
}

func openWithMode(filename string, mode internalMode, opts ...Option) (*sql.DB, error) {
	if filename == "" {
		return nil, ErrEmptyFilename
	}
	// Reject characters that would terminate or confuse the URI path segment.
	// '?' begins query component, '#' is a fragment delimiter; both disallowed inside raw filename here.
	if strings.ContainsAny(filename, "?#") {
		return nil, errors.Join(ErrOpenFailed, fmt.Errorf("filename %q contains reserved characters", filename))
	}

	// Create config with user options applied.
	cfg := &openConfig{
		params:  make(map[string]string),
		pragmas: make(map[string]string),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	// Merge defaults where not already set by user options.
	for k, v := range defaultOptions {
		if _, ok := cfg.params[k]; !ok {
			cfg.params[k] = v
		}
	}
	if _, ok := cfg.pragmas["temp_store"]; !ok {
		cfg.pragmas["temp_store"] = "MEMORY"
	}

	// Set the open mode.
	switch mode {
	case modeReadOnly:
		cfg.params["mode"] = string(modeReadOnly)
		// Never set journal mode in read-only mode, just use the default.
		delete(cfg.params, "_journal_mode")
	case modeReadWrite:
		cfg.params["mode"] = string(modeReadWrite)
	case modeReadWriteCreate:
		cfg.params["mode"] = string(modeReadWriteCreate)
	default:
		return nil, errors.Join(ErrInvalidMode, fmt.Errorf("invalid mode %s", mode))
	}

	// Generate a unique driver name for this open.
	// This could be improved but should be sufficient in practice and it's very simple.
	dName := fmt.Sprintf("sqlite3_bp_%d_%p", time.Now().UnixNano(), cfg)
	sql.Register(dName, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			// Apply PRAGMA optimize if enabled.
			if !cfg.disableOptimize { // run optimize unless disabled
				if _, err := conn.Exec("PRAGMA optimize", nil); err != nil {
					return errors.Join(ErrPragmaExec, fmt.Errorf("failed to execute %q: %w", "PRAGMA optimize", err))
				}
			}
			// Apply pragma.s
			for name, val := range cfg.pragmas {
				stmt := fmt.Sprintf("PRAGMA %s=%s", name, val)
				if _, err := conn.Exec(stmt, nil); err != nil {
					return errors.Join(ErrPragmaExec, fmt.Errorf("failed to execute %q: %w", stmt, err))
				}
			}
			return nil
		},
	})

	// Build the DSN string.
	// See https://www.sqlite.org/draft/uri.html for details.
	var finalOpts []string
	for k, v := range cfg.params {
		finalOpts = append(finalOpts, k+"="+v)
	}
	sort.Strings(finalOpts)
	dsn := "file:" + filename
	if len(finalOpts) > 0 {
		dsn += "?" + strings.Join(finalOpts, "&")
	}

	// Open the database.
	db, err := sql.Open(dName, dsn)
	if err != nil {
		return nil, errors.Join(ErrOpenFailed, fmt.Errorf("failed to open database %q: %w", filename, err))
	}

	// Configure the connection pool with a sensible number of connections.
	// Use between 2 and 8 connections based on GOMAXPROCS.
	// Rarely does SQLite benefit from more than 8 connections due to its
	// locking and concurrency model. Most applications will see diminishing
	// returns beyond 2-4 connections, but we allow up to 8 for highly concurrent
	// workloads on machines with many cores.
	parallelism := min(8, max(2, runtime.GOMAXPROCS(0)))
	db.SetMaxOpenConns(parallelism)
	db.SetMaxIdleConns(parallelism)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	// Validate connectivity and force driver initialization.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.Join(ErrPingFailed, fmt.Errorf("failed to ping database %q: %w", filename, err))
	}
	return db, nil
}
