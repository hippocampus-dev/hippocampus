//! MySQL Protocol Error Packet Parsing
//!
//! ## Protocol Documentation
//! - ERR_Packet: <https://dev.mysql.com/doc/internals/en/packet-ERR_Packet.html>
//! - Generic Response Packets: <https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_basic_response_packets.html>

pub fn parse_error_packet(
    data: &[u8],
) -> Result<(crate::types::ErrorPacket, usize), crate::error::Error> {
    let mut offset = 0;

    if data.is_empty() || data[0] != crate::constants::ERR_PACKET {
        return Err(crate::error::Error::InvalidPacket(
            "Not an error packet".to_string(),
        ));
    }
    offset += 1;

    if data.len() < offset + 2 {
        return Err(crate::error::Error::IncompletePacket(
            offset + 2 - data.len(),
        ));
    }
    let error_code = u16::from_le_bytes([data[offset], data[offset + 1]]);
    offset += 2;

    let (sql_state_marker, sql_state) = if offset < data.len() && data[offset] == b'#' {
        let marker = data[offset];
        offset += 1;

        if data.len() < offset + 5 {
            return Err(crate::error::Error::IncompletePacket(
                offset + 5 - data.len(),
            ));
        }
        let state = String::from_utf8(data[offset..offset + 5].to_vec())?;
        offset += 5;

        (Some(marker), Some(state))
    } else {
        (None, None)
    };

    let error_message = String::from_utf8(data[offset..].to_vec())?;

    Ok((
        crate::types::ErrorPacket {
            error_code,
            sql_state_marker,
            sql_state,
            error_message,
        },
        data.len(),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_error_packet_success() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code 1024
        data.push(b'#'); // SQL state marker
        data.extend_from_slice(b"HY000"); // SQL state
        data.extend_from_slice(b"Access denied"); // error message

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, consumed) = result.unwrap();
        assert_eq!(error_packet.error_code, 1024);
        assert_eq!(error_packet.sql_state_marker, Some(b'#'));
        assert_eq!(error_packet.sql_state, Some("HY000".to_string()));
        assert_eq!(error_packet.error_message, "Access denied");
        assert_eq!(consumed, data.len());
    }

    #[test]
    fn test_parse_error_packet_without_sql_state() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code 1024
        data.extend_from_slice(b"Access denied"); // error message

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, consumed) = result.unwrap();
        assert_eq!(error_packet.error_code, 1024);
        assert_eq!(error_packet.sql_state_marker, None);
        assert_eq!(error_packet.sql_state, None);
        assert_eq!(error_packet.error_message, "Access denied");
        assert_eq!(consumed, data.len());
    }

    #[test]
    fn test_parse_error_packet_empty_message() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code 1024

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, consumed) = result.unwrap();
        assert_eq!(error_packet.error_code, 1024);
        assert_eq!(error_packet.sql_state_marker, None);
        assert_eq!(error_packet.sql_state, None);
        assert_eq!(error_packet.error_message, "");
        assert_eq!(consumed, data.len());
    }

    #[test]
    fn test_parse_error_packet_not_error() {
        let data = vec![0x01]; // not an error packet
        let result = parse_error_packet(&data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_parse_error_packet_empty_data() {
        let data = vec![];
        let result = parse_error_packet(&data);
        assert!(matches!(result, Err(crate::error::Error::InvalidPacket(_))));
    }

    #[test]
    fn test_parse_error_packet_incomplete_error_code() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.push(0x00); // only 1 byte of error code
        let result = parse_error_packet(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_error_packet_incomplete_sql_state() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code
        data.push(b'#'); // SQL state marker
        data.extend_from_slice(b"HY00"); // incomplete SQL state (only 4 chars)
        let result = parse_error_packet(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(_))
        ));
    }

    #[test]
    fn test_parse_error_packet_max_error_code() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0xFF, 0xFF]); // max error code 65535
        data.extend_from_slice(b"Maximum error code");

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, _) = result.unwrap();
        assert_eq!(error_packet.error_code, 65535);
        assert_eq!(error_packet.error_message, "Maximum error code");
    }

    #[test]
    fn test_parse_error_packet_unicode_message() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code
        data.extend_from_slice("エラーメッセージ".as_bytes()); // Japanese error message

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, _) = result.unwrap();
        assert_eq!(error_packet.error_message, "エラーメッセージ");
    }

    #[test]
    fn test_parse_error_packet_sql_state_without_marker() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code
        data.extend_from_slice(b"HY000"); // SQL state without # marker
        data.extend_from_slice(b" Error message");

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, _) = result.unwrap();
        assert_eq!(error_packet.error_code, 1024);
        assert_eq!(error_packet.sql_state_marker, None);
        assert_eq!(error_packet.sql_state, None);
        assert_eq!(error_packet.error_message, "HY000 Error message");
    }

    #[test]
    fn test_parse_error_packet_long_message() {
        let mut data = vec![crate::constants::ERR_PACKET];
        data.extend_from_slice(&[0x00, 0x04]); // error code
        let long_message = "A".repeat(1000);
        data.extend_from_slice(long_message.as_bytes());

        let result = parse_error_packet(&data);
        assert!(result.is_ok());
        let (error_packet, _) = result.unwrap();
        assert_eq!(error_packet.error_message, long_message);
    }
}
