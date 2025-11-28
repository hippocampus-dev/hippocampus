//! MySQL Client/Server Protocol Parser for Query Monitoring
//!
//! Provides parsing functionality for MySQL protocol packets focused on query monitoring without external dependencies.
//!
//! ## Main API
//! - `command::parse_command` - Parse MySQL command packets (COM_QUERY, etc.) from payload data
//! - `resultset::parse_query_response` - Parse query responses (OK/Error/ResultSet) from raw packet data
//! - `header::read_packet` - Read MySQL packet with header
//!
//! ## Usage Example
//! ```rust,ignore
//! // Parse a single packet
//! let (payload, consumed) = header::read_packet(raw_packet_data)?;
//!
//! // For commands (client->server), parse the payload
//! let command = command::parse_command(payload)?;
//!
//! // For responses (server->client), provide raw packet data including headers
//! // Note: For responses spanning multiple packets, concatenate all raw packet data
//! let response = resultset::parse_query_response(raw_packet_data)?;
//!
//! // Application layer is responsible for:
//! // - Managing sequence IDs
//! // - Assembling multi-packet messages (when payload_length == 0xFFFFFF)
//! ```
//!
//! ## Protocol Documentation
//! - Main Protocol Overview: <https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html>
//! - Command Phase: <https://dev.mysql.com/doc/internals/en/command-phase.html>
//! - Basic Data Types: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_data_types.html>

pub mod command;
pub mod constants;
pub mod error;
pub mod header;
pub mod resultset;
pub mod types;

// Internal modules - not exposed in public API
mod binary_resultset;
mod error_packet;
mod ok_packet;

// Re-export main types for convenience
pub use command::{Command, CommandContext};
pub use resultset::{QueryResponse, ResultSet};
