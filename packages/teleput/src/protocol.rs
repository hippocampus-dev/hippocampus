use tokio::io::AsyncWriteExt;

pub const VERSION: u32 = 1;

pub const SERVER_NAME: &str = "teleput";

// Without keepalive a peer that vanishes without a FIN is noticed only after the kernel's tcp_retries2 retransmissions.
pub fn configure(stream: &tokio::net::TcpStream) -> Result<(), error::Error> {
    stream.set_nodelay(true)?;
    rustix::net::sockopt::set_socket_keepalive(stream, true)?;
    rustix::net::sockopt::set_tcp_keepidle(stream, std::time::Duration::from_secs(5))?;
    rustix::net::sockopt::set_tcp_keepintvl(stream, std::time::Duration::from_secs(1))?;
    rustix::net::sockopt::set_tcp_keepcnt(stream, 3)?;
    rustix::net::sockopt::set_tcp_user_timeout(stream, 3000)?;
    Ok(())
}

pub async fn transmit<W: tokio::io::AsyncWrite + Unpin>(
    writer: &mut W,
    buffer: &[u8],
) -> Result<(), error::Error> {
    writer.write_all(buffer).await?;
    writer.flush().await?;
    Ok(())
}

#[derive(serde::Serialize, serde::Deserialize, Debug, Clone, Copy, PartialEq)]
pub enum Message {
    Hello {
        version: u32,
    },
    Enter,
    Leave,
    /// Absolute position in the client's logical pixels.
    Motion {
        x: f32,
        y: f32,
    },
    Button {
        button: u32,
        pressed: bool,
    },
    Scroll {
        dx: i32,
        dy: i32,
    },
    Key {
        key: u32,
        pressed: bool,
    },
    /// Sent by the client; a zero-sized area means it has no absolute pointer to move.
    Screen {
        x: u32,
        y: u32,
        width: u32,
        height: u32,
    },
}

impl Message {
    // serde_binary::de::from_slice reports neither the bytes it consumed nor a short read apart from other errors.
    pub fn encode(&self, buffer: &mut Vec<u8>) -> Result<(), error::Error> {
        let body = serde_binary::ser::to_vec(self)?;
        buffer.push(u8::try_from(body.len())?);
        buffer.extend_from_slice(&body);
        Ok(())
    }

    pub fn parse(received: &mut Vec<u8>) -> Result<Vec<Message>, error::Error> {
        let mut messages = Vec::new();
        let mut consumed = 0;
        while let Some(&length) = received.get(consumed) {
            let end = consumed + 1 + length as usize;
            if received.len() < end {
                break;
            }
            messages.push(serde_binary::de::from_slice(&received[consumed + 1..end])?);
            consumed = end;
        }
        received.drain(..consumed);
        Ok(messages)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_round_trip() {
        let messages = vec![
            Message::Hello { version: VERSION },
            Message::Enter,
            Message::Leave,
            Message::Motion { x: 1.5, y: 2.25 },
            Message::Button {
                button: 0x110,
                pressed: true,
            },
            Message::Scroll { dx: 0, dy: -120 },
            Message::Key {
                key: 30,
                pressed: false,
            },
            Message::Screen {
                x: 0,
                y: 0,
                width: 1920,
                height: 1080,
            },
        ];
        let mut buffer = Vec::new();
        for message in &messages {
            message.encode(&mut buffer).unwrap();
        }
        assert_eq!(Message::parse(&mut buffer).unwrap(), messages);
        assert!(buffer.is_empty());
    }

    #[test]
    fn test_partial() {
        let mut buffer = Vec::new();
        Message::Enter.encode(&mut buffer).unwrap();
        Message::Motion { x: 3.0, y: 4.0 }
            .encode(&mut buffer)
            .unwrap();
        let tail = buffer.split_off(10);
        assert_eq!(Message::parse(&mut buffer).unwrap(), vec![Message::Enter]);
        assert_eq!(buffer.len(), 5);
        buffer.extend_from_slice(&tail);
        assert_eq!(
            Message::parse(&mut buffer).unwrap(),
            vec![Message::Motion { x: 3.0, y: 4.0 }]
        );
        assert!(buffer.is_empty());
    }
}
