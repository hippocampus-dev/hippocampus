use tokio::io::AsyncWriteExt;

pub const VERSION: u32 = 2;

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

#[derive(serde::Serialize, serde::Deserialize, Debug, Clone, PartialEq)]
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
    /// Sent by the client ahead of one `Screen` per region; zero means it has no absolute pointer to move.
    Screens {
        count: u32,
    },
    Screen {
        x: u32,
        y: u32,
        width: u32,
        height: u32,
    },
    Clipboard {
        text: String,
    },
    /// Microphone samples from the server in the format `AUDIO_FORMAT` gives pw-record and pw-cat.
    Audio {
        samples: Bytes,
    },
}

pub const AUDIO_FORMAT: [&str; 7] = [
    "--raw",
    "--rate",
    "48000",
    "--channels",
    "1",
    "--format",
    "s16",
];

// serde encodes a Vec<u8> through serialize_seq, which serde_binary leaves unimplemented!().
#[derive(Debug, Clone, PartialEq)]
pub struct Bytes(pub Vec<u8>);

impl serde::Serialize for Bytes {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_bytes(&self.0)
    }
}

impl<'de> serde::Deserialize<'de> for Bytes {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct Visitor;

        impl serde::de::Visitor<'_> for Visitor {
            type Value = Bytes;

            fn expecting(&self, formatter: &mut std::fmt::Formatter) -> std::fmt::Result {
                formatter.write_str("bytes")
            }

            fn visit_byte_buf<E>(self, value: Vec<u8>) -> Result<Bytes, E> {
                Ok(Bytes(value))
            }
        }

        deserializer.deserialize_byte_buf(Visitor)
    }
}

impl Message {
    // serde_binary::de::from_slice reports neither the bytes it consumed nor a short read apart from other errors.
    pub fn encode(&self, buffer: &mut Vec<u8>) -> Result<(), error::Error> {
        let body = serde_binary::ser::to_vec(self)?;
        buffer.extend_from_slice(&u32::try_from(body.len())?.to_be_bytes());
        buffer.extend_from_slice(&body);
        Ok(())
    }

    pub fn parse(received: &mut Vec<u8>) -> Result<Vec<Message>, error::Error> {
        let mut messages = Vec::new();
        let mut consumed = 0;
        while let Some(length) = received.get(consumed..consumed + 4) {
            let end = consumed + 4 + u32::from_be_bytes(length.try_into()?) as usize;
            if received.len() < end {
                break;
            }
            messages.push(serde_binary::de::from_slice(&received[consumed + 4..end])?);
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
            Message::Screens { count: 1 },
            Message::Screen {
                x: 0,
                y: 0,
                width: 1920,
                height: 1080,
            },
            Message::Clipboard {
                text: "é".repeat(200),
            },
            Message::Audio {
                samples: Bytes(vec![0, 255, 128]),
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
        let tail = buffer.split_off(12);
        assert_eq!(Message::parse(&mut buffer).unwrap(), vec![Message::Enter]);
        assert_eq!(buffer.len(), 4);
        buffer.extend_from_slice(&tail);
        assert_eq!(
            Message::parse(&mut buffer).unwrap(),
            vec![Message::Motion { x: 3.0, y: 4.0 }]
        );
        assert!(buffer.is_empty());
    }
}
