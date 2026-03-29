package sqlite

import (
	"database/sql"
	"encoding/json"
)

type subscriptionStore struct {
	db *sql.DB
}

func (s *subscriptionStore) SetSubscription(did, channel string, topics []string) error {
	data, err := json.Marshal(topics)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO subscriptions (did, channel, topics) VALUES (?, ?, ?)`,
		did, channel, string(data))
	return err
}

func (s *subscriptionStore) GetSubscription(did, channel string) ([]string, error) {
	var data string
	err := s.db.QueryRow(
		`SELECT topics FROM subscriptions WHERE did = ? AND channel = ?`, did, channel).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var topics []string
	if err := json.Unmarshal([]byte(data), &topics); err != nil {
		return nil, err
	}
	return topics, nil
}

func (s *subscriptionStore) GetAllSubscriptions(did string) (map[string][]string, error) {
	rows, err := s.db.Query(
		`SELECT channel, topics FROM subscriptions WHERE did = ?`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var channel, data string
		if err := rows.Scan(&channel, &data); err != nil {
			return nil, err
		}
		var topics []string
		if err := json.Unmarshal([]byte(data), &topics); err != nil {
			return nil, err
		}
		result[channel] = topics
	}
	return result, rows.Err()
}
