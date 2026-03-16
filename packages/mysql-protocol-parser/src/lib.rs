//! MySQL client/server protocol parser for query monitoring.
//!
//! Provides parsing functionality for [MySQL protocol] packets focused on query monitoring
//! without external dependencies.
//!
//! [MySQL protocol]: https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html
//!
//! # Main API
//!
//! - [`command::parse_command`]: Parse MySQL command packets (COM_QUERY, etc.) from payload data
//! - [`resultset::parse_query_response`]: Parse query responses (OK/Error/ResultSet) from raw packet data
//! - [`header::read_packet`]: Read MySQL packet with header
//!
//! # Examples
//!
//! ```rust,ignore
//! // Parse a single packet
//! let (payload, consumed) = header::read_packet(raw_packet_data)?;
//!
//! // For commands (client->server), parse the payload
//! let command = command::parse_command(payload)?;
//!
//! // For responses (server->client), provide raw packet data including headers
//! let response = resultset::parse_query_response(raw_packet_data)?;
//! ```

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
