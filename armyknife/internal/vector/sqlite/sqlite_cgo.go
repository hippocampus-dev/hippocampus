//go:build cgo
// +build cgo

package sqlite

import (
	"armyknife/internal/vector"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/xerrors"
)

type Vec struct {
	db        *sql.DB
	dimension int
	mutex     sync.RWMutex
}

func NewVec(path string, dimension int) (*Vec, error) {
	vec.Auto()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, xerrors.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, xerrors.Errorf("failed to ping database: %w", err)
	}

	store := &Vec{
		db:        db,
		dimension: dimension,
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, xerrors.Errorf("failed to initialize: %w", err)
	}

	return store, nil
}

func (s *Vec) initialize() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		metadata TEXT,
		vector_data BLOB,
		indexed_at INTEGER,
		source TEXT
	)`)
	if err != nil {
		return xerrors.Errorf("failed to create documents table: %w", err)
	}

	createIndexSQL := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS vec_index USING vec0(
		id TEXT PRIMARY KEY,
		embedding float[%d]
	)`, s.dimension)
	_, err = s.db.Exec(createIndexSQL)
	if err != nil {
		return xerrors.Errorf("failed to create vector index: %w", err)
	}

	return nil
}

func (s *Vec) Index(ctx context.Context, id string, vector []float32, metadata map[string]string, source string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	vectorBlob, err := vec.SerializeFloat32(vector)
	if err != nil {
		return xerrors.Errorf("failed to serialize vector: %w", err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return xerrors.Errorf("failed to marshal metadata: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	indexedAt := time.Now().Unix()
	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO documents (id, metadata, vector_data, indexed_at, source) VALUES (?, ?, ?, ?, ?)`,
		id, string(metadataJSON), vectorBlob, indexedAt, source,
	)
	if err != nil {
		return xerrors.Errorf("failed to insert document: %w", err)
	}

	_, _ = tx.ExecContext(ctx, `DELETE FROM vec_index WHERE id = ?`, id)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO vec_index (id, embedding) VALUES (?, ?)`,
		id, vectorBlob,
	)
	if err != nil {
		return xerrors.Errorf("failed to insert into vector index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Vec) Search(ctx context.Context, queryVector []float32, limit int) ([]*vector.SearchResult, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	queryBlob, err := vec.SerializeFloat32(queryVector)
	if err != nil {
		return nil, xerrors.Errorf("failed to serialize query vector: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.metadata, vec_distance_cosine(v.embedding, ?) as distance
		FROM vec_index v
		JOIN documents d ON v.id = d.id
		ORDER BY distance
		LIMIT ?
	`, queryBlob, limit)
	if err != nil {
		return nil, xerrors.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []*vector.SearchResult
	for rows.Next() {
		var id, metadataStr string
		var distance sql.NullFloat64

		if err := rows.Scan(&id, &metadataStr, &distance); err != nil {
			return nil, xerrors.Errorf("failed to scan row: %w", err)
		}

		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			return nil, xerrors.Errorf("failed to unmarshal metadata: %w", err)
		}

		if !distance.Valid {
			continue
		}

		results = append(results, &vector.SearchResult{
			ID:         id,
			Metadata:   metadata,
			Distance:   distance.Float64,
			Similarity: 1.0 - distance.Float64,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, xerrors.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (s *Vec) Exists(ctx context.Context, id string) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM documents WHERE id = ?)`,
		id,
	).Scan(&exists)

	if err != nil {
		return false, xerrors.Errorf("failed to check document existence: %w", err)
	}

	return exists, nil
}

func (s *Vec) Delete(ctx context.Context, id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err != nil {
		return xerrors.Errorf("failed to delete from documents: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM vec_index WHERE id = ?`, id)
	if err != nil {
		return xerrors.Errorf("failed to delete from vector index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Vec) DeleteOldEntries(ctx context.Context, source string, beforeTimestamp int64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM documents WHERE source = ? AND indexed_at < ?`,
		source, beforeTimestamp,
	)
	if err != nil {
		return xerrors.Errorf("failed to query old documents: %w", err)
	}
	defer rows.Close()

	var idsToDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return xerrors.Errorf("failed to scan document id: %w", err)
		}
		idsToDelete = append(idsToDelete, id)
	}

	if err := rows.Err(); err != nil {
		return xerrors.Errorf("error iterating rows: %w", err)
	}

	if len(idsToDelete) == 0 {
		return nil
	}

	for _, id := range idsToDelete {
		_, err = tx.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
		if err != nil {
			return xerrors.Errorf("failed to delete from documents: %w", err)
		}

		_, err = tx.ExecContext(ctx, `DELETE FROM vec_index WHERE id = ?`, id)
		if err != nil {
			return xerrors.Errorf("failed to delete from vector index: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Vec) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
