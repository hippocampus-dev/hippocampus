package routes

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"slack-logger/internal/encryption"
	"slack-logger/internal/types"
)

// ConversationsHistoryRequestInternal extends the standard Slack API request with internal fields
type ConversationsHistoryRequestInternal struct {
	types.ConversationsHistoryRequest
	ChannelName string `json:"channel_name"`
}

func ConversationsHistory(storageService StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var c ConversationsHistoryRequestInternal

		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(types.ConversationsHistoryResponse{
				OK:    false,
				Error: "invalid_json",
			})
			return
		}

		// Validate required parameters
		if c.Channel == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(types.ConversationsHistoryResponse{
				OK:    false,
				Error: "invalid_arguments",
			})
			return
		}

		// Internal validation for channel_name
		if c.ChannelName == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(types.ConversationsHistoryResponse{
				OK:    false,
				Error: "invalid_arguments",
			})
			return
		}

		limit := c.Limit
		if limit <= 0 {
			limit = 100
		} else if limit > 1000 {
			limit = 1000
		}

		offset, err := decodeCursor(c.Cursor)
		if err != nil {
			slog.Warn("invalid cursor, starting from beginning", "cursor", c.Cursor, "error", err)
			offset = 0
		}

		request := &types.GetLogsRequest{
			Channel:   c.Channel,
			Limit:     limit,
			Offset:    offset,
			Inclusive: c.Inclusive,
		}

		if oldestTime, err := parseSlackTimestamp(c.Oldest); err == nil && oldestTime != nil {
			request.Since = oldestTime
		}

		if latestTime, err := parseSlackTimestamp(c.Latest); err == nil && latestTime != nil {
			request.Until = latestTime
		}

		encryptedMessages, err := storageService.GetByChannelName(r.Context(), c.ChannelName, request)
		if err != nil {
			slog.Error("failed to get messages", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(types.ConversationsHistoryResponse{
				OK:    false,
				Error: "channel_not_found",
			})
			return
		}

		decryptedMessages := make([]types.SlackMessage, 0, len(encryptedMessages))
		for _, encMsg := range encryptedMessages {
			decryptedData, err := encryption.Decrypt(c.Channel, encMsg.EncryptedData)
			if err != nil {
				slog.Warn("failed to decrypt message, skipping", "error", err)
				continue
			}

			var message types.SlackMessage
			if err := json.Unmarshal(decryptedData, &message); err != nil {
				slog.Warn("failed to unmarshal decrypted message, skipping", "error", err)
				continue
			}

			decryptedMessages = append(decryptedMessages, message)
		}

		hasMore := false
		nextCursor := ""
		if len(encryptedMessages) == limit {
			hasMore = true
			nextCursor = encodeCursor(offset + limit)
		}

		response := &types.ConversationsHistoryResponse{
			OK:                  true,
			Messages:            decryptedMessages,
			HasMore:             hasMore,
			PinCount:            0,
			ChannelActionsCount: 0,
		}

		if hasMore {
			response.ResponseMetadata = &types.ResponseMetadata{
				NextCursor: nextCursor,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("failed to encode response", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}
