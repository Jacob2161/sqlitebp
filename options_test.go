package sqlitebp

import (
	"errors"
	"math"
	"path/filepath"
	"testing"
)

func TestWithCacheSizeMiB(t *testing.T) {
	type testCase struct {
		name    string
		mib     int
		want    string
		wantErr bool
	}
	tests := []testCase{
		{name: "normal", mib: 32, want: "-32768"},
		{name: "zero", mib: 0, wantErr: true},
		{name: "negative", mib: -1, wantErr: true},
		{name: "beyond 32-bit KiB", mib: 2097152, want: "-2147483648"},
	}
	if int64(^uint(0)>>1) == math.MaxInt64 {
		maxMiB := int64(math.MaxInt64 / 1024)
		tests = append(tests,
			testCase{"largest safe", int(maxMiB), "-9223372036854774784", false},
			testCase{"overflow", int(maxMiB + 1), "", true},
		)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &openConfig{params: make(map[string]string)}
			err := WithCacheSizeMiB(tt.mib)(cfg)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidConfigOption) {
					t.Fatalf("got %v, want ErrInvalidConfigOption", err)
				}
				if _, ok := cfg.params["_cache_size"]; ok {
					t.Fatal("invalid size set _cache_size")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.params["_cache_size"]; got != tt.want {
				t.Errorf("_cache_size = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpen_CacheSizeMiB(t *testing.T) {
	db, err := OpenReadWriteCreate(filepath.Join(t.TempDir(), "cache.db"), WithCacheSizeMiB(32))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var size int64
	if err := db.QueryRow("PRAGMA cache_size").Scan(&size); err != nil {
		t.Fatal(err)
	}
	if size != -32768 {
		t.Errorf("PRAGMA cache_size = %d, want -32768", size)
	}
}
