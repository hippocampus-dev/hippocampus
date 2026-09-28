//! MySQL Protocol OK Packet Parsing
//!
//! ## Protocol Documentation
//! - OK Packet: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_ok_packet.html>

pub fn parse_ok_packet(data: &[u8]) -> Result<crate::types::OkPacket, crate::error::Error> {
    if data.is_empty() {
        return Err(crate::error::Error::IncompletePacket(1));
    }

    if data[0] != crate::constants::OK_PACKET && data[0] != 0xFE {
        return Err(crate::error::Error::InvalidPacket(
            "Not an OK packet".to_string(),
        ));
    }

    let mut offset = 1;

    let (affected_rows, consumed) = crate::types::read_lenenc_int(&data[offset..])?;
    offset += consumed;

    let (last_insert_id, consumed) = crate::types::read_lenenc_int(&data[offset..])?;
    offset += consumed;

    if data.len() < offset + 2 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 2 - data.len(),
        ));
    }
    let status_flags = u16::from_le_bytes([data[offset], data[offset + 1]]);
    offset += 2;

    if data.len() < offset + 2 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 2 - data.len(),
        ));
    }
    let warnings = u16::from_le_bytes([data[offset], data[offset + 1]]);
    offset += 2;

    let info = if offset < data.len() {
        String::from_utf8(data[offset..].to_vec())?
    } else {
        String::new()
    };

    let session_state_changes =
        if status_flags & crate::constants::SERVER_SESSION_STATE_CHANGED != 0 {
            if let Ok((_, info_consumed)) = crate::types::read_lenenc_string(&data[offset..]) {
                offset += info_consumed;
                if let Ok((session_bytes, _)) = crate::types::read_lenenc_string(&data[offset..]) {
                    Some(session_bytes)
                } else {
                    None
                }
            } else {
                None
            }
        } else {
            None
        };

    Ok(crate::types::OkPacket {
        affected_rows,
        last_insert_id,
        status_flags,
        warnings,
        info,
        session_state_changes,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_ok_packet_simple() {
        let data = vec![
            0x00, // OK packet header
            0x00, // affected rows (0)
            0x00, // last insert id (0)
            0x02, 0x00, // status flags
            0x00, 0x00, // warnings
        ];

        let result = parse_ok_packet(&data);
        assert!(result.is_ok());
        let packet = result.unwrap();
        assert_eq!(packet.affected_rows, 0);
        assert_eq!(packet.last_insert_id, 0);
        assert_eq!(packet.status_flags, 0x0002);
        assert_eq!(packet.warnings, 0);
        assert_eq!(packet.info, "");
    }

    #[test]
    fn test_parse_ok_packet_with_info() {
        let mut data = vec![
            0x00, // OK packet header
            0x01, // affected rows (1)
            0x00, // last insert id (0)
            0x02, 0x00, // status flags
            0x00, 0x00, // warnings
        ];
        data.extend_from_slice(b"Rows matched: 1");

        let result = parse_ok_packet(&data);
        assert!(result.is_ok());
        let packet = result.unwrap();
        assert_eq!(packet.affected_rows, 1);
        assert_eq!(packet.last_insert_id, 0);
        assert_eq!(packet.status_flags, 0x0002);
        assert_eq!(packet.warnings, 0);
        assert_eq!(packet.info, "Rows matched: 1");
    }

    #[test]
    fn test_parse_ok_packet_eof_header() {
        let data = vec![
            0xFE, // EOF packet header (can be OK in newer protocols)
            0x00, // affected rows (0)
            0x00, // last insert id (0)
            0x02, 0x00, // status flags
            0x00, 0x00, // warnings
        ];

        let result = parse_ok_packet(&data);
        assert!(result.is_ok());
        let packet = result.unwrap();
        assert_eq!(packet.affected_rows, 0);
        assert_eq!(packet.last_insert_id, 0);
    }

    #[test]
    fn test_parse_ok_packet_large_values() {
        let data = vec![
            0x00, // OK packet header
            0xFC, 0x10, 0x27, // affected rows (10000 in lenenc)
            0xFC, 0x20, 0x4E, // last insert id (20000 in lenenc)
            0x02, 0x00, // status flags
            0x00, 0x00, // warnings
        ];

        let result = parse_ok_packet(&data);
        assert!(result.is_ok());
        let packet = result.unwrap();
        assert_eq!(packet.affected_rows, 10000);
        assert_eq!(packet.last_insert_id, 20000);
    }

    #[test]
    fn test_parse_ok_packet_empty_data() {
        let data = vec![];
        let result = parse_ok_packet(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(1))
        ));
    }

    #[test]
    fn test_parse_ok_packet_invalid_header() {
        let data = vec![0xFF]; // Error packet header
        let result = parse_ok_packet(&data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_parse_ok_packet_incomplete() {
        let data = vec![
            0x00, // OK packet header
            0x00, // affected rows (0)
                  // Missing rest of packet
        ];
        let result = parse_ok_packet(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }
}
