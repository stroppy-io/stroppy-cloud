package badger

import (
	"context"
	"encoding/json"
	"fmt"

	badgerdb "github.com/dgraph-io/badger/v4"

	"github.com/stroppy-io/hatchet-workflow/internal/core/dag"
)

const dagPrefix = "dag:"

// DAGStorage implements dag.Storage backed by BadgerDB.
type DAGStorage struct {
	db *badgerdb.DB
}

// NewDAGStorage creates a new DAGStorage.
func NewDAGStorage(db *badgerdb.DB) *DAGStorage {
	return &DAGStorage{db: db}
}

func (s *DAGStorage) Save(_ context.Context, id string, snap *dag.Snapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("badger dag storage: marshal: %w", err)
	}

	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Set(key(id), data)
	})
}

func (s *DAGStorage) Load(_ context.Context, id string) (*dag.Snapshot, error) {
	var snap dag.Snapshot

	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(key(id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &snap)
		})
	})

	if err == badgerdb.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("badger dag storage: load: %w", err)
	}

	return &snap, nil
}

func key(id string) []byte {
	return []byte(dagPrefix + id)
}
