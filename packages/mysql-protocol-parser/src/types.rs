//! MySQL Protocol Data Types and Structures
//!
//! ## Protocol Documentation
//! - Basic Data Types: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_data_types.html>
//! - Integer Types: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_dt_integers.html>
//! - String Types: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_dt_strings.html>
//! - Packet Structure: <https://dev.mysql.com/doc/internals/en/mysql-packet.html>

#[derive(Debug, Clone)]
pub struct PacketHeader {
    pub payload_length: u32,
    pub sequence_id: u8,
}

#[derive(Debug, Clone)]
pub struct OkPacket {
    pub affected_rows: u64,
    pub last_insert_id: u64,
    pub status_flags: u16,
    pub warnings: u16,
    pub info: String,
    pub session_state_changes: Option<Vec<u8>>,
}

#[derive(Debug, Clone)]
pub struct ErrorPacket {
    pub error_code: u16,
    pub sql_state_marker: Option<u8>,
    pub sql_state: Option<String>,
    pub error_message: String,
}

#[derive(Debug, Clone)]
pub struct ColumnDefinition {
    pub catalog: String,
    pub schema: String,
    pub table: String,
    pub org_table: String,
    pub name: String,
    pub org_name: String,
    pub character_set: u16,
    pub column_length: u32,
    pub column_type: u8,
    pub flags: u16,
    pub decimals: u8,
}

#[derive(Debug, Clone)]
pub struct ResultSetRow {
    pub values: Vec<Option<Vec<u8>>>,
}

#[derive(Debug, Clone)]
pub struct ComQuery {
    pub query: String,
}

#[derive(Debug, Clone)]
pub struct ComInitDb {
    pub database: String,
}

#[derive(Debug, Clone)]
pub struct ComStmtPrepare {
    pub query: String,
}

#[derive(Debug, Clone)]
pub struct ComStmtExecute {
    pub statement_id: u32,
    pub flags: u8,
    pub iteration_count: u32,
    pub params: Vec<StmtParameter>,
}

#[derive(Debug, Clone)]
pub struct StmtPrepareOk {
    pub statement_id: u32,
    pub num_columns: u16,
    pub num_params: u16,
    pub warning_count: u16,
}

#[derive(Debug, Clone)]
pub struct StmtParameter {
    pub value: Option<Vec<u8>>,
    pub param_type: Option<u16>,
    pub unsigned_flag: bool,
}

#[derive(Debug, Clone)]
pub struct ParamType {
    pub field_type: u8,
    pub unsigned_flag: bool,
}

#[derive(Debug, Clone)]
pub struct ComStmtSendLongData {
    pub statement_id: u32,
    pub param_id: u16,
    pub data: Vec<u8>,
}

#[derive(Debug, Clone)]
pub struct ComStmtFetch {
    pub statement_id: u32,
    pub num_rows: u32,
}

#[derive(Debug, Clone)]
pub struct ComFieldList {
    pub table: String,
    pub field_wildcard: String,
}

#[derive(Debug, Clone)]
pub struct ComCreateDb {
    pub database: String,
}

#[derive(Debug, Clone)]
pub struct ComDropDb {
    pub database: String,
}

#[derive(Debug, Clone)]
pub struct ComChangeUser {
    pub user: String,
    pub auth_plugin_data: Vec<u8>,
    pub database: String,
    pub character_set: u16,
    pub auth_plugin_name: String,
}

#[derive(Debug, Clone)]
pub struct ComBinlogDump {
    pub binlog_position: u32,
    pub flags: u16,
    pub server_id: u32,
    pub binlog_filename: String,
}

#[derive(Debug, Clone)]
pub struct ComSetOption {
    pub option: u16,
}

#[derive(Debug, Clone)]
pub struct ComBinlogDumpGtid {
    pub flags: u16,
    pub server_id: u32,
    pub binlog_filename_len: u32,
    pub binlog_filename: String,
    pub binlog_position: u64,
    pub data_size: u32,
    pub data: Vec<u8>,
}

#[derive(Debug, Clone)]
pub struct ComClone {
    pub plugin_name: String,
    pub plugin_data: Vec<u8>,
}

#[derive(Debug, Clone)]
pub struct ComSubscribeGroupReplicationStream {
    pub flags: u64,
    pub payload: Vec<u8>,
}

pub fn read_lenenc_int(data: &[u8]) -> Result<(u64, usize), crate::error::Error> {
    if data.is_empty() {
        return Err(crate::error::Error::IncompletePacket(1));
    }

    match data[0] {
        0xFC => {
            if data.len() < 3 {
                return Err(crate::error::Error::IncompletePacket(3 - data.len()));
            }
            let value = u16::from_le_bytes([data[1], data[2]]) as u64;
            Ok((value, 3))
        }
        0xFD => {
            if data.len() < 4 {
                return Err(crate::error::Error::IncompletePacket(4 - data.len()));
            }
            let value = u32::from_le_bytes([data[1], data[2], data[3], 0]) as u64;
            Ok((value, 4))
        }
        0xFE => {
            if data.len() < 9 {
                return Err(crate::error::Error::IncompletePacket(9 - data.len()));
            }
            let value = u64::from_le_bytes([
                data[1], data[2], data[3], data[4], data[5], data[6], data[7], data[8],
            ]);
            Ok((value, 9))
        }
        n => Ok((n as u64, 1)),
    }
}

