package sqlitebp

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpen_ValidModes(t *testing.T) {
	tempDir := t.TempDir()
	tests := []struct {
		name     string
		opener   func(string) (*sql.DB, error)
		filename string
		setup    func(string) error
		wantErr  bool
	}{
		{
			name:     "Create new (RWC)",
			opener:   func(p string) (*sql.DB, error) { return OpenReadWriteCreate(p) },
			filename: filepath.Join(tempDir, "test_rwc.db"),
			setup:    nil,
			wantErr:  false,
		},
		{
			name:     "ReadWrite existing",
			opener:   func(p string) (*sql.DB, error) { return OpenReadWrite(p) },
			filename: filepath.Join(tempDir, "test_rw.db"),
			setup: func(path string) error {
				d, err := sql.Open("sqlite3", "file:"+path)
				if err != nil {
					return err
				}
				defer d.Close()
				_, err = d.Exec("CREATE TABLE test (id INTEGER) STRICT")
				return err
			},
			wantErr: false,
		},
		{
			name:     "ReadOnly existing",
			opener:   func(p string) (*sql.DB, error) { return OpenReadOnly(p) },
			filename: filepath.Join(tempDir, "test_ro.db"),
			setup: func(path string) error {
				d, err := sql.Open("sqlite3", "file:"+path)
				if err != nil {
					return err
				}
				defer d.Close()
				_, err = d.Exec("CREATE TABLE test (id INTEGER) STRICT")
				return err
			},
			wantErr: false,
		},
		{
			name:     "ReadWrite missing",
			opener:   func(p string) (*sql.DB, error) { return OpenReadWrite(p) },
			filename: filepath.Join(tempDir, "missing_rw.db"),
			setup:    nil,
			wantErr:  true,
		},
		{
			name:     "ReadOnly missing",
			opener:   func(p string) (*sql.DB, error) { return OpenReadOnly(p) },
			filename: filepath.Join(tempDir, "missing_ro.db"),
			setup:    nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(tt.filename); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			db, err := tt.opener(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error")
					if db != nil {
						db.Close()
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if err := db.Ping(); err != nil {
				t.Errorf("ping: %v", err)
			}
			db.Close()
		})
	}
}

func TestOpen_EmptyFilename(t *testing.T) {
	if _, err := OpenReadWriteCreate("", WithBusyTimeoutSeconds(1)); err == nil || !strings.Contains(err.Error(), "filename cannot be empty") {
		t.Fatalf("expected empty filename error, got %v", err)
	}
}

func TestOpen_DuplicateOptions(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "dup.db")
	if _, err := OpenReadWriteCreate(fn, WithBusyTimeoutSeconds(1), WithBusyTimeoutSeconds(2)); err == nil || !strings.Contains(err.Error(), "_busy_timeout already specified") {
		t.Fatalf("expected duplicate timeout error, got %v", err)
	}
}

func TestOpen_DefaultPragmasApplied(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "pragmas.db")
	db, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		t.Fatalf("optimize: %v", err)
	}

	var ts string
	if err := db.QueryRow("PRAGMA temp_store").Scan(&ts); err != nil {
		t.Fatalf("temp_store: %v", err)
	}
	if ts != "2" {
		t.Errorf("temp_store=%s want 2", ts)
	}
}

func TestConnectHook_PragmasAppliedToEachConnection(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "hook.db")
	db, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT) STRICT"); err != nil {
		t.Fatalf("table: %v", err)
	}
	const workers = 10
	errs := make(chan error, workers)
	stores := make(chan string, workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			var s string
			if e := db.QueryRow("PRAGMA temp_store").Scan(&s); e != nil {
				errs <- e
				return
			}
			stores <- s
			if _, e := db.Exec("INSERT INTO test (value) VALUES (?)", fmt.Sprintf("v-%d", id)); e != nil {
				errs <- e
				return
			}
			errs <- nil
		}(i)
	}
	for i := 0; i < workers; i++ {
		if e := <-errs; e != nil {
			t.Error(e)
		}
	}
	ok := 0
	for i := 0; i < workers; i++ {
		if <-stores == "2" {
			ok++
		}
	}
	if ok != workers {
		t.Errorf("temp_store correct on %d/%d", ok, workers)
	}
}

func TestOpen_DefaultOptionsApplied(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "defaults.db")
	db, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	tests := []struct {
		pragma, want, name string
	}{
		{"foreign_keys", "1", "fk"},
		{"journal_mode", "wal", "journal"},
		{"synchronous", "1", "sync"},
	}
	for _, tc := range tests {
		var v string
		if err := db.QueryRow("PRAGMA " + tc.pragma).Scan(&v); err != nil {
			t.Errorf("pragma %s: %v", tc.pragma, err)
			continue
		}
		if strings.ToLower(v) != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, v, tc.want)
		}
	}
}

