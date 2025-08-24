package sqlitebp

import (
	"errors"
	"fmt"
	"strings"
)

// openConfig holds user-specified parameters and per-connection pragmas.
// params are translated into DSN key/value pairs.
// pragmas are explicit PRAGMA statements applied via the driver ConnectHook for each connection.
type openConfig struct {
	params          map[string]string
	pragmas         map[string]string
	disableOptimize bool
}

// Option configures database parameters prior to opening.
type Option func(*openConfig) error

// WithOptimize enables or disables running PRAGMA optimize on each new connection (default enabled).
func WithOptimize(enabled bool) Option {
	return func(c *openConfig) error {
		// default is enabled; only store disabled state
		c.disableOptimize = !enabled
		return nil
	}
}

// WithBusyTimeoutSeconds sets the busy timeout (seconds >=0). Translated to _busy_timeout (ms).
func WithBusyTimeoutSeconds(sec int) Option {
	return func(c *openConfig) error {
		if sec < 0 {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("busy timeout must be >= 0"))
		}
		if _, exists := c.params["_busy_timeout"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_busy_timeout already specified"))
		}
		// use int64 for multiplication to avoid overflow on 32-bit platforms for large values
		c.params["_busy_timeout"] = fmt.Sprintf("%d", int64(sec)*1000)
		return nil
	}
}

// WithCacheSizeMiB sets the page cache size in MiB (negative KiB form).
func WithCacheSizeMiB(mib int) Option {
	return func(c *openConfig) error {
		if mib <= 0 {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("cache size must be > 0"))
		}
		if _, exists := c.params["_cache_size"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_cache_size already specified"))
		}
		c.params["_cache_size"] = fmt.Sprintf("-%d", mib*1024)
		return nil
	}
}

// WithJournalMode sets journal mode (ignored in read-only opens where we do not force WAL).
func WithJournalMode(mode string) Option {
	return func(c *openConfig) error {
		if _, exists := c.params["_journal_mode"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_journal_mode already specified"))
		}
		m := strings.ToUpper(mode)
		switch m {
		case "WAL", "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
			c.params["_journal_mode"] = m
		default:
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("invalid journal mode %q", mode))
		}
		return nil
	}
}

// WithSynchronous sets the synchronous level.
func WithSynchronous(level string) Option {
	return func(c *openConfig) error {
		if _, exists := c.params["_synchronous"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_synchronous already specified"))
		}
		l := strings.ToUpper(level)
		switch l {
		case "OFF", "NORMAL", "FULL", "EXTRA":
			c.params["_synchronous"] = l
		default:
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("invalid synchronous level %q", level))
		}
		return nil
	}
}

// WithForeignKeys enables or disables foreign key enforcement.
func WithForeignKeys(enabled bool) Option {
	return func(c *openConfig) error {
		if _, exists := c.params["_foreign_keys"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_foreign_keys already specified"))
		}
		if enabled {
			c.params["_foreign_keys"] = "true"
		} else {
			c.params["_foreign_keys"] = "false"
		}
		return nil
	}
}

// WithTempStore overrides temp_store using a per-connection PRAGMA (DEFAULT, FILE, MEMORY).
// This cannot be reliably set via DSN driver underscore parameter; we apply it through ConnectHook.
func WithTempStore(store string) Option {
	return func(c *openConfig) error {
		if _, exists := c.pragmas["temp_store"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("temp_store already specified"))
		}
		s := strings.ToUpper(store)
		switch s {
		case "DEFAULT", "FILE", "MEMORY":
			c.pragmas["temp_store"] = s
		default:
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("invalid temp_store %q", store))
		}
		return nil
	}
}

// WithMMapSize sets the mmap size in bytes (0 disables memory mapping growth beyond default). Applies via DSN.
func WithMMapSize(bytes int64) Option {
	return func(c *openConfig) error {
		if bytes < 0 {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("mmap size must be >= 0"))
		}
		if _, exists := c.params["_mmap_size"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_mmap_size already specified"))
		}
		c.params["_mmap_size"] = fmt.Sprintf("%d", bytes)
		return nil
	}
}

// WithCaseSensitiveLike toggles case_sensitive_like pragma.
func WithCaseSensitiveLike(enabled bool) Option {
	return func(c *openConfig) error {
		if _, exists := c.params["_case_sensitive_like"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_case_sensitive_like already specified"))
		}
		if enabled {
			c.params["_case_sensitive_like"] = "true"
		} else {
			c.params["_case_sensitive_like"] = "false"
		}
		return nil
	}
}

// WithRecursiveTriggers toggles recursive_triggers.
func WithRecursiveTriggers(enabled bool) Option {
	return func(c *openConfig) error {
		if _, exists := c.params["_recursive_triggers"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_recursive_triggers already specified"))
		}
		if enabled {
			c.params["_recursive_triggers"] = "true"
		} else {
			c.params["_recursive_triggers"] = "false"
		}
		return nil
	}
}

// WithSecureDelete sets secure_delete mode (FAST, ON, OFF).
func WithSecureDelete(mode string) Option {
	return func(c *openConfig) error {
		if _, exists := c.params["_secure_delete"]; exists {
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("_secure_delete already specified"))
		}
		m := strings.ToUpper(mode)
		switch m {
		case "FAST", "ON", "OFF":
			c.params["_secure_delete"] = m
		default:
			return errors.Join(ErrInvalidConfigOption, fmt.Errorf("invalid secure_delete %q", mode))
		}
		return nil
	}
}
