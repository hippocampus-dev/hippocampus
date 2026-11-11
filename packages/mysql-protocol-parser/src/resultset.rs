//! MySQL Protocol Result Set Parsing (OK/EOF/Column Definition/Text Result Set)
//!
//! ## Protocol Documentation
//! - OK Packet: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_ok_packet.html>
//! - EOF Packet: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_eof_packet.html>
//! - Result Set Packets: <https://mariadb.com/kb/en/result-set-packets/>
//! - Column Definition: <https://dev.mysql.com/doc/internals/en/com-query-response.html>
//! - Text Protocol: <https://dev.mysql.com/doc/internals/en/text-protocol.html>
//! - Text Resultset Row: <https://dev.mysql.com/doc/internals/en/com-query-response.html#packet-ProtocolText::ResultsetRow>

pub fn parse_column_definition(
    data: &[u8],
) -> Result<(crate::types::ColumnDefinition, usize), crate::error::Error> {
    let mut offset = 0;

    let (catalog_bytes, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
    let catalog = String::from_utf8(catalog_bytes)?;
    offset += consumed;

    let (schema_bytes, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
    let schema = String::from_utf8(schema_bytes)?;
    offset += consumed;

    let (table_bytes, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
    let table = String::from_utf8(table_bytes)?;
    offset += consumed;

    let (org_table_bytes, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
    let org_table = String::from_utf8(org_table_bytes)?;
    offset += consumed;

    let (name_bytes, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
    let name = String::from_utf8(name_bytes)?;
    offset += consumed;

    let (org_name_bytes, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
    let org_name = String::from_utf8(org_name_bytes)?;
    offset += consumed;

    if data.len() < offset + 1 || data[offset] != 0x0C {
        return Err(crate::error::Error::InvalidPacket(
            "Invalid fixed length field".to_string(),
        ));
    }
    offset += 1;

    if data.len() < offset + 2 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 2 - data.len(),
        ));
    }
    let character_set = u16::from_le_bytes([data[offset], data[offset + 1]]);
    offset += 2;

    if data.len() < offset + 4 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 4 - data.len(),
        ));
    }
    let column_length = u32::from_le_bytes([
        data[offset],
        data[offset + 1],
        data[offset + 2],
        data[offset + 3],
    ]);
    offset += 4;

    if data.len() < offset + 1 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 1 - data.len(),
        ));
    }
    let column_type = data[offset];
    offset += 1;

    if data.len() < offset + 2 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 2 - data.len(),
        ));
    }
    let flags = u16::from_le_bytes([data[offset], data[offset + 1]]);
    offset += 2;

    if data.len() < offset + 1 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 1 - data.len(),
        ));
    }
    let decimals = data[offset];
    offset += 1;

    if data.len() < offset + 2 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 2 - data.len(),
        ));
    }
    offset += 2;

    Ok((
        crate::types::ColumnDefinition {
            catalog,
            schema,
            table,
            org_table,
            name,
            org_name,
            character_set,
            column_length,
            column_type,
            flags,
            decimals,
        },
        offset,
    ))
}

pub fn parse_result_row_text(
    data: &[u8],
    columns: &[crate::types::ColumnDefinition],
) -> Result<(crate::types::ResultSetRow, usize), crate::error::Error> {
    if columns.is_empty() {
        return Ok((crate::types::ResultSetRow { values: vec![] }, 0));
    }

    let mut offset = 0;
    let mut values = Vec::with_capacity(columns.len());

    for _ in columns {
        if data.len() <= offset {
            return Err(crate::error::Error::IncompletePacket(1));
        }

        if data[offset] == 0xFB {
            values.push(None);
            offset += 1;
        } else {
            let (value, consumed) = crate::types::read_lenenc_string(&data[offset..])?;
            values.push(Some(value));
            offset += consumed;
        }
    }

    Ok((crate::types::ResultSetRow { values }, offset))
}

#[derive(Debug, Clone)]
pub struct ResultSet {
    pub column_count: u64,
    pub columns: Vec<crate::types::ColumnDefinition>,
    pub rows: Vec<crate::types::ResultSetRow>,
    pub is_binary: bool,
}

