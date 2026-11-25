schema "slack_logger" {
  charset = "utf8mb4"
  collate = "utf8mb4_0900_ai_ci"
}

table "encrypted_messages" {
  schema = schema.slack_logger

  column "id" {
    type = int
    unsigned = true
    auto_increment = true
  }

  column "channel_name" {
    type = varchar(255)
    null = false
  }

  column "message_ts" {
    type = varchar(255)
    null = false
  }

  column "thread_ts" {
    type = varchar(255)
    null = true
  }

  column "salt" {
    type = varbinary(32)
    null = false
  }

  column "encrypted_data" {
    type = longblob
    null = false
  }

  column "timestamp" {
    type = timestamp
    null = false
  }

  column "created_at" {
    type = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_channel_timestamp" {
    columns = [column.channel_name, column.timestamp]
    type = BTREE
    comment = "For GetMessagesByChannelName query"
  }

  index "idx_channel_message_ts" {
    columns = [column.channel_name, column.message_ts]
    type = BTREE
    unique = true
    comment = "For UpdateMessage and DeleteMessage operations"
  }

  index "idx_channel_thread" {
    columns = [column.channel_name, column.thread_ts]
    type = BTREE
    comment = "For GetMessagesByThreadTs query"
  }
}
