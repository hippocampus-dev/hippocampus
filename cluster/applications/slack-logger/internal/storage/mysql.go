package storage

import (
	"context"
	"database/sql"
	"time"

	"slack-logger/internal/db"
	"slack-logger/internal/routes"
	"slack-logger/internal/types"

	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"golang.org/x/xerrors"
)

type mysqlStorageService struct {
	queries *db.Queries
}

const (
	maxIdleConns    = 10
	connMaxLifetime = 5 * time.Minute
	connMaxIdleTime = 30 * time.Second
)

func NewMySQLService(dsn string) (routes.StorageService, error) {
	database, err := otelsql.Open("mysql", dsn,
		otelsql.WithAttributes(semconv.DBSystemMySQL),
		otelsql.WithSQLCommenter(true),
	)
	if err != nil {
		return nil, xerrors.Errorf("failed to open database: %w", err)
	}

	database.SetMaxIdleConns(maxIdleConns)
	database.SetConnMaxLifetime(connMaxLifetime)
	database.SetConnMaxIdleTime(connMaxIdleTime)

	if err := otelsql.RegisterDBStatsMetrics(
		database,
		otelsql.WithAttributes(semconv.DBSystemMySQL),
	); err != nil {
		return nil, xerrors.Errorf("failed to register DB stats metrics: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		return nil, xerrors.Errorf("failed to ping database: %w", err)
	}

	queries := db.New(database)
	return &mysqlStorageService{
		queries: queries,
	}, nil
}

func buildEncryptedMessage(channelName, messageTs string, threadTs sql.NullString, salt, encryptedData []byte, timestamp time.Time) *types.EncryptedMessage {
	msg := &types.EncryptedMessage{
		ChannelName:   channelName,
		MessageTs:     messageTs,
		Salt:          salt,
		EncryptedData: encryptedData,
		Timestamp:     timestamp,
	}
	if threadTs.Valid {
		msg.ThreadTs = &threadTs.String
	}
	return msg
}

func buildNullString(value *string) sql.NullString {
	if value != nil {
		return sql.NullString{String: *value, Valid: true}
	}
	return sql.NullString{String: "", Valid: false}
}

func (m *mysqlStorageService) Save(ctx context.Context, message *types.EncryptedMessage) error {
	params := db.InsertMessageParams{
		ChannelName:   message.ChannelName,
		MessageTs:     message.MessageTs,
		ThreadTs:      buildNullString(message.ThreadTs),
		Salt:          message.Salt,
		EncryptedData: message.EncryptedData,
		Timestamp:     message.Timestamp,
	}

	err := m.queries.InsertMessage(ctx, params)
	if err != nil {
		return xerrors.Errorf("failed to insert message: %w", err)
	}
	return nil
}

func (m *mysqlStorageService) Update(ctx context.Context, channelName, messageTs string, message *types.EncryptedMessage) error {
	params := db.UpdateMessageParams{
		ThreadTs:      buildNullString(message.ThreadTs),
		Salt:          message.Salt,
		EncryptedData: message.EncryptedData,
		Timestamp:     message.Timestamp,
		ChannelName:   channelName,
		MessageTs:     messageTs,
	}

	err := m.queries.UpdateMessage(ctx, params)
	if err != nil {
		return xerrors.Errorf("failed to update message: %w", err)
	}
	return nil
}

func (m *mysqlStorageService) Delete(ctx context.Context, channelName, messageTs string) error {
	err := m.queries.DeleteMessage(ctx, db.DeleteMessageParams{
		ChannelName: channelName,
		MessageTs:   messageTs,
	})
	if err != nil {
		return xerrors.Errorf("failed to delete message: %w", err)
	}
	return nil
}

func (m *mysqlStorageService) GetByChannelName(ctx context.Context, channelName string, request *types.GetLogsRequest) ([]*types.EncryptedMessage, error) {
	limit := int32(100)
	if request.Limit > 0 {
		limit = int32(request.Limit)
	}

	params := db.GetMessagesByChannelNameParams{
		ChannelName: channelName,
		Limit:       limit,
		Offset:      int32(request.Offset),
	}

	if request.Since != nil {
		params.SinceTime = sql.NullTime{Time: *request.Since, Valid: true}
		params.SinceTime_2 = params.SinceTime
		params.SinceTime_3 = params.SinceTime
	}

	if request.Until != nil {
		params.UntilTime = sql.NullTime{Time: *request.Until, Valid: true}
		params.UntilTime_2 = params.UntilTime
		params.UntilTime_3 = params.UntilTime
	}

	if request.Inclusive {
		params.Inclusive = sql.NullInt64{Int64: 1, Valid: true}
		params.Inclusive_2 = params.Inclusive
	} else {
		params.Inclusive = sql.NullInt64{Int64: 0, Valid: true}
		params.Inclusive_2 = params.Inclusive
	}

	rows, err := m.queries.GetMessagesByChannelName(ctx, params)
	if err != nil {
		return nil, xerrors.Errorf("failed to get messages: %w", err)
	}

	messages := make([]*types.EncryptedMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, buildEncryptedMessage(row.ChannelName, row.MessageTs, row.ThreadTs, row.Salt, row.EncryptedData, row.Timestamp))
	}

	return messages, nil
}

func (m *mysqlStorageService) GetByThreadTs(ctx context.Context, channelName string, threadTs string, request *types.GetLogsRequest) ([]*types.EncryptedMessage, error) {
	limit := int32(100)
	if request.Limit > 0 {
		limit = int32(request.Limit)
	}

	params := db.GetMessagesByThreadTsParams{
		ChannelName: channelName,
		ThreadTs:    sql.NullString{String: threadTs, Valid: threadTs != ""},
		MessageTs:   threadTs,
		Limit:       limit,
		Offset:      int32(request.Offset),
	}

	if request.Since != nil {
		params.SinceTime = sql.NullTime{Time: *request.Since, Valid: true}
		params.SinceTime_2 = params.SinceTime
		params.SinceTime_3 = params.SinceTime
	}

	if request.Until != nil {
		params.UntilTime = sql.NullTime{Time: *request.Until, Valid: true}
		params.UntilTime_2 = params.UntilTime
		params.UntilTime_3 = params.UntilTime
	}

	if request.Inclusive {
		params.Inclusive = sql.NullInt64{Int64: 1, Valid: true}
		params.Inclusive_2 = params.Inclusive
	} else {
		params.Inclusive = sql.NullInt64{Int64: 0, Valid: true}
		params.Inclusive_2 = params.Inclusive
	}

	rows, err := m.queries.GetMessagesByThreadTs(ctx, params)
	if err != nil {
		return nil, xerrors.Errorf("failed to get messages by thread ts: %w", err)
	}

	messages := make([]*types.EncryptedMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, buildEncryptedMessage(row.ChannelName, row.MessageTs, row.ThreadTs, row.Salt, row.EncryptedData, row.Timestamp))
	}

	return messages, nil
}
