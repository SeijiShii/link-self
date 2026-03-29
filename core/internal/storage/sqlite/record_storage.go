package sqlite

import (
	"context"
	"database/sql"

	"github.com/SeijiShii/link-self/core/internal/syncdb"
)

type recordStorage struct {
	db *sql.DB
}

func (s *recordStorage) Put(ctx context.Context, record *syncdb.SyncRecord) error {
	if record == nil {
		return nil
	}
	deleted := 0
	if record.Deleted {
		deleted = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO sync_records (id, group_id, did, timestamp, body, deleted)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		record.ID, record.GroupID, record.DID, record.Timestamp, record.Body, deleted)
	return err
}

func (s *recordStorage) Get(ctx context.Context, id string) (*syncdb.SyncRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, group_id, did, timestamp, body, deleted FROM sync_records WHERE id = ?`, id)
	r := &syncdb.SyncRecord{}
	var deleted int
	err := row.Scan(&r.ID, &r.GroupID, &r.DID, &r.Timestamp, &r.Body, &deleted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Deleted = deleted != 0
	return r, nil
}

func (s *recordStorage) GetTimestamp(ctx context.Context, id string) (int64, error) {
	var ts int64
	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp FROM sync_records WHERE id = ?`, id).Scan(&ts)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return ts, err
}

func (s *recordStorage) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sync_records WHERE id = ?`, id)
	return err
}

func (s *recordStorage) List(ctx context.Context) ([]*syncdb.SyncRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, did, timestamp, body, deleted FROM sync_records`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*syncdb.SyncRecord
	for rows.Next() {
		r := &syncdb.SyncRecord{}
		var deleted int
		if err := rows.Scan(&r.ID, &r.GroupID, &r.DID, &r.Timestamp, &r.Body, &deleted); err != nil {
			return nil, err
		}
		r.Deleted = deleted != 0
		result = append(result, r)
	}
	return result, rows.Err()
}