func TestOpen_FilenameWithSpecialCharacters(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "file with spaces & symbols.db")
	db, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT) STRICT"); err != nil {
		t.Fatalf("table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO test (name) VALUES (?)", "x"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var c int
	if err := db.QueryRow("SELECT COUNT(*) FROM test").Scan(&c); err != nil || c != 1 {
		t.Fatalf("count got %d err %v", c, err)
	}
}

func TestOpen_ReadOnlyMode(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "ro.db")
	db, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT) STRICT"); err != nil {
		t.Fatalf("table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO test (name) VALUES (?)", "data"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	ro, err := OpenReadOnly(fn)
	if err != nil {
		t.Fatalf("ro open: %v", err)
	}
	defer ro.Close()

	var name string
	if err := ro.QueryRow("SELECT name FROM test WHERE id=1").Scan(&name); err != nil || name != "data" {
		t.Fatalf("read got %s err %v", name, err)
	}
	if _, err := ro.Exec("INSERT INTO test (name) VALUES ('fail')"); err == nil {
		t.Errorf("expected write failure in read-only mode")
	}
}

func TestOpen_ContextTimeout(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "timeout.db")
	start := time.Now()
	db, err := OpenReadWriteCreate(fn)
	dur := time.Since(start)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.Close()
	if dur > 2*time.Second {
		t.Errorf("open slow: %v", dur)
	}
}

func TestOpen_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "concurrent.db")
	db, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT) STRICT"); err != nil {
		t.Fatalf("table: %v", err)
	}
	db.Close()

	rw, _ := OpenReadWrite(fn)
	ro, _ := OpenReadOnly(fn)
	defer rw.Close()
	defer ro.Close()

	if _, err := rw.Exec("INSERT INTO test (value) VALUES ('v')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var c int
	if err := ro.QueryRow("SELECT COUNT(*) FROM test").Scan(&c); err != nil || c != 1 {
		t.Fatalf("count=%d err=%v", c, err)
	}
}

func TestConcurrentAccess_ManyConnections(t *testing.T) {
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "stress.db")
	init, err := OpenReadWriteCreate(fn)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := init.Exec(`CREATE TABLE test_data (id INTEGER PRIMARY KEY AUTOINCREMENT, writer_id INTEGER NOT NULL, value TEXT NOT NULL, timestamp INTEGER NOT NULL) STRICT`); err != nil {
		t.Fatalf("table: %v", err)
	}
	init.Close()

	writerErrs := make(chan error, 100)
	readerErrs := make(chan error, 100)
	writerDone := make(chan bool, 100)
	readerDone := make(chan bool, 100)

	startTime := time.Now()
	for i := 0; i < 100; i++ {
		go func(id int) {
			defer func() { writerDone <- true }()
			w, err := OpenReadWrite(fn)
			if err != nil {
				writerErrs <- err
				return
			}
			defer w.Close()
			for j := 0; j < 10; j++ {
				if _, e := w.Exec("INSERT INTO test_data (writer_id, value, timestamp) VALUES (?, ?, ?)", id, fmt.Sprintf("d-%d-%d", id, j), time.Now().UnixNano()); e != nil {
					writerErrs <- e
					return
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 100; i++ {
		go func(id int) {
			defer func() { readerDone <- true }()
			r, err := OpenReadOnly(fn)
			if err != nil {
				readerErrs <- err
				return
			}
			defer r.Close()
			for j := 0; j < 5; j++ {
				var cnt int
				if e := r.QueryRow("SELECT COUNT(*) FROM test_data").Scan(&cnt); e != nil {
					readerErrs <- e
					return
				}
				if cnt == 0 && time.Since(startTime) > 50*time.Millisecond {
					readerErrs <- fmt.Errorf("no data after 50ms")
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}

	timeout := time.After(30 * time.Second)
	wc, rc := 0, 0
	for wc < 100 || rc < 100 {
		select {
		case <-writerDone:
			wc++
		case <-readerDone:
			rc++
		case e := <-writerErrs:
			t.Errorf("writer: %v", e)
		case e := <-readerErrs:
			t.Errorf("reader: %v", e)
		case <-timeout:
			t.Fatalf("timeout writers=%d readers=%d", wc, rc)
		}
	}

	ver, err := OpenReadOnly(fn)
	if err != nil {
		t.Fatalf("verify open: %v", err)
	}
	defer ver.Close()
	var rows, writers int
	if err := ver.QueryRow("SELECT COUNT(*) FROM test_data").Scan(&rows); err != nil || rows != 1000 {
		t.Fatalf("rows=%d err=%v", rows, err)
	}
	if err := ver.QueryRow("SELECT COUNT(DISTINCT writer_id) FROM test_data").Scan(&writers); err != nil || writers != 100 {
		t.Fatalf("writers=%d err=%v", writers, err)
	}
}

func BenchmarkOpen(b *testing.B) {
	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn := filepath.Join(tempDir, "bench.db")
		os.Remove(fn)
		db, err := OpenReadWriteCreate(fn)
		if err != nil {
			b.Fatalf("open: %v", err)
		}
		db.Close()
	}
}
