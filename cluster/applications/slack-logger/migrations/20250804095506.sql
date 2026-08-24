-- Create "encrypted_messages" table
CREATE TABLE `encrypted_messages` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `channel_name` varchar(255) NOT NULL,
  `message_ts` varchar(255) NOT NULL,
  `thread_ts` varchar(255) NULL,
  `salt` varbinary(32) NOT NULL,
  `encrypted_data` longblob NOT NULL,
  `timestamp` timestamp NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_channel_message_ts` (`channel_name`, `message_ts`) COMMENT "For UpdateMessage and DeleteMessage operations",
  INDEX `idx_channel_thread` (`channel_name`, `thread_ts`) COMMENT "For GetMessagesByThreadTs query",
  INDEX `idx_channel_timestamp` (`channel_name`, `timestamp`) COMMENT "For GetMessagesByChannelName query"
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
