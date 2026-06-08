//! MySQL Protocol Constants
//!
//! ## Protocol Documentation
//! - OK/EOF/ERR Packets: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_response_packets.html>
//! - OK Packet: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_ok_packet.html>
//! - EOF Packet: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_eof_packet.html>
//! - Command Phase: <https://dev.mysql.com/doc/internals/en/command-phase.html>
//! - Capability Flags: <https://dev.mysql.com/doc/internals/en/capability-flags.html>

pub const OK_PACKET: u8 = 0x00;
pub const EOF_PACKET: u8 = 0xFE;
pub const ERR_PACKET: u8 = 0xFF;

pub const COM_SLEEP: u8 = 0x00;
pub const COM_QUIT: u8 = 0x01;
pub const COM_INIT_DB: u8 = 0x02;
pub const COM_QUERY: u8 = 0x03;
pub const COM_FIELD_LIST: u8 = 0x04;
pub const COM_CREATE_DB: u8 = 0x05;
pub const COM_DROP_DB: u8 = 0x06;
pub const COM_STATISTICS: u8 = 0x08;
pub const COM_DEBUG: u8 = 0x0D;
pub const COM_PING: u8 = 0x0E;
pub const COM_CHANGE_USER: u8 = 0x11;
pub const COM_BINLOG_DUMP: u8 = 0x12;
pub const COM_STMT_PREPARE: u8 = 0x16;
pub const COM_STMT_EXECUTE: u8 = 0x17;
pub const COM_STMT_SEND_LONG_DATA: u8 = 0x18;
pub const COM_STMT_CLOSE: u8 = 0x19;
pub const COM_STMT_RESET: u8 = 0x1A;
pub const COM_SET_OPTION: u8 = 0x1B;
pub const COM_STMT_FETCH: u8 = 0x1C;
pub const COM_BINLOG_DUMP_GTID: u8 = 0x1E;
pub const COM_RESET_CONNECTION: u8 = 0x1F;
pub const COM_CLONE: u8 = 0x20;
pub const COM_SUBSCRIBE_GROUP_REPLICATION_STREAM: u8 = 0x21;
pub const COM_END: u8 = 0x22;

pub const MAX_PACKET_SIZE: usize = 0xFFFFFF;

pub const MYSQL_TYPE_DECIMAL: u8 = 0x00;
pub const MYSQL_TYPE_TINY: u8 = 0x01;
pub const MYSQL_TYPE_SHORT: u8 = 0x02;
pub const MYSQL_TYPE_LONG: u8 = 0x03;
pub const MYSQL_TYPE_FLOAT: u8 = 0x04;
pub const MYSQL_TYPE_DOUBLE: u8 = 0x05;
pub const MYSQL_TYPE_NULL: u8 = 0x06;
pub const MYSQL_TYPE_TIMESTAMP: u8 = 0x07;
pub const MYSQL_TYPE_LONGLONG: u8 = 0x08;
pub const MYSQL_TYPE_INT24: u8 = 0x09;
pub const MYSQL_TYPE_DATE: u8 = 0x0A;
pub const MYSQL_TYPE_TIME: u8 = 0x0B;
pub const MYSQL_TYPE_DATETIME: u8 = 0x0C;
pub const MYSQL_TYPE_YEAR: u8 = 0x0D;
pub const MYSQL_TYPE_NEWDATE: u8 = 0x0E;
pub const MYSQL_TYPE_VARCHAR: u8 = 0x0F;
pub const MYSQL_TYPE_BIT: u8 = 0x10;
pub const MYSQL_TYPE_TIMESTAMP2: u8 = 0x11;
pub const MYSQL_TYPE_DATETIME2: u8 = 0x12;
pub const MYSQL_TYPE_TIME2: u8 = 0x13;
pub const MYSQL_TYPE_NEWDECIMAL: u8 = 0xF6;
pub const MYSQL_TYPE_ENUM: u8 = 0xF7;
pub const MYSQL_TYPE_SET: u8 = 0xF8;
pub const MYSQL_TYPE_TINY_BLOB: u8 = 0xF9;
pub const MYSQL_TYPE_MEDIUM_BLOB: u8 = 0xFA;
pub const MYSQL_TYPE_LONG_BLOB: u8 = 0xFB;
pub const MYSQL_TYPE_BLOB: u8 = 0xFC;
pub const MYSQL_TYPE_VAR_STRING: u8 = 0xFD;
pub const MYSQL_TYPE_STRING: u8 = 0xFE;
pub const MYSQL_TYPE_GEOMETRY: u8 = 0xFF;

pub const MYSQL_TYPE_JSON: u8 = 0xF5;

pub const SERVER_SESSION_STATE_CHANGED: u16 = 0x4000;
