//! MySQL Protocol Command Parsing
//!
//! ## Protocol Documentation
//! - Command Phase: <https://dev.mysql.com/doc/internals/en/command-phase.html>
//! - COM_QUERY: <https://dev.mysql.com/doc/internals/en/com-query.html>
//! - Prepared Statements: <https://dev.mysql.com/doc/internals/en/prepared-statements.html>
//! - Text Protocol: <https://dev.mysql.com/doc/internals/en/text-protocol.html>

pub fn parse_command(data: &[u8]) -> Result<Command, crate::error::Error> {
    parse_command_with_context(data, None)
}

pub fn parse_command_with_context(
    data: &[u8],
    context: Option<&CommandContext>,
) -> Result<Command, crate::error::Error> {
    if data.is_empty() {
        return Err(crate::error::Error::IncompletePacket(1));
    }

    let command_type = data[0];
    let payload = &data[1..];

    match command_type {
        crate::constants::COM_SLEEP => Ok(Command::Sleep),
        crate::constants::COM_QUIT => Ok(Command::Quit),
        crate::constants::COM_INIT_DB => {
            let database = String::from_utf8(payload.to_vec())?;
            Ok(Command::InitDb(crate::types::ComInitDb { database }))
        }
        crate::constants::COM_QUERY => {
            let query = String::from_utf8(payload.to_vec())?;
            Ok(Command::Query(crate::types::ComQuery { query }))
        }
        crate::constants::COM_FIELD_LIST => {
            let (table, table_len) = crate::types::read_null_terminated_string(payload)?;
            let field_wildcard = String::from_utf8(payload[table_len..].to_vec())?;

            Ok(Command::FieldList(crate::types::ComFieldList {
                table,
                field_wildcard,
            }))
        }
        crate::constants::COM_CREATE_DB => {
            let database = String::from_utf8(payload.to_vec())?;
            Ok(Command::CreateDb(crate::types::ComCreateDb { database }))
        }
        crate::constants::COM_DROP_DB => {
            let database = String::from_utf8(payload.to_vec())?;
            Ok(Command::DropDb(crate::types::ComDropDb { database }))
        }
        crate::constants::COM_STATISTICS => Ok(Command::Statistics),
        crate::constants::COM_DEBUG => Ok(Command::Debug),
        crate::constants::COM_PING => Ok(Command::Ping),
        crate::constants::COM_CHANGE_USER => {
            let mut offset = 0;

            let (user, user_len) = crate::types::read_null_terminated_string(&payload[offset..])?;
            offset += user_len;

            if payload.len() < offset + 1 {
                return Err(crate::error::Error::IncompletePacket(1));
            }
            let auth_response_len = payload[offset] as usize;
            offset += 1;

            if payload.len() < offset + auth_response_len {
                return Err(crate::error::Error::IncompletePacket(
                    offset + auth_response_len - payload.len(),
                ));
            }
            let auth_plugin_data = payload[offset..offset + auth_response_len].to_vec();
            offset += auth_response_len;

            let (database, db_len) = crate::types::read_null_terminated_string(&payload[offset..])?;
            offset += db_len;

            if payload.len() < offset + 2 {
                return Err(crate::error::Error::IncompletePacket(
                    offset + 2 - payload.len(),
                ));
            }
            let character_set = u16::from_le_bytes([payload[offset], payload[offset + 1]]);
            offset += 2;

            let (auth_plugin_name, _) =
                crate::types::read_null_terminated_string(&payload[offset..])?;

            Ok(Command::ChangeUser(crate::types::ComChangeUser {
                user,
                auth_plugin_data,
                database,
                character_set,
                auth_plugin_name,
            }))
        }
        crate::constants::COM_BINLOG_DUMP => {
            if payload.len() < 10 {
                return Err(crate::error::Error::IncompletePacket(10 - payload.len()));
            }

            let binlog_position =
                u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
            let flags = u16::from_le_bytes([payload[4], payload[5]]);
            let server_id = u32::from_le_bytes([payload[6], payload[7], payload[8], payload[9]]);
            let binlog_filename = String::from_utf8(payload[10..].to_vec())?;

            Ok(Command::BinlogDump(crate::types::ComBinlogDump {
                binlog_position,
                flags,
                server_id,
                binlog_filename,
            }))
        }
        crate::constants::COM_STMT_PREPARE => {
            let query = String::from_utf8(payload.to_vec())?;
            Ok(Command::StmtPrepare(crate::types::ComStmtPrepare { query }))
        }
        crate::constants::COM_STMT_EXECUTE => {
            let mut offset = 0;

            if payload.len() < 4 {
                return Err(crate::error::Error::IncompletePacket(4));
            }
            let statement_id = u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
            offset += 4;

            if payload.len() < offset + 1 {
                return Err(crate::error::Error::IncompletePacket(
                    offset + 1 - payload.len(),
                ));
            }
            let flags = payload[offset];
            offset += 1;

            if payload.len() < offset + 4 {
                return Err(crate::error::Error::IncompletePacket(
                    offset + 4 - payload.len(),
                ));
            }
            let iteration_count = u32::from_le_bytes([
                payload[offset],
                payload[offset + 1],
                payload[offset + 2],
                payload[offset + 3],
            ]);
            offset += 4;

            if offset >= payload.len() {
                return Ok(Command::StmtExecute(crate::types::ComStmtExecute {
                    statement_id,
                    flags,
                    iteration_count,
                    params: Vec::new(),
                }));
            }

            let num_params = context
                .and_then(|context| context.statement_params.get(&statement_id))
                .copied()
                .unwrap_or(0);

            let params = if num_params > 0 {
                parse_stmt_execute_params(&payload[offset..], num_params)?
            } else {
                parse_stmt_execute_params_simple(&payload[offset..])?
            };

            Ok(Command::StmtExecute(crate::types::ComStmtExecute {
                statement_id,
                flags,
                iteration_count,
                params,
            }))
        }
        crate::constants::COM_STMT_SEND_LONG_DATA => {
            if payload.len() < 6 {
                return Err(crate::error::Error::IncompletePacket(6 - payload.len()));
            }

            let statement_id = u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
            let param_id = u16::from_le_bytes([payload[4], payload[5]]);
            let data_value = payload[6..].to_vec();

            Ok(Command::StmtSendLongData(
                crate::types::ComStmtSendLongData {
                    statement_id,
                    param_id,
                    data: data_value,
                },
            ))
        }
        crate::constants::COM_STMT_CLOSE => {
            if payload.len() < 4 {
                return Err(crate::error::Error::IncompletePacket(4 - payload.len()));
            }
            let statement_id = u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
            Ok(Command::StmtClose(statement_id))
        }
        crate::constants::COM_STMT_RESET => {
            if payload.len() < 4 {
                return Err(crate::error::Error::IncompletePacket(4 - payload.len()));
            }
            let statement_id = u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
            Ok(Command::StmtReset(statement_id))
        }
        crate::constants::COM_SET_OPTION => {
            if payload.len() < 2 {
                return Err(crate::error::Error::IncompletePacket(2 - payload.len()));
            }

            let option = u16::from_le_bytes([payload[0], payload[1]]);

            Ok(Command::SetOption(crate::types::ComSetOption { option }))
        }
        crate::constants::COM_STMT_FETCH => {
            if payload.len() < 8 {
                return Err(crate::error::Error::IncompletePacket(8 - payload.len()));
            }

            let statement_id = u32::from_le_bytes([payload[0], payload[1], payload[2], payload[3]]);
            let num_rows = u32::from_le_bytes([payload[4], payload[5], payload[6], payload[7]]);

            Ok(Command::StmtFetch(crate::types::ComStmtFetch {
                statement_id,
                num_rows,
            }))
        }
        crate::constants::COM_BINLOG_DUMP_GTID => {
            let mut offset = 0;

            if payload.len() < 10 {
                return Err(crate::error::Error::IncompletePacket(10 - payload.len()));
            }

            let flags = u16::from_le_bytes([payload[0], payload[1]]);
            offset += 2;

            let server_id = u32::from_le_bytes([
                payload[offset],
                payload[offset + 1],
                payload[offset + 2],
                payload[offset + 3],
            ]);
            offset += 4;

            let binlog_filename_len = u32::from_le_bytes([
                payload[offset],
                payload[offset + 1],
                payload[offset + 2],
                payload[offset + 3],
            ]);
            offset += 4;

            if payload.len() < offset + binlog_filename_len as usize {
                return Err(crate::error::Error::IncompletePacket(
                    offset + binlog_filename_len as usize - payload.len(),
                ));
            }
            let binlog_filename =
                String::from_utf8(payload[offset..offset + binlog_filename_len as usize].to_vec())?;
            offset += binlog_filename_len as usize;

            if payload.len() < offset + 8 {
                return Err(crate::error::Error::IncompletePacket(
                    offset + 8 - payload.len(),
                ));
            }
            let binlog_position = u64::from_le_bytes([
                payload[offset],
                payload[offset + 1],
                payload[offset + 2],
                payload[offset + 3],
                payload[offset + 4],
                payload[offset + 5],
                payload[offset + 6],
                payload[offset + 7],
            ]);
            offset += 8;

            if payload.len() < offset + 4 {
                return Err(crate::error::Error::IncompletePacket(
                    offset + 4 - payload.len(),
                ));
            }
            let data_size = u32::from_le_bytes([
                payload[offset],
                payload[offset + 1],
                payload[offset + 2],
                payload[offset + 3],
            ]);
            offset += 4;

            let gtid_data = payload[offset..].to_vec();

            Ok(Command::BinlogDumpGtid(crate::types::ComBinlogDumpGtid {
                flags,
                server_id,
                binlog_filename_len,
                binlog_filename,
                binlog_position,
                data_size,
                data: gtid_data,
            }))
        }
        crate::constants::COM_RESET_CONNECTION => Ok(Command::ResetConnection),
        crate::constants::COM_CLONE => {
            let (plugin_name, name_len) = crate::types::read_null_terminated_string(payload)?;
            let plugin_data = payload[name_len..].to_vec();

            Ok(Command::Clone(crate::types::ComClone {
                plugin_name,
                plugin_data,
            }))
        }
        crate::constants::COM_SUBSCRIBE_GROUP_REPLICATION_STREAM => {
            if payload.len() < 8 {
                return Err(crate::error::Error::IncompletePacket(8 - payload.len()));
            }

            let flags = u64::from_le_bytes([
                payload[0], payload[1], payload[2], payload[3], payload[4], payload[5], payload[6],
                payload[7],
            ]);
            let payload = payload[8..].to_vec();

            Ok(Command::SubscribeGroupReplicationStream(
                crate::types::ComSubscribeGroupReplicationStream { flags, payload },
            ))
        }
        crate::constants::COM_END => Ok(Command::End),
        _ => Err(crate::error::Error::InvalidPacket(format!(
            "Unknown command type: {command_type}"
        ))),
    }
}

