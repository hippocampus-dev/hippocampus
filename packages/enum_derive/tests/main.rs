#[test]
fn len() {
    #[allow(dead_code)]
    #[derive(enum_derive::EnumLen)]
    enum E {
        A,
        B,
        C,
        D,
    }

    assert_eq!(E::len(), 4)
}

#[test]
fn iter_to_string() {
    #[allow(dead_code)]
    #[derive(enum_derive::EnumIter, enum_derive::EnumToString)]
    enum E {
        A,
        B,
        C,
        D,
    }
    for (i, e) in E::iter().enumerate() {
        match i {
            0 => {
                assert_eq!(e.to_string(), "A".to_string());
            }
            1 => {
                assert_eq!(e.to_string(), "B".to_string());
            }
            2 => {
                assert_eq!(e.to_string(), "C".to_string());
            }
            3 => {
                assert_eq!(e.to_string(), "D".to_string());
            }
            _ => {}
        }
    }
}
