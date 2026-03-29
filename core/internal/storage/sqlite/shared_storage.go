package sqlite

import (
	"context"
	"database/sql"

	"github.com/SeijiShii/link-self/core/internal/groupshare"
)

type sharedStorage struct {
	db *sql.DB
}

func (s *sharedStorage) PutShared(ctx context.Context, record *groupshare.SharedRecord) error {
	deleted := 0
	if record.Deleted {
		deleted = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO shared_records (channel, id, topic, group_id, did, timestamp, body, deleted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Channel, record.ID, record.Topic, record.GroupID, record.DID,
		record.Timestamp, record.Body, deleted)
	return err
}

func (s *sharedStorage) GetShared(ctx context.Context, channel, id string) (*groupshare.SharedRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT channel, id, topic, group_id, did, timestamp, body, deleted
		 FROM shared_records WHERE channel = ? AND id = ?`, channel, id)
	return scanSharedRecord(row)
}

func (s *sharedStorage) GetTimestamp(ctx context.Context, channel, id string) (int64, error) {
	var ts int64
	err := s.db.QueryRowContext(ctx,
		`SELECT timestamp FROM shared_records WHERE channel = ? AND id = ?`, channel, id).Scan(&ts)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return ts, err
}

func (s *sharedStorage) DeleteShared(ctx context.Context, channel, id string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM shared_records WHERE channel = ? AND id = ?`, channel, id)
	return err
}

func (s *sharedStorage) ListByChannel(ctx context.Context, channel string) ([]*groupshare.SharedRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT channel, id, topic, group_id, did, timestamp, body, deleted
		 FROM shared_records WHERE channel = ?`, channel)
	if err != nil {
		return nil, err
	}
	return scanSharedRecords(rows)
}

func (s *sharedStorage) ListByGroup(ctx context.Context, groupID string) ([]*groupshare.SharedRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT channel, id, topic, group_id, did, timestamp, body, deleted
		 FROM shared_records WHERE group_id = ?`, groupID)
	if err != nil {
		return nil, err
	}
	return scanSharedRecords(rows)
}

func (s *sharedStorage) ListByChannelAndTopic(ctx context.Context, channel, topic string) ([]*groupshare.SharedRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT channel, id, topic, group_id, did, timestamp, body, deleted
		 FROM shared_records WHERE channel = ? AND topic = ?`, channel, topic)
	if err != nil {
		return nil, err
	}
	return scanSharedRecords(rows)
}

func (s *sharedStorage) DeleteExpired(ctx context.Context, channel string, before int64) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM shared_records WHERE channel = ? AND timestamp < ?`, channel, before)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// scanSharedRecord scans a single row into a SharedRecord.
func scanSharedRecord(row *sql.Row) (*groupshare.SharedRecord, error) {
	r := &groupshare.SharedRecord{}
	var deleted int
	err := row.Scan(&r.Channel, &r.ID, &r.Topic, &r.GroupID, &r.DID, &r.Timestamp, &r.Body, &deleted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Deleted = deleted != 0
	return r, nil
}

// scanSharedRecords scans multiple rows into a SharedRecord slice.
func scanSharedRecords(rows *sql.Rows) ([]*groupshare.SharedRecord, error) {
	defer rows.Close()
	var result []*groupshare.SharedRecord
	for rows.Next() {
		r := &groupshare.SharedRecord{}
		var deleted int
		if err := rows.Scan(&r.Channel, &r.ID, &r.Topic, &r.GroupID, &r.DID, &r.Timestamp, &r.Body, &deleted); err != nil {
			return nil, err
		}
		r.Deleted = deleted != 0
		result = append(result, r)
	}
	return result, rows.Err()
}