fn parse_result_set_from_start(
    data: &[u8],
    is_binary: bool,
) -> Result<QueryResponse, crate::error::Error> {
    let (first_packet, first_consumed) = crate::header::read_packet(data)?;
    let (column_count, _) = crate::types::read_lenenc_int(first_packet)?;
    let mut current_offset = first_consumed;
    let mut columns = Vec::with_capacity(column_count as usize);

    for _ in 0..column_count {
        let (payload, consumed) = crate::header::read_packet(&data[current_offset..])?;
        current_offset += consumed;
        let (column, _) = parse_column_definition(payload)?;
        columns.push(column);
    }

    if data.len() > current_offset + 4 && data[current_offset + 4] == crate::constants::EOF_PACKET {
        let (_, consumed) = crate::header::read_packet(&data[current_offset..])?;
        current_offset += consumed;
    }

    let mut rows = Vec::new();
    while current_offset < data.len() {
        let (payload, consumed) = crate::header::read_packet(&data[current_offset..])?;
        current_offset += consumed;

        if payload.len() >= 5 && payload[0] == crate::constants::EOF_PACKET {
            break;
        }

        let (row, _) = if is_binary {
            crate::binary_resultset::parse_result_row_binary(payload, &columns)?
        } else {
            parse_result_row_text(payload, &columns)?
        };
        rows.push(row);
    }

    Ok(QueryResponse::ResultSet(ResultSet {
        column_count,
        columns,
        rows,
        is_binary,
    }))
}

pub fn parse_stmt_prepare_response(data: &[u8]) -> Result<QueryResponse, crate::error::Error> {
    if data.is_empty() {
        return Err(crate::error::Error::IncompletePacket(1));
    }

    let (first_packet, _offset) = crate::header::read_packet(data)?;

    match first_packet[0] {
        crate::constants::OK_PACKET => {
            if first_packet.len() < 12 {
                return Err(crate::error::Error::IncompletePacket(
                    12 - first_packet.len(),
                ));
            }

            let statement_id = u32::from_le_bytes([
                first_packet[1],
                first_packet[2],
                first_packet[3],
                first_packet[4],
            ]);
            let num_columns = u16::from_le_bytes([first_packet[5], first_packet[6]]);
            let num_params = u16::from_le_bytes([first_packet[7], first_packet[8]]);
            let warning_count = u16::from_le_bytes([first_packet[10], first_packet[11]]);

            Ok(QueryResponse::StmtPrepareOk(crate::types::StmtPrepareOk {
                statement_id,
                num_columns,
                num_params,
                warning_count,
            }))
        }
        crate::constants::ERR_PACKET => {
            let (error_packet, _) = crate::error_packet::parse_error_packet(first_packet)?;
            Ok(QueryResponse::Error(error_packet))
        }
        _ => Err(crate::error::Error::InvalidPacket(
            "Invalid COM_STMT_PREPARE response".to_string(),
        )),
    }
}

pub fn parse_query_response(data: &[u8]) -> Result<QueryResponse, crate::error::Error> {
    if data.is_empty() {
        return Err(crate::error::Error::IncompletePacket(1));
    }

    if data.len() >= 5
        && data[4] != crate::constants::OK_PACKET
        && data[4] != crate::constants::ERR_PACKET
    {
        return parse_result_set_from_start(data, false);
    }

    let (first_packet, _first_consumed) = crate::header::read_packet(data)?;

    match first_packet[0] {
        crate::constants::OK_PACKET => {
            let ok_packet = crate::ok_packet::parse_ok_packet(first_packet)?;
            Ok(QueryResponse::Ok(ok_packet))
        }
        crate::constants::ERR_PACKET => {
            let (error_packet, _) = crate::error_packet::parse_error_packet(first_packet)?;
            Ok(QueryResponse::Error(error_packet))
        }
        _ => parse_result_set_from_start(data, false),
    }
}