pub fn read_lenenc_string(data: &[u8]) -> Result<(Vec<u8>, usize), crate::error::Error> {
    let (length, length_size) = read_lenenc_int(data)?;
    let string_start = length_size;
    let string_end = string_start + length as usize;

    if data.len() < string_end {
        return Err(crate::error::Error::IncompletePacket(
            string_end - data.len(),
        ));
    }

    Ok((data[string_start..string_end].to_vec(), string_end))
}

pub fn read_null_terminated_string(data: &[u8]) -> Result<(String, usize), crate::error::Error> {
    let null_pos = data.iter().position(|&b| b == 0).ok_or_else(|| {
        crate::error::Error::InvalidPacket("No null terminator found".to_string())
    })?;

    let string = String::from_utf8(data[..null_pos].to_vec())?;
    Ok((string, null_pos + 1))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_read_lenenc_int_one_byte() {
        let data = vec![0x42];
        let result = read_lenenc_int(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, 0x42);
        assert_eq!(consumed, 1);
    }

    #[test]
    fn test_read_lenenc_int_two_bytes() {
        let data = vec![0xFC, 0x34, 0x12];
        let result = read_lenenc_int(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, 0x1234);
        assert_eq!(consumed, 3);
    }

    #[test]
    fn test_read_lenenc_int_three_bytes() {
        let data = vec![0xFD, 0x78, 0x56, 0x34];
        let result = read_lenenc_int(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, 0x345678);
        assert_eq!(consumed, 4);
    }

    #[test]
    fn test_read_lenenc_int_eight_bytes() {
        let data = vec![0xFE, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01];
        let result = read_lenenc_int(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, 0x0102030405060708);
        assert_eq!(consumed, 9);
    }

    #[test]
    fn test_read_lenenc_int_incomplete_two_bytes() {
        let data = vec![0xFC, 0x34];
        let result = read_lenenc_int(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(1))
        ));
    }

    #[test]
    fn test_read_lenenc_int_incomplete_three_bytes() {
        let data = vec![0xFD, 0x78, 0x56];
        let result = read_lenenc_int(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(1))
        ));
    }

    #[test]
    fn test_read_lenenc_int_incomplete_eight_bytes() {
        let data = vec![0xFE, 0x08, 0x07, 0x06, 0x05];
        let result = read_lenenc_int(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(4))
        ));
    }

    #[test]
    fn test_read_lenenc_int_empty_data() {
        let data = vec![];
        let result = read_lenenc_int(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(1))
        ));
    }

    #[test]
    fn test_read_lenenc_string_simple() {
        let data = vec![0x05, b'H', b'e', b'l', b'l', b'o'];
        let result = read_lenenc_string(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, b"Hello");
        assert_eq!(consumed, 6);
    }

    #[test]
    fn test_read_lenenc_string_empty() {
        let data = vec![0x00];
        let result = read_lenenc_string(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value.len(), 0);
        assert_eq!(consumed, 1);
    }

    #[test]
    fn test_read_lenenc_string_two_byte_length() {
        let mut data = vec![0xFC, 0x05, 0x00];
        data.extend_from_slice(b"Hello");
        let result = read_lenenc_string(&data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, b"Hello");
        assert_eq!(consumed, 8);
    }

    #[test]
    fn test_read_lenenc_string_incomplete_length() {
        let data = vec![0xFC, 0x05];
        let result = read_lenenc_string(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(1))
        ));
    }

    #[test]
    fn test_read_lenenc_string_incomplete_data() {
        let data = vec![0x05, b'H', b'e'];
        let result = read_lenenc_string(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(3))
        ));
    }

    #[test]
    fn test_read_null_terminated_string_simple() {
        let data = b"Hello\0World";
        let result = read_null_terminated_string(data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, "Hello");
        assert_eq!(consumed, 6);
    }

    #[test]
    fn test_read_null_terminated_string_empty() {
        let data = b"\0";
        let result = read_null_terminated_string(data);
        assert!(result.is_ok());
        let (value, consumed) = result.unwrap();
        assert_eq!(value, "");
        assert_eq!(consumed, 1);
    }

    #[test]
    fn test_read_null_terminated_string_no_terminator() {
        let data = b"Hello";
        let result = read_null_terminated_string(data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_read_null_terminated_string_empty_data() {
        let data = b"";
        let result = read_null_terminated_string(data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }
}