#[derive(Debug, Clone)]
pub struct CommandContext {
    pub statement_params: std::collections::HashMap<u32, u16>,
}

fn parse_stmt_execute_params_simple(
    data: &[u8],
) -> Result<Vec<crate::types::StmtParameter>, crate::error::Error> {
    let mut offset = 0;
    let mut params = Vec::new();

    if data.is_empty() {
        return Ok(params);
    }

    while offset < data.len() {
        if let Ok((string_data, consumed)) = crate::types::read_lenenc_string(&data[offset..])
            && consumed > 0
            && consumed <= data.len() - offset
        {
            params.push(crate::types::StmtParameter {
                value: Some(string_data.clone()),
                param_type: None,
                unsigned_flag: false,
            });
            offset += consumed;
            continue;
        }

        offset += 1;
    }

    Ok(params)
}

fn parse_stmt_execute_params(
    data: &[u8],
    num_params: u16,
) -> Result<Vec<crate::types::StmtParameter>, crate::error::Error> {
    if data.is_empty() || num_params == 0 {
        return Ok(Vec::new());
    }

    let mut offset = 0;
    let mut params = Vec::with_capacity(num_params as usize);

    let null_bitmap_len = num_params.div_ceil(8) as usize;
    if data.len() < null_bitmap_len {
        return Err(crate::error::Error::IncompletePacket(
            null_bitmap_len - data.len(),
        ));
    }
    let null_bitmap = &data[offset..offset + null_bitmap_len];
    offset += null_bitmap_len;

    if data.len() <= offset {
        return Err(crate::error::Error::IncompletePacket(1));
    }
    let new_params_bound_flag = data[offset];
    offset += 1;

    let mut param_types = Vec::with_capacity(num_params as usize);
    if new_params_bound_flag == 1 {
        for _ in 0..num_params {
            if data.len() < offset + 2 {
                return Err(crate::error::Error::IncompletePacket(
                    offset + 2 - data.len(),
                ));
            }
            let field_type = data[offset];
            let flags = data[offset + 1];
            param_types.push(crate::types::ParamType {
                field_type,
                unsigned_flag: (flags & 0x80) != 0,
            });
            offset += 2;
        }
    }
    for i in 0..num_params as usize {
        let byte_pos = i / 8;
        let bit_pos = i % 8;
        let is_null = (null_bitmap[byte_pos] & (1 << bit_pos)) != 0;

        if is_null {
            params.push(crate::types::StmtParameter {
                value: None,
                param_type: if i < param_types.len() {
                    Some(param_types[i].field_type as u16)
                } else {
                    None
                },
                unsigned_flag: if i < param_types.len() {
                    param_types[i].unsigned_flag
                } else {
                    false
                },
            });
        } else {
            let field_type = if i < param_types.len() {
                param_types[i].field_type
            } else {
                return Err(crate::error::Error::InvalidPacket(
                    "Missing parameter type information".to_string(),
                ));
            };

            let data = &data[offset..];
            let (value, consumed) = match field_type {
                crate::constants::MYSQL_TYPE_NULL => Ok((vec![], 0)),

                crate::constants::MYSQL_TYPE_TINY => {
                    if data.is_empty() {
                        return Err(crate::error::Error::IncompletePacket(1));
                    }
                    Ok((vec![data[0]], 1))
                }

                crate::constants::MYSQL_TYPE_SHORT | crate::constants::MYSQL_TYPE_YEAR => {
                    if data.len() < 2 {
                        return Err(crate::error::Error::IncompletePacket(2 - data.len()));
                    }
                    Ok((data[..2].to_vec(), 2))
                }

                crate::constants::MYSQL_TYPE_LONG | crate::constants::MYSQL_TYPE_INT24 => {
                    if data.len() < 4 {
                        return Err(crate::error::Error::IncompletePacket(4 - data.len()));
                    }
                    Ok((data[..4].to_vec(), 4))
                }

                crate::constants::MYSQL_TYPE_FLOAT => {
                    if data.len() < 4 {
                        return Err(crate::error::Error::IncompletePacket(4 - data.len()));
                    }
                    Ok((data[..4].to_vec(), 4))
                }

                crate::constants::MYSQL_TYPE_LONGLONG => {
                    if data.len() < 8 {
                        return Err(crate::error::Error::IncompletePacket(8 - data.len()));
                    }
                    Ok((data[..8].to_vec(), 8))
                }

                crate::constants::MYSQL_TYPE_DOUBLE => {
                    if data.len() < 8 {
                        return Err(crate::error::Error::IncompletePacket(8 - data.len()));
                    }
                    Ok((data[..8].to_vec(), 8))
                }

                crate::constants::MYSQL_TYPE_DATE
                | crate::constants::MYSQL_TYPE_DATETIME
                | crate::constants::MYSQL_TYPE_TIMESTAMP => {
                    if data.is_empty() {
                        return Err(crate::error::Error::IncompletePacket(1));
                    }
                    let length = data[0] as usize;
                    if data.len() < 1 + length {
                        return Err(crate::error::Error::IncompletePacket(
                            1 + length - data.len(),
                        ));
                    }
                    Ok((data[1..1 + length].to_vec(), 1 + length))
                }

                crate::constants::MYSQL_TYPE_TIME => {
                    if data.is_empty() {
                        return Err(crate::error::Error::IncompletePacket(1));
                    }
                    let length = data[0] as usize;
                    if data.len() < 1 + length {
                        return Err(crate::error::Error::IncompletePacket(
                            1 + length - data.len(),
                        ));
                    }
                    Ok((data[1..1 + length].to_vec(), 1 + length))
                }

                crate::constants::MYSQL_TYPE_VARCHAR
                | crate::constants::MYSQL_TYPE_VAR_STRING
                | crate::constants::MYSQL_TYPE_STRING
                | crate::constants::MYSQL_TYPE_BLOB
                | crate::constants::MYSQL_TYPE_TINY_BLOB
                | crate::constants::MYSQL_TYPE_MEDIUM_BLOB
                | crate::constants::MYSQL_TYPE_LONG_BLOB
                | crate::constants::MYSQL_TYPE_DECIMAL
                | crate::constants::MYSQL_TYPE_NEWDECIMAL
                | crate::constants::MYSQL_TYPE_BIT
                | crate::constants::MYSQL_TYPE_ENUM
                | crate::constants::MYSQL_TYPE_SET
                | crate::constants::MYSQL_TYPE_GEOMETRY
                | crate::constants::MYSQL_TYPE_JSON => crate::types::read_lenenc_string(data),

                _ => Err(crate::error::Error::InvalidPacket(format!(
                    "Unknown parameter type: {field_type}"
                ))),
            }?;
            params.push(crate::types::StmtParameter {
                value: Some(value),
                param_type: Some(field_type as u16),
                unsigned_flag: if i < param_types.len() {
                    param_types[i].unsigned_flag
                } else {
                    false
                },
            });
            offset += consumed;
        }
    }

    Ok(params)
}