pub fn parse_stmt_execute_response(data: &[u8]) -> Result<QueryResponse, crate::error::Error> {
    if data.is_empty() {
        return Err(crate::error::Error::IncompletePacket(1));
    }

    if data.len() >= 5
        && data[4] != crate::constants::OK_PACKET
        && data[4] != crate::constants::ERR_PACKET
    {
        return parse_result_set_from_start(data, true);
    }

    let (first_packet, _first_consumed) = crate::header::read_packet(data)?;

    match first_packet[0] {
        crate::constants::OK_PACKET => {
            let ok_packet = crate::ok_packet::parse_ok_packet(first_packet)?;
            Ok(QueryResponse::Ok(ok_packet))
        }
        crate::constants::ERR_PACKET => {
            let (error_packet, _) = crate::error_packet::parse_error_packet(first_packet)?;
            Ok(QueryResponse::Error(error_packet))
        }
        _ => parse_result_set_from_start(data, true),
    }
}

#[derive(Debug, Clone)]
pub enum QueryResponse {
    Ok(crate::types::OkPacket),
    Error(crate::types::ErrorPacket),
    ResultSet(ResultSet),
    StmtPrepareOk(crate::types::StmtPrepareOk),
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_column_definition_success() {
        let mut data = vec![];
        data.push(0x03); // catalog length
        data.extend_from_slice(b"def"); // catalog
        data.push(0x04); // schema length
        data.extend_from_slice(b"test"); // schema
        data.push(0x05); // table length
        data.extend_from_slice(b"users"); // table
        data.push(0x05); // org_table length
        data.extend_from_slice(b"users"); // org_table
        data.push(0x02); // name length
        data.extend_from_slice(b"id"); // name
        data.push(0x02); // org_name length
        data.extend_from_slice(b"id"); // org_name
        data.push(0x0C); // fixed length field marker
        data.extend_from_slice(&[0x21, 0x00]); // character set
        data.extend_from_slice(&[0x0B, 0x00, 0x00, 0x00]); // column length
        data.push(0x03); // column type (MYSQL_TYPE_LONG)
        data.extend_from_slice(&[0x03, 0x42]); // flags
        data.push(0x00); // decimals
        data.extend_from_slice(&[0x00, 0x00]); // reserved

        let result = parse_column_definition(&data);
        assert!(result.is_ok());
        let (column, consumed) = result.unwrap();
        assert_eq!(column.catalog, "def");
        assert_eq!(column.schema, "test");
        assert_eq!(column.table, "users");
        assert_eq!(column.org_table, "users");
        assert_eq!(column.name, "id");
        assert_eq!(column.org_name, "id");
        assert_eq!(column.character_set, 0x0021);
        assert_eq!(column.column_length, 0x0000000B);
        assert_eq!(column.column_type, 0x03);
        assert_eq!(column.flags, 0x4203);
        assert_eq!(column.decimals, 0x00);
        assert!(consumed > 0);
    }

