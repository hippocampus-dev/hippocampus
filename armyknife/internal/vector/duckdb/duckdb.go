package duckdb

import (
	"armyknife/internal/vector"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb"
	"golang.org/x/xerrors"
)

type Vec struct {
	db        *sql.DB
	dimension int
	mutex     sync.RWMutex
}

func NewVec(path string, dimension int) (*Vec, error) {
	db, err := sql.Open("duckdb", path)
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
	_, err := s.db.Exec(`INSTALL vss`)
	if err != nil {
		return xerrors.Errorf("failed to install vss extension: %w", err)
	}

	_, err = s.db.Exec(`LOAD vss`)
	if err != nil {
		return xerrors.Errorf("failed to load vss extension: %w", err)
	}

	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		metadata TEXT,
		vector_data FLOAT[],
		indexed_at BIGINT,
		source TEXT
	)`)
	if err != nil {
		return xerrors.Errorf("failed to create documents table: %w", err)
	}

	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_documents_source_indexed_at
		ON documents(source, indexed_at)`)
	if err != nil {
		return xerrors.Errorf("failed to create index on documents: %w", err)
	}

	createIndexSQL := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS vec_index
		ON documents USING HNSW (vector_data)
		WITH (metric = 'cosine')`)
	_, err = s.db.Exec(createIndexSQL)
	if err != nil {
		return xerrors.Errorf("failed to create vector index: %w", err)
	}

	return nil
}

func (s *Vec) Index(ctx context.Context, id string, vector []float32, metadata map[string]string, source string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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

	vectorJSON, _ := json.Marshal(vector)

	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO documents (id, metadata, vector_data, indexed_at, source)
		VALUES (?, ?, ?::FLOAT[], ?, ?)`,
		id, string(metadataJSON), string(vectorJSON), indexedAt, source,
	)
	if err != nil {
		return xerrors.Errorf("failed to insert document: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Vec) Search(ctx context.Context, queryVector []float32, limit int) ([]*vector.SearchResult, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	queryJSON, _ := json.Marshal(queryVector)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, metadata, array_cosine_similarity(vector_data, ?::FLOAT[]) as similarity
		FROM documents
		ORDER BY similarity DESC
		LIMIT ?
	`, string(queryJSON), limit)
	if err != nil {
		return nil, xerrors.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []*vector.SearchResult
	for rows.Next() {
		var id, metadataStr string
		var similarity sql.NullFloat64

		if err := rows.Scan(&id, &metadataStr, &similarity); err != nil {
			return nil, xerrors.Errorf("failed to scan row: %w", err)
		}

		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			return nil, xerrors.Errorf("failed to unmarshal metadata: %w", err)
		}

		if !similarity.Valid {
			continue
		}

		results = append(results, &vector.SearchResult{
			ID:         id,
			Metadata:   metadata,
			Distance:   1.0 - similarity.Float64,
			Similarity: similarity.Float64,
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

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Vec) Touch(ctx context.Context, ids []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	indexedAt := time.Now().Unix()
	const batchSize = 500

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		query := "UPDATE documents SET indexed_at = ? WHERE id IN ("
		args := make([]interface{}, len(batch)+1)
		args[0] = indexedAt
		for j, id := range batch {
			if j > 0 {
				query += ","
			}
			query += "?"
			args[j+1] = id
		}
		query += ")"

		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return xerrors.Errorf("failed to batch update timestamps: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Vec) DeleteOld(ctx context.Context, source string, olderThan int64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xerrors.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`DELETE FROM documents WHERE source = ? AND indexed_at < ?`,
		source, olderThan,
	)
	if err != nil {
		return xerrors.Errorf("failed to delete documents: %w", err)
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