#[derive(Debug, Clone)]
pub enum Command {
    Sleep,
    Quit,
    InitDb(crate::types::ComInitDb),
    Query(crate::types::ComQuery),
    FieldList(crate::types::ComFieldList),
    CreateDb(crate::types::ComCreateDb),
    DropDb(crate::types::ComDropDb),
    Statistics,
    Debug,
    Ping,
    ChangeUser(crate::types::ComChangeUser),
    BinlogDump(crate::types::ComBinlogDump),
    StmtPrepare(crate::types::ComStmtPrepare),
    StmtExecute(crate::types::ComStmtExecute),
    StmtSendLongData(crate::types::ComStmtSendLongData),
    StmtClose(u32),
    StmtReset(u32),
    SetOption(crate::types::ComSetOption),
    StmtFetch(crate::types::ComStmtFetch),
    BinlogDumpGtid(crate::types::ComBinlogDumpGtid),
    ResetConnection,
    Clone(crate::types::ComClone),
    SubscribeGroupReplicationStream(crate::types::ComSubscribeGroupReplicationStream),
    End,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_command_query() {
        let data = vec![crate::constants::COM_QUERY];
        let mut query_data = data;
        query_data.extend_from_slice(b"SELECT * FROM users");

        let result = parse_command(&query_data);
        assert!(result.is_ok());
        let command = result.unwrap();

        if let Command::Query(query) = command {
            assert_eq!(query.query, "SELECT * FROM users");
        } else {
            panic!("Expected Query command");
        }
    }

