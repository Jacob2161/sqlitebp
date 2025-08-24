package sqlitebp_test

import (
	"fmt"
	"log"
	"os"

	"github.com/jacob2161/sqlitebp"
)

func Example() {
	os.Remove("example.db")
	os.Remove("example.db-shm")
	os.Remove("example.db-wal")
	db, err := sqlitebp.OpenReadWriteCreate("example.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	defer func() { os.Remove("example.db"); os.Remove("example.db-shm"); os.Remove("example.db-wal") }()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT NOT NULL) STRICT`)
	if err != nil {
		log.Fatal(err)
	}
	res, err := db.Exec("INSERT INTO users (name) VALUES (?)", "Alice")
	if err != nil {
		log.Fatal(err)
	}
	id, _ := res.LastInsertId()
	fmt.Printf("Inserted user with ID: %d\n", id)
}

func Example_customOptions() {
	os.Remove("custom.db")
	os.Remove("custom.db-shm")
	os.Remove("custom.db-wal")
	db, err := sqlitebp.OpenReadWriteCreate("custom.db",
		sqlitebp.WithBusyTimeoutSeconds(30),
		sqlitebp.WithCacheSizeMiB(64),
		sqlitebp.WithSynchronous("FULL"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	defer func() { os.Remove("custom.db"); os.Remove("custom.db-shm"); os.Remove("custom.db-wal") }()
	fmt.Println("Database opened with custom options")
}

func Example_readOnly() {
	os.Remove("readonly.db")
	os.Remove("readonly.db-shm")
	os.Remove("readonly.db-wal")
	// Ensure exists
	db, err := sqlitebp.OpenReadWriteCreate("readonly.db")
	if err != nil {
		log.Fatal(err)
	}
	db.Close()
	ro, err := sqlitebp.OpenReadOnly("readonly.db")
	if err != nil {
		log.Fatal(err)
	}
	defer ro.Close()
	var count int
	if err := ro.QueryRow("SELECT COUNT(*) FROM sqlite_master").Scan(&count); err != nil {
		log.Fatal(err)
	}
	if _, err := ro.Exec("CREATE TABLE test (id INTEGER) STRICT"); err != nil {
		fmt.Println("Write failed as expected in read-only mode")
	}
	defer func() { os.Remove("readonly.db"); os.Remove("readonly.db-shm"); os.Remove("readonly.db-wal") }()
}