    #[test]
    fn test_parse_column_definition_invalid_fixed_length() {
        let mut data = vec![];
        data.push(0x03); // catalog length
        data.extend_from_slice(b"def"); // catalog
        data.push(0x04); // schema length
        data.extend_from_slice(b"test"); // schema
        data.push(0x05); // table length
        data.extend_from_slice(b"users"); // table
        data.push(0x05); // org_table length
        data.extend_from_slice(b"users"); // org_table
        data.push(0x02); // name length
        data.extend_from_slice(b"id"); // name
        data.push(0x02); // org_name length
        data.extend_from_slice(b"id"); // org_name
        data.push(0x0D); // invalid fixed length field marker (should be 0x0C)

        let result = parse_column_definition(&data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_parse_column_definition_incomplete() {
        let mut data = vec![];
        data.push(0x03); // catalog length
        data.extend_from_slice(b"def"); // catalog
        data.push(0x04); // schema length
        data.extend_from_slice(b"te"); // incomplete schema

        let result = parse_column_definition(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_result_row_text_empty_columns() {
        let data = vec![];
        let columns = vec![];
        let result = parse_result_row_text(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 0);
        assert_eq!(consumed, 0);
    }

    #[test]
    fn test_parse_result_row_text_single_null() {
        let data = vec![0xFB];
        let columns = vec![create_test_column()];
        let result = parse_result_row_text(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 1);
        assert!(row.values[0].is_none());
        assert_eq!(consumed, 1);
    }

    #[test]
    fn test_parse_result_row_text_single_string() {
        let mut data = vec![];
        data.push(0x05); // length
        data.extend_from_slice(b"Hello");

        let columns = vec![create_test_column()];
        let result = parse_result_row_text(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 1);
        assert_eq!(row.values[0], Some(b"Hello".to_vec()));
        assert_eq!(consumed, 6);
    }

    #[test]
    fn test_parse_result_row_text_mixed_values() {
        let mut data = vec![];
        data.push(0x03); // length
        data.extend_from_slice(b"123");
        data.push(0xFB); // NULL
        data.push(0x05); // length
        data.extend_from_slice(b"World");

        let columns = vec![
            create_test_column(),
            create_test_column(),
            create_test_column(),
        ];
        let result = parse_result_row_text(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 3);
        assert_eq!(row.values[0], Some(b"123".to_vec()));
        assert!(row.values[1].is_none());
        assert_eq!(row.values[2], Some(b"World".to_vec()));
        assert_eq!(consumed, 11);
    }

    #[test]
    fn test_parse_result_row_text_empty_strings() {
        let data = vec![0x00, 0x00]; // empty strings

        let columns = vec![create_test_column(), create_test_column()];
        let result = parse_result_row_text(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 2);
        assert_eq!(row.values[0], Some(vec![]));
        assert_eq!(row.values[1], Some(vec![]));
        assert_eq!(consumed, 2);
    }

    #[test]
    fn test_parse_result_row_text_incomplete() {
        let data = vec![0x05, 0x01, 0x02]; // length 5 but only 2 bytes of data
        let columns = vec![create_test_column()];
        let result = parse_result_row_text(&data, &columns);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_result_row_text_missing_column_data() {
        let data = vec![0xFB]; // Only one value
        let columns = vec![create_test_column(), create_test_column()];
        let result = parse_result_row_text(&data, &columns);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_result_row_text_large_string() {
        let mut data = vec![0xFC, 0x00, 0x01]; // 256 bytes
        data.extend(vec![b'A'; 256]);

        let columns = vec![create_test_column()];
        let result = parse_result_row_text(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 1);
        assert_eq!(row.values[0].as_ref().unwrap().len(), 256);
        assert_eq!(consumed, 259);
    }

    fn create_test_column() -> crate::types::ColumnDefinition {
        crate::types::ColumnDefinition {
            catalog: String::from("def"),
            schema: String::from("test"),
            table: String::from("test_table"),
            org_table: String::from("test_table"),
            name: String::from("test_column"),
            org_name: String::from("test_column"),
            character_set: 0x21,
            column_length: 255,
            column_type: crate::constants::MYSQL_TYPE_VARCHAR,
            flags: 0,
            decimals: 0,
        }
    }

    #[test]
    fn test_parse_column_count_packet() {
        let data = vec![0x02]; // column count = 2
        let (count, consumed) = crate::types::read_lenenc_int(&data).unwrap();
        assert_eq!(count, 2);
        assert_eq!(consumed, 1);
    }

    #[test]
    fn test_parse_text_row() {
        let columns = vec![create_test_column(), create_test_column()];

        let mut row_data = vec![];
        row_data.push(0x01);
        row_data.extend_from_slice(b"1");
        row_data.push(0x04);
        row_data.extend_from_slice(b"John");

        let (row, consumed) = parse_result_row_text(&row_data, &columns).unwrap();
        assert_eq!(row.values.len(), 2);
        assert_eq!(row.values[0], Some(b"1".to_vec()));
        assert_eq!(row.values[1], Some(b"John".to_vec()));
        assert_eq!(consumed, 7);
    }

    #[test]
    fn test_parse_stmt_prepare_response_success() {
        let packet_data = vec![
            crate::constants::OK_PACKET,
            0x01,
            0x00,
            0x00,
            0x00,
            0x02,
            0x00,
            0x01,
            0x00,
            0x00,
            0x00,
            0x00,
        ];

        let mut data = vec![];
        data.extend_from_slice(&[packet_data.len() as u8, 0x00, 0x00]);
        data.push(0x00);
        data.extend_from_slice(&packet_data);

        let result = parse_stmt_prepare_response(&data);
        assert!(result.is_ok());
        match result.unwrap() {
            QueryResponse::StmtPrepareOk(stmt) => {
                assert_eq!(stmt.statement_id, 1);
                assert_eq!(stmt.num_columns, 2);
                assert_eq!(stmt.num_params, 1);
                assert_eq!(stmt.warning_count, 0);
            }
            _ => panic!("Expected StmtPrepareOk response"),
        }
    }

    #[test]
    fn test_parse_stmt_prepare_response_error() {
        let mut packet_data = vec![crate::constants::ERR_PACKET, 0x00, 0x04, b'#'];
        packet_data.extend_from_slice(b"HY000");
        packet_data.extend_from_slice(b"Syntax error");

        let mut data = vec![];
        data.extend_from_slice(&[packet_data.len() as u8, 0x00, 0x00]);
        data.push(0x00);
        data.extend_from_slice(&packet_data);

        let result = parse_stmt_prepare_response(&data);
        assert!(result.is_ok());
        match result.unwrap() {
            QueryResponse::Error(err) => {
                assert_eq!(err.error_code, 1024);
                assert_eq!(err.sql_state, Some("HY000".to_string()));
                assert_eq!(err.error_message, "Syntax error");
            }
            _ => panic!("Expected Error response"),
        }
    }

    #[test]
    fn test_parse_stmt_execute_response_ok() {
        let packet_data = vec![
            crate::constants::OK_PACKET,
            0x00,
            0x00,
            0x02,
            0x00,
            0x00,
            0x00,
        ];

        let mut data = vec![];
        data.extend_from_slice(&[packet_data.len() as u8, 0x00, 0x00]);
        data.push(0x00);
        data.extend_from_slice(&packet_data);

        let result = parse_stmt_execute_response(&data);
        assert!(result.is_ok());
        match result.unwrap() {
            QueryResponse::Ok(ok) => {
                assert_eq!(ok.affected_rows, 0);
                assert_eq!(ok.last_insert_id, 0);
                assert_eq!(ok.status_flags, 2);
                assert_eq!(ok.warnings, 0);
            }
            _ => panic!("Expected Ok response"),
        }
    }

    #[test]
    fn test_parse_stmt_execute_response_binary_resultset() {
        // Binary result set: column count packet
        let column_count_packet = vec![0x02]; // 2 columns

        // Column definition packets (simplified)
        let mut col1_data = vec![];
        col1_data.push(0x03);
        col1_data.extend_from_slice(b"def");
        col1_data.push(0x00); // schema
        col1_data.push(0x00); // table
        col1_data.push(0x00); // org_table
        col1_data.push(0x02);
        col1_data.extend_from_slice(b"id"); // name
        col1_data.push(0x02);
        col1_data.extend_from_slice(b"id"); // org_name
        col1_data.push(0x0C); // fixed length marker
        col1_data.extend_from_slice(&[0x21, 0x00]); // charset
        col1_data.extend_from_slice(&[0x0B, 0x00, 0x00, 0x00]); // length
        col1_data.push(crate::constants::MYSQL_TYPE_LONG); // type
        col1_data.extend_from_slice(&[0x00, 0x00]); // flags
        col1_data.push(0x00); // decimals
        col1_data.extend_from_slice(&[0x00, 0x00]); // reserved

        let mut col2_data = vec![];
        col2_data.push(0x03);
        col2_data.extend_from_slice(b"def");
        col2_data.push(0x00); // schema
        col2_data.push(0x00); // table
        col2_data.push(0x00); // org_table
        col2_data.push(0x04);
        col2_data.extend_from_slice(b"name"); // name
        col2_data.push(0x04);
        col2_data.extend_from_slice(b"name"); // org_name
        col2_data.push(0x0C); // fixed length marker
        col2_data.extend_from_slice(&[0x21, 0x00]); // charset
        col2_data.extend_from_slice(&[0xFF, 0x00, 0x00, 0x00]); // length
        col2_data.push(crate::constants::MYSQL_TYPE_VARCHAR); // type
        col2_data.extend_from_slice(&[0x00, 0x00]); // flags
        col2_data.push(0x00); // decimals
        col2_data.extend_from_slice(&[0x00, 0x00]); // reserved

        // EOF packet
        let eof_data = vec![
            crate::constants::EOF_PACKET,
            0x00,
            0x00, // warnings
            0x02,
            0x00, // status flags
        ];

        // Build complete packet stream
        let mut data = vec![];

        // Column count packet
        data.extend_from_slice(&[column_count_packet.len() as u8, 0x00, 0x00]);
        data.push(0x00);
        data.extend_from_slice(&column_count_packet);

        // Column 1 definition
        data.extend_from_slice(&[col1_data.len() as u8, 0x00, 0x00]);
        data.push(0x01);
        data.extend_from_slice(&col1_data);

        // Column 2 definition
        data.extend_from_slice(&[col2_data.len() as u8, 0x00, 0x00]);
        data.push(0x02);
        data.extend_from_slice(&col2_data);

        // EOF packet after columns
        data.extend_from_slice(&[eof_data.len() as u8, 0x00, 0x00]);
        data.push(0x03);
        data.extend_from_slice(&eof_data);

        let result = parse_stmt_execute_response(&data);
        assert!(result.is_ok());
        match result.unwrap() {
            QueryResponse::ResultSet(rs) => {
                assert_eq!(rs.column_count, 2);
                assert_eq!(rs.columns.len(), 2);
                assert!(rs.is_binary);
                assert_eq!(rs.columns[0].name, "id");
                assert_eq!(rs.columns[1].name, "name");
            }
            _ => panic!("Expected ResultSet response"),
        }
    }

    #[test]
    fn test_parse_query_response_text_resultset() {
        // Text result set: column count packet
        let column_count_packet = vec![0x01]; // 1 column

        // Column definition packet
        let mut col_data = vec![];
        col_data.push(0x03);
        col_data.extend_from_slice(b"def");
        col_data.push(0x04);
        col_data.extend_from_slice(b"test"); // schema
        col_data.push(0x05);
        col_data.extend_from_slice(b"users"); // table
        col_data.push(0x05);
        col_data.extend_from_slice(b"users"); // org_table
        col_data.push(0x05);
        col_data.extend_from_slice(b"count"); // name
        col_data.push(0x05);
        col_data.extend_from_slice(b"count"); // org_name
        col_data.push(0x0C); // fixed length marker
        col_data.extend_from_slice(&[0x21, 0x00]); // charset
        col_data.extend_from_slice(&[0x15, 0x00, 0x00, 0x00]); // length
        col_data.push(crate::constants::MYSQL_TYPE_LONGLONG); // type
        col_data.extend_from_slice(&[0x00, 0x00]); // flags
        col_data.push(0x00); // decimals
        col_data.extend_from_slice(&[0x00, 0x00]); // reserved

        // Row data
        let mut row_data = vec![];
        row_data.push(0x02);
        row_data.extend_from_slice(b"42"); // "42"

        // EOF packet
        let eof_data = vec![
            crate::constants::EOF_PACKET,
            0x00,
            0x00, // warnings
            0x02,
            0x00, // status flags
        ];

        // Build complete packet stream
        let mut data = vec![];

        // Column count packet
        data.extend_from_slice(&[column_count_packet.len() as u8, 0x00, 0x00]);
        data.push(0x00);
        data.extend_from_slice(&column_count_packet);

        // Column definition
        data.extend_from_slice(&[col_data.len() as u8, 0x00, 0x00]);
        data.push(0x01);
        data.extend_from_slice(&col_data);

        // Row packet
        data.extend_from_slice(&[row_data.len() as u8, 0x00, 0x00]);
        data.push(0x02);
        data.extend_from_slice(&row_data);

        // EOF packet
        data.extend_from_slice(&[eof_data.len() as u8, 0x00, 0x00]);
        data.push(0x03);
        data.extend_from_slice(&eof_data);

        let result = parse_query_response(&data);
        assert!(result.is_ok());
        match result.unwrap() {
            QueryResponse::ResultSet(rs) => {
                assert_eq!(rs.column_count, 1);
                assert_eq!(rs.columns.len(), 1);
                assert!(!rs.is_binary);
                assert_eq!(rs.columns[0].name, "count");
                assert_eq!(rs.rows.len(), 1);
                assert_eq!(rs.rows[0].values[0], Some(b"42".to_vec()));
            }
            _ => panic!("Expected ResultSet response"),
        }
    }
}
