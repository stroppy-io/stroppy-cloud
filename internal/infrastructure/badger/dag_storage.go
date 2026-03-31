package badger

import (
	"context"
	"encoding/json"
	"fmt"

	badgerdb "github.com/dgraph-io/badger/v4"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
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

func (s *DAGStorage) List(_ context.Context) ([]dag.RunSummary, error) {
	var results []dag.RunSummary
	err := s.db.View(func(txn *badgerdb.Txn) error {
		prefix := []byte(dagPrefix)
		it := txn.NewIterator(badgerdb.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			id := string(item.Key()[len(prefix):])
			err := item.Value(func(val []byte) error {
				var snap dag.Snapshot
				if err := json.Unmarshal(val, &snap); err != nil {
					return nil // skip corrupt entries
				}
				summary := dag.RunSummary{ID: id, Total: len(snap.Nodes), StartedAt: snap.StartedAt, FinishedAt: snap.FinishedAt}
				// Extract db_kind and provider from saved RunConfig if available.
				if snap.State != nil {
					summary.Provider = snap.State.Provider
					if len(snap.State.RunConfig) > 0 {
						var rc struct {
							Database struct {
								Kind string `json:"kind"`
							} `json:"database"`
						}
						if json.Unmarshal(snap.State.RunConfig, &rc) == nil {
							summary.DBKind = rc.Database.Kind
						}
					}
				}
				for _, n := range snap.Nodes {
					switch n.Status {
					case dag.StatusDone:
						summary.Done++
					case dag.StatusFailed:
						summary.Failed++
					default:
						summary.Pending++
					}
				}
				summary.Nodes = snap.Nodes
				results = append(results, summary)
				return nil
			})
			if err != nil {
				continue
			}
		}
		return nil
	})
	return results, err
}

func (s *DAGStorage) Delete(_ context.Context, id string) error {
	return s.db.Update(func(txn *badgerdb.Txn) error {
		return txn.Delete(key(id))
	})
}

func key(id string) []byte {
	return []byte(dagPrefix + id)
}
