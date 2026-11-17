package routes

import (
	"context"

	"slack-logger/internal/types"
)

type StorageService interface {
	Save(ctx context.Context, message *types.EncryptedMessage) error
	Update(ctx context.Context, channelName, messageTs string, message *types.EncryptedMessage) error
	Delete(ctx context.Context, channelName, messageTs string) error
	GetByChannelName(ctx context.Context, channelName string, request *types.GetLogsRequest) ([]*types.EncryptedMessage, error)
	GetByThreadTs(ctx context.Context, channelName string, threadTs string, request *types.GetLogsRequest) ([]*types.EncryptedMessage, error)
}
