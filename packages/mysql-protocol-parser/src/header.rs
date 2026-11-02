//! MySQL Protocol Packet Header Parsing
//!
//! ## Protocol Documentation
//! - MySQL Packet Structure: <https://dev.mysql.com/doc/internals/en/mysql-packet.html>
//! - Protocol Basics: <https://dev.mysql.com/doc/internals/en/describing-packets.html>

pub fn parse_header(
    data: &[u8],
) -> Result<(crate::types::PacketHeader, usize), crate::error::Error> {
    if data.len() < 4 {
        return Err(crate::error::Error::IncompletePacket(4 - data.len()));
    }

    let payload_length = u32::from_le_bytes([data[0], data[1], data[2], 0]);

    if payload_length > crate::constants::MAX_PACKET_SIZE as u32 {
        return Err(crate::error::Error::InvalidPacket(format!(
            "Packet size {} exceeds maximum {}",
            payload_length,
            crate::constants::MAX_PACKET_SIZE
        )));
    }

    let sequence_id = data[3];

    Ok((
        crate::types::PacketHeader {
            payload_length,
            sequence_id,
        },
        4,
    ))
}

pub fn read_packet(data: &[u8]) -> Result<(&[u8], usize), crate::error::Error> {
    let (header, header_size) = parse_header(data)?;
    let total_size = header_size + header.payload_length as usize;

    if data.len() < total_size {
        return Err(crate::error::Error::IncompletePacket(
            total_size - data.len(),
        ));
    }

    let payload = &data[header_size..total_size];
    Ok((payload, total_size))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_header_success() {
        let data = vec![0x05, 0x00, 0x00, 0x01];
        let result = parse_header(&data);
        assert!(result.is_ok());
        let (header, consumed) = result.unwrap();
        assert_eq!(header.payload_length, 5);
        assert_eq!(header.sequence_id, 1);
        assert_eq!(consumed, 4);
    }

    #[test]
    fn test_parse_header_max_packet_size() {
        let data = vec![0xFF, 0xFF, 0xFF, 0x00];
        let result = parse_header(&data);
        assert!(result.is_ok());
        let (header, consumed) = result.unwrap();
        assert_eq!(header.payload_length, 0xFFFFFF);
        assert_eq!(header.sequence_id, 0);
        assert_eq!(consumed, 4);
    }

    #[test]
    fn test_parse_header_incomplete_packet() {
        let data = vec![0x05, 0x00];
        let result = parse_header(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(2))
        ));
    }

    #[test]
    fn test_parse_header_empty_data() {
        let data = vec![];
        let result = parse_header(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(4))
        ));
    }

    #[test]
    fn test_read_packet_success() {
        let data = vec![0x05, 0x00, 0x00, 0x01, 0x48, 0x65, 0x6C, 0x6C, 0x6F];
        let result = read_packet(&data);
        assert!(result.is_ok());
        let (payload, total_size) = result.unwrap();
        assert_eq!(payload, b"Hello");
        assert_eq!(total_size, 9);
    }

    #[test]
    fn test_read_packet_incomplete_payload() {
        let data = vec![0x05, 0x00, 0x00, 0x01, 0x48, 0x65];
        let result = read_packet(&data);
        assert!(matches!(
            result,
            Err(crate::error::Error::IncompletePacket(3))
        ));
    }

    #[test]
    fn test_read_packet_empty_payload() {
        let data = vec![0x00, 0x00, 0x00, 0x01];
        let result = read_packet(&data);
        assert!(result.is_ok());
        let (payload, total_size) = result.unwrap();
        assert_eq!(payload.len(), 0);
        assert_eq!(total_size, 4);
    }

    #[test]
    fn test_read_packet_large_payload() {
        let mut data = vec![0x00, 0x01, 0x00, 0x00];
        data.extend(vec![0xAA; 256]);
        let result = read_packet(&data);
        assert!(result.is_ok());
        let (payload, total_size) = result.unwrap();
        assert_eq!(payload.len(), 256);
        assert_eq!(total_size, 260);
    }

    #[test]
    fn test_parse_header_with_different_sequence_ids() {
        for seq_id in 0u8..=255 {
            let data = vec![0x01, 0x00, 0x00, seq_id];
            let result = parse_header(&data);
            assert!(result.is_ok());
            let (header, _) = result.unwrap();
            assert_eq!(header.sequence_id, seq_id);
        }
    }
}
