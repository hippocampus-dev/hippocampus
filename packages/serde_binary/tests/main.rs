#[derive(serde::Serialize, serde::Deserialize, Debug, PartialEq)]
enum Message {
    Unit,
    Newtype(u32),
    Struct {
        flag: bool,
        small: u8,
        signed: i32,
        large: u64,
        single: f32,
        double: f64,
    },
}

#[test]
fn ok() -> Result<(), serde_binary::Error> {
    let messages = vec![
        Message::Unit,
        Message::Newtype(7),
        Message::Struct {
            flag: true,
            small: 255,
            signed: -120,
            large: u64::MAX,
            single: 1.5,
            double: -2.25,
        },
    ];
    for message in messages {
        let bytes = serde_binary::ser::to_vec(&message)?;
        assert_eq!(serde_binary::de::from_slice::<Message>(&bytes)?, message);
    }
    Ok(())
}

#[test]
fn layout() -> Result<(), serde_binary::Error> {
    assert_eq!(serde_binary::ser::to_vec(&Message::Unit)?, vec![0, 0, 0, 0]);
    assert_eq!(
        serde_binary::ser::to_vec(&Message::Newtype(7))?,
        vec![0, 0, 0, 1, 0, 0, 0, 7]
    );
    Ok(())
}

#[test]
fn invalid() {
    assert!(serde_binary::de::from_slice::<bool>(&[2]).is_err());
}
