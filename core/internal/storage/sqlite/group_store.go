package sqlite

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/SeijiShii/link-self/core/internal/group"
)

type groupStore struct {
	db *sql.DB
}

func (s *groupStore) CreateGroup(members []string, ownerDIDs []string) (string, error) {
	id := uuid.New().String()
	membersJSON, err := json.Marshal(members)
	if err != nil {
		return "", err
	}
	ownersJSON, err := json.Marshal(ownerDIDs)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO groups_ (id, members, owners) VALUES (?, ?, ?)`,
		id, string(membersJSON), string(ownersJSON))
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *groupStore) GetGroup(groupID string) (*group.Group, error) {
	var membersJSON, ownersJSON string
	err := s.db.QueryRow(
		`SELECT members, owners FROM groups_ WHERE id = ?`, groupID).Scan(&membersJSON, &ownersJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	g := &group.Group{ID: groupID}
	if err := json.Unmarshal([]byte(membersJSON), &g.Members); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(ownersJSON), &g.Owners); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *groupStore) ListGroupIDsForMember(memberDID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT id FROM groups_ WHERE EXISTS (
			SELECT 1 FROM json_each(members) WHERE value = ?
		)`, memberDID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *groupStore) UpdateGroup(groupID string, members []string, ownerDIDs []string) error {
	membersJSON, err := json.Marshal(members)
	if err != nil {
		return err
	}
	ownersJSON, err := json.Marshal(ownerDIDs)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE groups_ SET members = ?, owners = ? WHERE id = ?`,
		string(membersJSON), string(ownersJSON), groupID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return group.ErrGroupNotFound
	}
	return nil
}

func (s *groupStore) DeleteGroup(groupID string) error {
	res, err := s.db.Exec(`DELETE FROM groups_ WHERE id = ?`, groupID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return group.ErrGroupNotFound
	}
	return nil
}