    #[test]
    fn test_parse_command_stmt_prepare() {
        let data = vec![crate::constants::COM_STMT_PREPARE];
        let mut query_data = data;
        query_data.extend_from_slice(b"SELECT * FROM users WHERE id = ?");

        let result = parse_command(&query_data);
        assert!(result.is_ok());
        let command = result.unwrap();

        if let Command::StmtPrepare(stmt) = command {
            assert_eq!(stmt.query, "SELECT * FROM users WHERE id = ?");
        } else {
            panic!("Expected StmtPrepare command");
        }
    }

    #[test]
    fn test_parse_command_stmt_execute() {
        let mut data = vec![crate::constants::COM_STMT_EXECUTE];
        data.extend_from_slice(&[0x01, 0x02, 0x03, 0x04]);
        data.push(0x00);
        data.extend_from_slice(&[0x01, 0x00, 0x00, 0x00]);

        let result = parse_command(&data);
        assert!(result.is_ok());
        let command = result.unwrap();

        if let Command::StmtExecute(execute) = command {
            assert_eq!(execute.statement_id, 0x04030201);
            assert_eq!(execute.flags, 0x00);
            assert_eq!(execute.iteration_count, 0x01);
        } else {
            panic!("Expected StmtExecute command");
        }
    }

    #[test]
    fn test_parse_command_quit() {
        let data = vec![crate::constants::COM_QUIT];

        let result = parse_command(&data);
        assert!(result.is_ok());
        let command = result.unwrap();

        assert!(matches!(command, Command::Quit));
    }

