package types

import (
	"encoding/json"
	"time"
)

// ===============================
// Slack API Compatible Types
// ===============================

// SlackMessage represents a Slack message event
// https://api.slack.com/events/message
type SlackMessage struct {
	Type      string `json:"type"`
	Channel   string `json:"channel"`
	User      string `json:"user,omitempty"`
	Text      string `json:"text"`
	Timestamp string `json:"ts"`

	Subtype         string            `json:"subtype,omitempty"`
	Hidden          bool              `json:"hidden,omitempty"`
	Edited          *SlackMessageEdit `json:"edited,omitempty"`
	IsStarred       bool              `json:"is_starred,omitempty"`
	PinnedTo        []string          `json:"pinned_to,omitempty"`
	Reactions       []SlackReaction   `json:"reactions,omitempty"`
	ThreadTimestamp string            `json:"thread_ts,omitempty"`

	Team        string `json:"team,omitempty"`
	ClientMsgID string `json:"client_msg_id,omitempty"`

	BotID      string      `json:"bot_id,omitempty"`
	BotProfile interface{} `json:"bot_profile,omitempty"`

	Attachments []interface{}         `json:"attachments,omitempty"`
	Blocks      []interface{}         `json:"blocks,omitempty"`
	Metadata    *SlackMessageMetadata `json:"metadata,omitempty"`

	DeletedTs string `json:"deleted_ts,omitempty"`

	// Custom field for our application
	ChannelName string `json:"channel_name"`
}

// SlackMessageEdit represents message edit information
// https://api.slack.com/events/message
type SlackMessageEdit struct {
	User      string `json:"user"`
	Timestamp string `json:"ts"`
}

// SlackReaction represents a reaction on a message
// https://api.slack.com/events/message
type SlackReaction struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

// SlackMessageMetadata represents message metadata
// https://api.slack.com/events/message
type SlackMessageMetadata struct {
	EventType    string                 `json:"event_type"`
	EventPayload map[string]interface{} `json:"event_payload"`
}

// SlackMessageChanged represents a message_changed event
// https://api.slack.com/events/message/message_changed
type SlackMessageChanged struct {
	Type      string        `json:"type"`
	Subtype   string        `json:"subtype"`
	Hidden    bool          `json:"hidden"`
	Channel   string        `json:"channel"`
	Timestamp string        `json:"ts"`
	Message   *SlackMessage `json:"message"`

	ChannelName string `json:"channel_name"` // Custom field for our application
}

// SlackMessageDeleted represents a message_deleted event
// https://api.slack.com/events/message/message_deleted
type SlackMessageDeleted struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	Hidden    bool   `json:"hidden"`
	Channel   string `json:"channel"`
	Timestamp string `json:"ts"`
	DeletedTs string `json:"deleted_ts"`

	ChannelName string `json:"channel_name"` // Custom field for our application
}

// ===============================
// Slack Events API Types
// ===============================

// SlackEventWrapper represents the outer event wrapper
// https://api.slack.com/types/event
type SlackEventWrapper struct {
	Type                string               `json:"type"`
	Token               string               `json:"token"`
	TeamID              string               `json:"team_id"`
	APIAppID            string               `json:"api_app_id"`
	Event               json.RawMessage      `json:"event"`
	EventContext        string               `json:"event_context,omitempty"`
	EventID             string               `json:"event_id"`
	EventTime           int64                `json:"event_time"`
	Authorizations      []SlackAuthorization `json:"authorizations,omitempty"`
	IsExtSharedChannel  bool                 `json:"is_ext_shared_channel,omitempty"`
	ContextTeamID       string               `json:"context_team_id,omitempty"`
	ContextEnterpriseID string               `json:"context_enterprise_id,omitempty"`
}

// SlackAuthorization represents event authorization
// https://api.slack.com/types/event
type SlackAuthorization struct {
	EnterpriseID        string `json:"enterprise_id,omitempty"`
	TeamID              string `json:"team_id"`
	UserID              string `json:"user_id"`
	IsBot               bool   `json:"is_bot"`
	IsEnterpriseInstall bool   `json:"is_enterprise_install"`
}

// SlackURLVerification represents URL verification challenge
// https://api.slack.com/events-api#subscriptions
type SlackURLVerification struct {
	Type      string `json:"type"`
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
}

// SlackChallengeResponse represents the challenge response
// https://api.slack.com/events-api#subscriptions
type SlackChallengeResponse struct {
	Challenge string `json:"challenge"`
}

// ===============================
// Slack Web API Request/Response Types
// ===============================

// ConversationsHistoryRequest represents conversations.history API request
// https://api.slack.com/methods/conversations.history
type ConversationsHistoryRequest struct {
	Channel            string `json:"channel"`
	Cursor             string `json:"cursor,omitempty"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty"`
	Inclusive          bool   `json:"inclusive,omitempty"`
	Latest             string `json:"latest,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Oldest             string `json:"oldest,omitempty"`
}

// ConversationsHistoryResponse represents conversations.history API response
// https://api.slack.com/methods/conversations.history
type ConversationsHistoryResponse struct {
	OK                  bool              `json:"ok"`
	Messages            []SlackMessage    `json:"messages,omitempty"`
	HasMore             bool              `json:"has_more"`
	PinCount            int               `json:"pin_count"`
	ChannelActionsTs    string            `json:"channel_actions_ts,omitempty"`
	ChannelActionsCount int               `json:"channel_actions_count"`
	ResponseMetadata    *ResponseMetadata `json:"response_metadata,omitempty"`
	Error               string            `json:"error,omitempty"`
	Warning             string            `json:"warning,omitempty"`
}

// ConversationsRepliesRequest represents conversations.replies API request
// https://api.slack.com/methods/conversations.replies
type ConversationsRepliesRequest struct {
	Channel            string `json:"channel"`
	Ts                 string `json:"ts"`
	Cursor             string `json:"cursor,omitempty"`
	IncludeAllMetadata bool   `json:"include_all_metadata,omitempty"`
	Inclusive          bool   `json:"inclusive,omitempty"`
	Latest             string `json:"latest,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Oldest             string `json:"oldest,omitempty"`
}

// ConversationsRepliesResponse represents conversations.replies API response
// https://api.slack.com/methods/conversations.replies
type ConversationsRepliesResponse struct {
	OK               bool              `json:"ok"`
	Messages         []SlackMessage    `json:"messages,omitempty"`
	HasMore          bool              `json:"has_more"`
	ResponseMetadata *ResponseMetadata `json:"response_metadata,omitempty"`
	Error            string            `json:"error,omitempty"`
	Warning          string            `json:"warning,omitempty"`
}

// ResponseMetadata represents API response metadata
// https://api.slack.com/docs/pagination
type ResponseMetadata struct {
	NextCursor string   `json:"next_cursor"`
	Messages   []string `json:"messages,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// ===============================
// Internal Application Types
// ===============================

// EncryptedMessage represents an encrypted message in storage
type EncryptedMessage struct {
	ChannelName   string    `json:"channel_name"`
	MessageTs     string    `json:"message_ts"`
	ThreadTs      *string   `json:"thread_ts,omitempty"`
	Salt          []byte    `json:"salt"`
	EncryptedData []byte    `json:"encrypted_data"`
	Timestamp     time.Time `json:"timestamp"`
}

// GetLogsRequest represents internal log retrieval request
type GetLogsRequest struct {
	Channel   string     `json:"channel"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Until     *time.Time `json:"until,omitempty"`
	Inclusive bool       `json:"inclusive,omitempty"`
}
