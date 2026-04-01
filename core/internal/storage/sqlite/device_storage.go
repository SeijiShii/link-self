package sqlite

import (
	"context"
	"database/sql"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
)

type deviceStorage struct {
	db *sql.DB
}

func (s *deviceStorage) Put(ctx context.Context, table, id string, body []byte, timestamp int64) (uint64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO device_records (tbl, id, body, timestamp) VALUES (?, ?, ?, ?)`,
		table, id, body, timestamp)
	if err != nil {
		return 0, err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO change_log (timestamp, tbl, record_id, op, body) VALUES (?, ?, ?, ?, ?)`,
		timestamp, table, id, int(devicesync.OpPut), body)
	if err != nil {
		return 0, err
	}

	seq, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(seq), nil
}

func (s *deviceStorage) Get(ctx context.Context, table, id string) (*devicesync.Record, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, tbl, body, timestamp FROM device_records WHERE tbl = ? AND id = ?`, table, id)
	r := &devicesync.Record{}
	err := row.Scan(&r.ID, &r.Table, &r.Body, &r.Timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *deviceStorage) Delete(ctx context.Context, table, id string, timestamp int64) (uint64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`DELETE FROM device_records WHERE tbl = ? AND id = ?`, table, id)
	if err != nil {
		return 0, err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO change_log (timestamp, tbl, record_id, op, body) VALUES (?, ?, ?, ?, NULL)`,
		timestamp, table, id, int(devicesync.OpDelete))
	if err != nil {
		return 0, err
	}

	seq, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(seq), nil
}

func (s *deviceStorage) List(ctx context.Context, table string) ([]*devicesync.Record, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tbl, body, timestamp FROM device_records WHERE tbl = ?`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*devicesync.Record
	for rows.Next() {
		r := &devicesync.Record{}
		if err := rows.Scan(&r.ID, &r.Table, &r.Body, &r.Timestamp); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *deviceStorage) GetTimestamp(ctx context.Context, table, id string) (int64, error) {
	var ts int64
	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp FROM device_records WHERE tbl = ? AND id = ?`, table, id).Scan(&ts)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return ts, err
}

func (s *deviceStorage) AppendChange(ctx context.Context, entry *devicesync.ChangeEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO change_log (seq, timestamp, tbl, record_id, op, body) VALUES (?, ?, ?, ?, ?, ?)`,
		entry.Seq, entry.Timestamp, entry.Table, entry.RecordID, int(entry.Op), entry.Body)
	return err
}

func (s *deviceStorage) ChangesSince(ctx context.Context, since uint64) ([]*devicesync.ChangeEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT seq, timestamp, tbl, record_id, op, body FROM change_log WHERE seq > ? ORDER BY seq`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*devicesync.ChangeEntry
	for rows.Next() {
		e := &devicesync.ChangeEntry{}
		var op int
		if err := rows.Scan(&e.Seq, &e.Timestamp, &e.Table, &e.RecordID, &op, &e.Body); err != nil {
			return nil, err
		}
		e.Op = devicesync.Op(op)
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *deviceStorage) LatestSeq(ctx context.Context) (uint64, error) {
	var seq sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(seq) FROM change_log`).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return uint64(seq.Int64), nil
}

func (s *deviceStorage) MinSeq(ctx context.Context) (uint64, error) {
	var seq sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MIN(seq) FROM change_log`).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return uint64(seq.Int64), nil
}

func (s *deviceStorage) TruncateChangeLog(ctx context.Context, minSeq uint64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM change_log WHERE seq < ?`, minSeq)
	return err
}

func (s *deviceStorage) ListTables(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT tbl FROM device_records`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
