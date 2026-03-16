//! MySQL Binary Protocol Result Set Parsing
//!
//! ## Protocol Documentation
//! - Binary Protocol Value: <https://dev.mysql.com/doc/internals/en/binary-protocol-value.html>
//! - Binary Protocol Result Set: <https://dev.mysql.com/doc/internals/en/binary-protocol-resultset.html>

pub fn parse_result_row_binary(
    data: &[u8],
    columns: &[crate::types::ColumnDefinition],
) -> Result<(crate::types::ResultSetRow, usize), crate::error::Error> {
    if columns.is_empty() {
        return Ok((crate::types::ResultSetRow { values: vec![] }, 0));
    }

    let mut offset = 0;

    if data.is_empty() || data[0] != 0x00 {
        return Err(crate::error::Error::InvalidPacket(
            "Binary result row must start with 0x00".to_string(),
        ));
    }
    offset += 1;

    let null_bitmap_len = (columns.len() + 7 + 2) / 8;
    if data.len() < offset + null_bitmap_len {
        return Err(crate::error::Error::IncompletePacket(
            offset + null_bitmap_len - data.len(),
        ));
    }

    let null_bitmap = &data[offset..offset + null_bitmap_len];
    offset += null_bitmap_len;

    let mut values = Vec::with_capacity(columns.len());

    for (i, column) in columns.iter().enumerate() {
        let byte_pos = (i + 2) / 8;
        let bit_pos = (i + 2) % 8;
        let is_null = (null_bitmap[byte_pos] & (1 << bit_pos)) != 0;

        if is_null {
            values.push(None);
        } else {
            let (value, consumed) = parse_binary_value(&data[offset..], column.column_type)?;
            values.push(Some(value));
            offset += consumed;
        }
    }

    Ok((crate::types::ResultSetRow { values }, offset))
}

