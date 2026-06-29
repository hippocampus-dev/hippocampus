package routes

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"slack-logger/internal/encryption"
	"slack-logger/internal/types"

	"golang.org/x/xerrors"
)

func HandleEvents(storageService StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			slog.Error("failed to decode request body", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(body, &typeCheck); err != nil {
			slog.Error("failed to unmarshal type", "error", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		switch typeCheck.Type {
		case "url_verification":
			handleURLVerification(w, body)
		case "event_callback":
			handleEventCallback(w, body, storageService)
		default:
			slog.Warn("unsupported event type", "type", typeCheck.Type)
			http.Error(w, "unsupported event type", http.StatusBadRequest)
		}
	}
}

func handleURLVerification(w http.ResponseWriter, body json.RawMessage) {
	var verification types.SlackURLVerification
	if err := json.Unmarshal(body, &verification); err != nil {
		slog.Error("failed to unmarshal URL verification", "error", err)
		http.Error(w, "invalid verification request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(types.SlackChallengeResponse{
		Challenge: verification.Challenge,
	}); err != nil {
		slog.Error("failed to encode challenge response", "error", err)
	}
}

func handleEventCallback(w http.ResponseWriter, body json.RawMessage, storageService StorageService) {
	var wrapper types.SlackEventWrapper
	if err := json.Unmarshal(body, &wrapper); err != nil {
		slog.Error("failed to unmarshal event wrapper", "error", err)
		http.Error(w, "invalid event format", http.StatusBadRequest)
		return
	}

	var eventType struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
	}
	if err := json.Unmarshal(wrapper.Event, &eventType); err != nil {
		slog.Error("failed to unmarshal event type", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if eventType.Type != "message" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch eventType.Subtype {
	case "message_changed":
		handleMessageChanged(w, wrapper.Event, storageService)
	case "message_deleted":
		handleMessageDeleted(w, wrapper.Event, storageService)
	case "", "me_message", "thread_broadcast", "bot_message":
		handleRegularMessage(w, wrapper.Event, storageService)
	case "channel_join", "channel_leave":
		slog.Debug("ignoring channel membership events", "subtype", eventType.Subtype)
		w.WriteHeader(http.StatusOK)
	default:
		slog.Info("unsupported message subtype", "subtype", eventType.Subtype)
		w.WriteHeader(http.StatusOK)
	}
}

func handleRegularMessage(w http.ResponseWriter, event json.RawMessage, storageService StorageService) {
	var message types.SlackMessage
	if err := json.Unmarshal(event, &message); err != nil {
		slog.Error("failed to unmarshal message event", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if message.Channel == "" || message.ChannelName == "" {
		slog.Warn("missing required fields", "channel", message.Channel, "channelName", message.ChannelName)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := encryptAndStoreMessage(&message, storageService, "save"); err != nil {
		slog.Error("failed to save message", "error", err)
	}
	w.WriteHeader(http.StatusOK)
}

func handleMessageChanged(w http.ResponseWriter, event json.RawMessage, storageService StorageService) {
	var messageChanged types.SlackMessageChanged
	if err := json.Unmarshal(event, &messageChanged); err != nil {
		slog.Error("failed to unmarshal message_changed event", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if messageChanged.Message == nil {
		slog.Warn("message_changed event missing message field")
		w.WriteHeader(http.StatusOK)
		return
	}

	if messageChanged.Message.Channel == "" {
		messageChanged.Message.Channel = messageChanged.Channel
	}
	if messageChanged.Message.ChannelName == "" {
		messageChanged.Message.ChannelName = messageChanged.ChannelName
	}

	if messageChanged.Message.Channel == "" || messageChanged.Message.ChannelName == "" || messageChanged.Message.Timestamp == "" {
		slog.Warn("missing required fields in message_changed", "channel", messageChanged.Message.Channel, "channelName", messageChanged.Message.ChannelName, "ts", messageChanged.Message.Timestamp)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := encryptAndStoreMessage(messageChanged.Message, storageService, "update"); err != nil {
		slog.Error("failed to update message", "error", err)
	}
	w.WriteHeader(http.StatusOK)
}

func handleMessageDeleted(w http.ResponseWriter, event json.RawMessage, storageService StorageService) {
	var messageDeleted types.SlackMessageDeleted
	if err := json.Unmarshal(event, &messageDeleted); err != nil {
		slog.Error("failed to unmarshal message_deleted event", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if messageDeleted.ChannelName == "" || messageDeleted.DeletedTs == "" {
		slog.Warn("missing required fields in message_deleted", "channelName", messageDeleted.ChannelName, "deletedTs", messageDeleted.DeletedTs)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := storageService.Delete(context.Background(), messageDeleted.ChannelName, messageDeleted.DeletedTs); err != nil {
		slog.Error("failed to delete message", "error", err)
	}

	w.WriteHeader(http.StatusOK)
}

func encryptAndStoreMessage(message *types.SlackMessage, storageService StorageService, operation string) error {
	data, err := json.Marshal(message)
	if err != nil {
		return xerrors.Errorf("failed to marshal message: %w", err)
	}

	encryptedData, err := encryption.Encrypt(message.Channel, data)
	if err != nil {
		return xerrors.Errorf("failed to encrypt message: %w", err)
	}

	salt := encryptedData[:encryption.SaltSize]
	encryptedMessage := &types.EncryptedMessage{
		ChannelName:   message.ChannelName,
		MessageTs:     message.Timestamp,
		Salt:          salt,
		EncryptedData: encryptedData,
		Timestamp:     time.Now(),
	}

	if message.ThreadTimestamp != "" {
		encryptedMessage.ThreadTs = &message.ThreadTimestamp
	}

	if operation == "update" {
		return storageService.Update(context.Background(), message.ChannelName, message.Timestamp, encryptedMessage)
	}
	return storageService.Save(context.Background(), encryptedMessage)
}