    #[test]
    fn test_parse_command_init_db() {
        let data = vec![crate::constants::COM_INIT_DB];
        let mut db_data = data;
        db_data.extend_from_slice(b"testdb");

        let result = parse_command(&db_data);
        assert!(result.is_ok());
        let command = result.unwrap();

        if let Command::InitDb(init_db) = command {
            assert_eq!(init_db.database, "testdb");
        } else {
            panic!("Expected InitDb command");
        }
    }

    #[test]
    fn test_parse_command_ping() {
        let data = vec![crate::constants::COM_PING];

        let result = parse_command(&data);
        assert!(result.is_ok());
        let command = result.unwrap();

        assert!(matches!(command, Command::Ping));
    }

    #[test]
    fn test_parse_command_stmt_close() {
        let mut data = vec![crate::constants::COM_STMT_CLOSE];
        data.extend_from_slice(&[0x01, 0x02, 0x03, 0x04]);

        let result = parse_command(&data);
        assert!(result.is_ok());
        let command = result.unwrap();

        if let Command::StmtClose(statement_id) = command {
            assert_eq!(statement_id, 0x04030201);
        } else {
            panic!("Expected StmtClose command");
        }
    }

    #[test]
    fn test_parse_command_empty_data() {
        let data = vec![];
        let result = parse_command(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(1))
        ));
    }

    #[test]
    fn test_parse_command_unknown_type() {
        let data = vec![0xFF];
        let result = parse_command(&data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_parse_command_stmt_execute_incomplete() {
        let mut data = vec![crate::constants::COM_STMT_EXECUTE];
        data.extend_from_slice(&[0x01, 0x02]);

        let result = parse_command(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_command_stmt_execute_with_params() {
        let mut data = vec![crate::constants::COM_STMT_EXECUTE];
        data.extend_from_slice(&[0x01, 0x00, 0x00, 0x00]);
        data.push(0x00);
        data.extend_from_slice(&[0x01, 0x00, 0x00, 0x00]);

        data.push(0x04);
        data.extend_from_slice(b"test");

        let result = parse_command(&data);
        assert!(result.is_ok());
        let command = result.unwrap();

        if let Command::StmtExecute(execute) = command {
            assert_eq!(execute.statement_id, 1);
            assert_eq!(execute.flags, 0);
            assert_eq!(execute.iteration_count, 1);
            assert!(!execute.params.is_empty());
            assert_eq!(execute.params[0].value, Some(b"test".to_vec()));
            assert_eq!(execute.params[0].param_type, None);
            assert!(!execute.params[0].unsigned_flag);
        } else {
            panic!("Expected StmtExecute command");
        }
    }

    #[test]
    fn test_parse_stmt_execute_params_empty() {
        let data = vec![];
        let result = parse_stmt_execute_params(&data, 0);
        assert!(result.is_ok());
        let params = result.unwrap();
        assert!(params.is_empty());
    }

    #[test]
    fn test_parse_stmt_execute_params_simple() {
        let mut data = vec![];
        data.push(0x05);
        data.extend_from_slice(b"hello");
        data.push(0x05);
        data.extend_from_slice(b"world");

        let result = parse_stmt_execute_params_simple(&data);
        assert!(result.is_ok());
        let params = result.unwrap();
        assert_eq!(params.len(), 2);
        assert_eq!(params[0].value, Some(b"hello".to_vec()));
        assert_eq!(params[1].value, Some(b"world".to_vec()));
    }

    #[test]
    fn test_parse_stmt_execute_params_with_null_bitmap() {
        let mut data = vec![
            0x01,
            0x01,
            crate::constants::MYSQL_TYPE_VARCHAR,
            0x00,
            crate::constants::MYSQL_TYPE_LONG,
            0x00,
        ];
        data.extend_from_slice(&[0x2A, 0x00, 0x00, 0x00]);

        let result = parse_stmt_execute_params(&data, 2);
        assert!(result.is_ok());
        let params = result.unwrap();
        assert_eq!(params.len(), 2);
        assert!(params[0].value.is_none());
        assert_eq!(
            params[0].param_type,
            Some(crate::constants::MYSQL_TYPE_VARCHAR as u16)
        );
        assert_eq!(params[1].value, Some(vec![0x2A, 0x00, 0x00, 0x00]));
        assert_eq!(
            params[1].param_type,
            Some(crate::constants::MYSQL_TYPE_LONG as u16)
        );
    }
}
