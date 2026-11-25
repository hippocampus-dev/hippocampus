package sqlite

import (
	"armyknife/internal/vector"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	_ "github.com/ncruces/go-sqlite3/driver"
	"golang.org/x/xerrors"
)

var identifierPattern = regexp.MustCompile(`[\p{L}\p{N}]+`)

type Vec struct {
	db        *sql.DB
	dimension int
	mutex     sync.RWMutex
}

func NewVec(path string, dimension int) (*Vec, error) {
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

	_, err = s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS fts_index USING fts5(
		id UNINDEXED,
		content
	)`)
	if err != nil {
		return xerrors.Errorf("failed to create full-text index: %w", err)
	}

	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return xerrors.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id, metadata FROM documents WHERE id NOT IN (SELECT id FROM fts_index)`)
	if err != nil {
		return xerrors.Errorf("failed to query documents missing from full-text index: %w", err)
	}
	defer rows.Close()

	missing := make(map[string]map[string]string)
	for rows.Next() {
		var id, metadataStr string
		if err := rows.Scan(&id, &metadataStr); err != nil {
			return xerrors.Errorf("failed to scan row: %w", err)
		}

		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			return xerrors.Errorf("failed to unmarshal metadata: %w", err)
		}
		missing[id] = metadata
	}
	if err := rows.Err(); err != nil {
		return xerrors.Errorf("error iterating rows: %w", err)
	}

	for id, metadata := range missing {
		if _, err := tx.Exec(`INSERT INTO fts_index (id, content) VALUES (?, ?)`, id, fullTextContent(metadata)); err != nil {
			return xerrors.Errorf("failed to insert into full-text index: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return xerrors.Errorf("failed to commit transaction: %w", err)
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

	_, _ = tx.ExecContext(ctx, `DELETE FROM fts_index WHERE id = ?`, id)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO fts_index (id, content) VALUES (?, ?)`,
		id, fullTextContent(metadata),
	)
	if err != nil {
		return xerrors.Errorf("failed to insert into full-text index: %w", err)
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

func (s *Vec) HybridSearch(ctx context.Context, queryVector []float32, queryText string, limit int) ([]*vector.SearchResult, error) {
	var terms []string
	seen := make(map[string]bool)
	for _, token := range identifierPattern.FindAllString(expandIdentifiers(queryText), -1) {
		token = strings.ToLower(token)
		if seen[token] {
			continue
		}
		seen[token] = true
		terms = append(terms, fmt.Sprintf(`"%s"`, token))
	}
	if len(terms) == 0 {
		return s.Search(ctx, queryVector, limit)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	queryBlob, err := vec.SerializeFloat32(queryVector)
	if err != nil {
		return nil, xerrors.Errorf("failed to serialize query vector: %w", err)
	}

	// The constant 60 is the k of Reciprocal Rank Fusion: https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf
	rows, err := s.db.QueryContext(ctx, `
		WITH vector_hits AS MATERIALIZED (
			SELECT id, row_number() OVER (ORDER BY distance, id) AS rank
			FROM (SELECT id, vec_distance_cosine(embedding, ?1) AS distance FROM vec_index ORDER BY distance LIMIT ?3)
		), text_hits AS MATERIALIZED (
			SELECT id, row_number() OVER (ORDER BY score, id) AS rank
			FROM (SELECT id, bm25(fts_index) AS score FROM fts_index WHERE fts_index MATCH ?2 ORDER BY score LIMIT ?3)
		), fused AS (
			SELECT id, SUM(1.0 / (60 + rank)) AS score
			FROM (SELECT id, rank FROM vector_hits UNION ALL SELECT id, rank FROM text_hits)
			GROUP BY id
		)
		SELECT d.id, d.metadata, vec_distance_cosine(v.embedding, ?1) AS distance
		FROM fused f
		JOIN documents d ON d.id = f.id
		JOIN vec_index v ON v.id = f.id
		ORDER BY f.score DESC, d.id
		LIMIT ?4
	`, queryBlob, strings.Join(terms, " OR "), limit*5, limit)
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

	_, err = tx.ExecContext(ctx, `DELETE FROM fts_index WHERE id = ?`, id)
	if err != nil {
		return xerrors.Errorf("failed to delete from full-text index: %w", err)
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
		`DELETE FROM vec_index WHERE id IN (SELECT id FROM documents WHERE source = ? AND indexed_at < ?)`,
		source, olderThan,
	)
	if err != nil {
		return xerrors.Errorf("failed to delete from vector index: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM fts_index WHERE id IN (SELECT id FROM documents WHERE source = ? AND indexed_at < ?)`,
		source, olderThan,
	)
	if err != nil {
		return xerrors.Errorf("failed to delete from full-text index: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM documents WHERE source = ? AND indexed_at < ?`,
		source, olderThan,
	)
	if err != nil {
		return xerrors.Errorf("failed to delete from documents: %w", err)
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

func fullTextContent(metadata map[string]string) string {
	values := make([]string, 0, len(metadata))
	for _, value := range metadata {
		if _, err := strconv.Atoi(value); err == nil {
			continue
		}
		values = append(values, value)
	}
	return expandIdentifiers(strings.Join(values, " "))
}

func expandIdentifiers(text string) string {
	var builder strings.Builder
	builder.WriteString(text)
	for _, identifier := range identifierPattern.FindAllString(text, -1) {
		runes := []rune(identifier)
		var parts []string
		start := 0
		for i := 1; i < len(runes); i++ {
			if unicode.IsUpper(runes[i]) && (!unicode.IsUpper(runes[i-1]) || i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				parts = append(parts, string(runes[start:i]))
				start = i
			}
		}
		if start == 0 {
			continue
		}
		parts = append(parts, string(runes[start:]))
		builder.WriteString(" ")
		builder.WriteString(strings.Join(parts, " "))
	}
	return builder.String()
}
