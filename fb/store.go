package fb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store is a pure-Go SQLite store with one table per record type. Records are
// upserted on their natural key and the full record kept as a JSON column so no
// field is ever lost.
type Store struct {
	db *sql.DB
}

var storeTables = map[string]string{
	"pages":     "page_id",
	"profiles":  "profile_id",
	"groups":    "group_id",
	"posts":     "post_id",
	"comments":  "comment_id",
	"reactions": "rowid",
	"photos":    "photo_id",
	"videos":    "video_id",
	"events":    "event_id",
}

// OpenStore opens (creating if needed) a SQLite store at path.
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, codeErr(ExitGeneric, "open db: %v", err)
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	for table, key := range storeTables {
		if key == "rowid" {
			if _, err := s.db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (data TEXT NOT NULL)`, table)); err != nil {
				return codeErr(ExitGeneric, "create %s: %v", table, err)
			}
			continue
		}
		q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s TEXT PRIMARY KEY, data TEXT NOT NULL)`, table, key)
		if _, err := s.db.Exec(q); err != nil {
			return codeErr(ExitGeneric, "create %s: %v", table, err)
		}
	}
	return nil
}

// Upsert stores a record into the named table keyed by id.
func (s *Store) Upsert(table, id string, record any) error {
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	key := storeTables[table]
	if table == "" || key == "" {
		return codeErr(ExitGeneric, "unknown table %q", table)
	}
	if key == "rowid" {
		_, err = s.db.Exec(fmt.Sprintf(`INSERT INTO %s (data) VALUES (?)`, table), string(b))
		return err
	}
	q := fmt.Sprintf(`INSERT INTO %s (%s, data) VALUES (?, ?)
		ON CONFLICT(%s) DO UPDATE SET data=excluded.data`, table, key, key)
	_, err = s.db.Exec(q, id, string(b))
	return err
}

// Query runs an arbitrary SQL query and returns column names and string rows.
func (s *Store) Query(q string) ([]string, [][]string, error) {
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, nil, codeErr(ExitGeneric, "query: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var out [][]string
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		rec := make([]string, len(cols))
		for i, v := range raw {
			switch t := v.(type) {
			case nil:
				rec[i] = ""
			case []byte:
				rec[i] = string(t)
			default:
				rec[i] = fmt.Sprintf("%v", t)
			}
		}
		out = append(out, rec)
	}
	return cols, out, rows.Err()
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }
