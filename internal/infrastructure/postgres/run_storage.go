package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
)

type RunStorage struct {
	pool *pgxpool.Pool
}

func NewRunStorage(pool *pgxpool.Pool) *RunStorage {
	return &RunStorage{pool: pool}
}

func (s *RunStorage) Save(ctx context.Context, tenantID, id string, snap *dag.Snapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("run storage: marshal: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO runs (id, tenant_id, snapshot, created_at, updated_at)
         VALUES ($1, $2, $3, NOW(), NOW())
         ON CONFLICT(id, tenant_id) DO UPDATE SET snapshot = excluded.snapshot, updated_at = NOW()`,
		id, tenantID, string(data),
	)
	return err
}

func (s *RunStorage) Load(ctx context.Context, tenantID, id string) (*dag.Snapshot, error) {
	var data string
	err := s.pool.QueryRow(ctx,
		"SELECT snapshot FROM runs WHERE id = $1 AND tenant_id = $2", id, tenantID,
	).Scan(&data)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var snap dag.Snapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return nil, fmt.Errorf("run storage: unmarshal: %w", err)
	}
	return &snap, nil
}

func (s *RunStorage) List(ctx context.Context, tenantID string) ([]dag.RunSummary, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, snapshot FROM runs WHERE tenant_id = $1 ORDER BY created_at DESC", tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []dag.RunSummary
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var snap dag.Snapshot
		if err := json.Unmarshal([]byte(data), &snap); err != nil {
			continue
		}
		summary := dag.RunSummary{
			ID: id, Total: len(snap.Nodes),
			StartedAt: snap.StartedAt, FinishedAt: snap.FinishedAt,
		}
		if snap.State != nil {
			summary.Provider = snap.State.Provider
			if len(snap.State.RunConfig) > 0 {
				var rc struct {
					Database struct {
						Kind    string `json:"kind"`
						Version string `json:"version"`
					} `json:"database"`
					Stroppy struct {
						Script   string `json:"script"`
						Duration string `json:"duration"`
						VUs      int    `json:"vus"`
					} `json:"stroppy"`
					Machines []struct {
						Role  string `json:"role"`
						Count int    `json:"count"`
					} `json:"machines"`
				}
				if json.Unmarshal(snap.State.RunConfig, &rc) == nil {
					summary.DBKind = rc.Database.Kind
					summary.DBVersion = rc.Database.Version
					summary.Script = rc.Stroppy.Script
					summary.Duration = rc.Stroppy.Duration
					summary.VUs = rc.Stroppy.VUs
					for _, m := range rc.Machines {
						if m.Role == "database" {
							summary.NodeCount += m.Count
						}
					}
				}
			}
		}
		for _, n := range snap.Nodes {
			switch n.Status {
			case dag.StatusDone:
				summary.Done++
			case dag.StatusFailed:
				summary.Failed++
			case dag.StatusCancelled:
				summary.Cancelled = true
				summary.Failed++ // count cancelled in failed for progress bar
			case dag.StatusRunning:
				summary.Pending++ // running = not yet done
			default:
				summary.Pending++
			}
		}
		summary.Nodes = snap.Nodes
		results = append(results, summary)
	}
	return results, nil
}

func (s *RunStorage) Delete(ctx context.Context, tenantID, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM runs WHERE id = $1 AND tenant_id = $2", id, tenantID)
	return err
}

func (s *RunStorage) SetBaseline(ctx context.Context, tenantID, name, runID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO baselines (name, tenant_id, run_id) VALUES ($1, $2, $3)
         ON CONFLICT(name, tenant_id) DO UPDATE SET run_id = excluded.run_id`,
		name, tenantID, runID,
	)
	return err
}

func (s *RunStorage) GetBaseline(ctx context.Context, tenantID, name string) (string, error) {
	var runID string
	err := s.pool.QueryRow(ctx,
		"SELECT run_id FROM baselines WHERE name = $1 AND tenant_id = $2", name, tenantID,
	).Scan(&runID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return runID, err
}

func (s *RunStorage) ListBaselines(ctx context.Context, tenantID string) (map[string]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT name, run_id FROM baselines WHERE tenant_id = $1", tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, runID string
		if err := rows.Scan(&name, &runID); err != nil {
			continue
		}
		result[name] = runID
	}
	return result, nil
}