fn parse_binary_value(
    data: &[u8],
    field_type: u8,
) -> Result<(Vec<u8>, usize), crate::error::Error> {
    match field_type {
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
        | crate::constants::MYSQL_TYPE_GEOMETRY => {
            let (value, consumed) = crate::types::read_lenenc_string(data)?;
            Ok((value, consumed))
        }

        crate::constants::MYSQL_TYPE_NEWDATE
        | crate::constants::MYSQL_TYPE_TIMESTAMP2
        | crate::constants::MYSQL_TYPE_DATETIME2
        | crate::constants::MYSQL_TYPE_TIME2
        | crate::constants::MYSQL_TYPE_JSON => {
            let (value, consumed) = crate::types::read_lenenc_string(data)?;
            Ok((value, consumed))
        }

        _ => Err(crate::error::Error::InvalidPacket(format!(
            "Unknown binary field type: {field_type}"
        ))),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_result_row_binary_empty_columns() {
        let data = vec![];
        let columns = vec![];
        let result = parse_result_row_binary(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 0);
        assert_eq!(consumed, 0);
    }

    #[test]
    fn test_parse_result_row_binary_all_null() {
        let mut data = vec![0x00];
        data.push(0xFF);

        let columns = vec![
            create_test_column(crate::constants::MYSQL_TYPE_LONG),
            create_test_column(crate::constants::MYSQL_TYPE_STRING),
        ];

        let result = parse_result_row_binary(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 2);
        assert!(row.values[0].is_none());
        assert!(row.values[1].is_none());
        assert_eq!(consumed, 2);
    }

    #[test]
    fn test_parse_result_row_binary_mixed_values() {
        let mut data = vec![0x00];
        data.push(0x08);
        data.extend_from_slice(&[0x2A, 0x00, 0x00, 0x00]);

        let columns = vec![
            create_test_column(crate::constants::MYSQL_TYPE_LONG),
            create_test_column(crate::constants::MYSQL_TYPE_STRING),
        ];

        let result = parse_result_row_binary(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 2);
        assert_eq!(row.values[0], Some(vec![0x2A, 0x00, 0x00, 0x00]));
        assert!(row.values[1].is_none());
        assert_eq!(consumed, 6);
    }

    #[test]
    fn test_parse_result_row_binary_string_value() {
        let mut data = vec![0x00];
        data.push(0x00);
        data.push(0x05);
        data.extend_from_slice(b"Hello");

        let columns = vec![create_test_column(crate::constants::MYSQL_TYPE_VAR_STRING)];

        let result = parse_result_row_binary(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 1);
        assert_eq!(row.values[0], Some(b"Hello".to_vec()));
        assert_eq!(consumed, 8);
    }

    #[test]
    fn test_parse_result_row_binary_datetime() {
        let mut data = vec![0x00];
        data.push(0x00);
        data.push(0x07);
        data.extend_from_slice(&[0xE5, 0x07, 0x0C, 0x19, 0x0E, 0x1E, 0x00]);

        let columns = vec![create_test_column(crate::constants::MYSQL_TYPE_DATETIME)];

        let result = parse_result_row_binary(&data, &columns);
        assert!(result.is_ok());
        let (row, consumed) = result.unwrap();
        assert_eq!(row.values.len(), 1);
        assert_eq!(
            row.values[0],
            Some(vec![0xE5, 0x07, 0x0C, 0x19, 0x0E, 0x1E, 0x00])
        );
        assert_eq!(consumed, 10);
    }

    #[test]
    fn test_parse_result_row_binary_invalid_header() {
        let data = vec![0x01];
        let columns = vec![create_test_column(crate::constants::MYSQL_TYPE_LONG)];

        let result = parse_result_row_binary(&data, &columns);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_parse_result_row_binary_incomplete_null_bitmap() {
        let data = vec![0x00];
        let columns = vec![create_test_column(crate::constants::MYSQL_TYPE_LONG)];

        let result = parse_result_row_binary(&data, &columns);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_binary_value_types() {
        let data = vec![0x42];
        let (value, consumed) =
            parse_binary_value(&data, crate::constants::MYSQL_TYPE_TINY).unwrap();
        assert_eq!(value, vec![0x42]);
        assert_eq!(consumed, 1);

        let data = vec![0x34, 0x12];
        let (value, consumed) =
            parse_binary_value(&data, crate::constants::MYSQL_TYPE_SHORT).unwrap();
        assert_eq!(value, vec![0x34, 0x12]);
        assert_eq!(consumed, 2);

        let data = vec![0x00, 0x00, 0x80, 0x3F];
        let (value, consumed) =
            parse_binary_value(&data, crate::constants::MYSQL_TYPE_FLOAT).unwrap();
        assert_eq!(value, vec![0x00, 0x00, 0x80, 0x3F]);
        assert_eq!(consumed, 4);

        let data = vec![0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08];
        let (value, consumed) =
            parse_binary_value(&data, crate::constants::MYSQL_TYPE_LONGLONG).unwrap();
        assert_eq!(value, vec![0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08]);
        assert_eq!(consumed, 8);

        let data = vec![];
        let (value, consumed) =
            parse_binary_value(&data, crate::constants::MYSQL_TYPE_NULL).unwrap();
        assert_eq!(value, vec![]);
        assert_eq!(consumed, 0);
    }

    #[test]
    fn test_parse_binary_value_unknown_type() {
        let data = vec![0x00];
        let result = parse_binary_value(&data, 0xEE);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    fn create_test_column(column_type: u8) -> crate::types::ColumnDefinition {
        crate::types::ColumnDefinition {
            catalog: String::from("def"),
            schema: String::from("test"),
            table: String::from("test_table"),
            org_table: String::from("test_table"),
            name: String::from("test_column"),
            org_name: String::from("test_column"),
            character_set: 0x21,
            column_length: 11,
            column_type,
            flags: 0,
            decimals: 0,
        }
    }
}
