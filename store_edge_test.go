package asyncx

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("sqlite", "file:asyncx_nf?mode=memory&cache=shared")
	defer db.Close()
	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	store := NewSQLStore(db)
	if rec, err := store.GetByID(context.Background(), "missing"); err == nil {
		// behavior: QueryRow.Scan will return error; our code retries with $1 and then returns err; so err should not be nil
		t.Fatalf("expected error, got rec=%#v err=nil", rec)
	}
}
