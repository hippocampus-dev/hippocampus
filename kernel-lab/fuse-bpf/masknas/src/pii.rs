const MASK: u8 = b'*';

fn pii_regex() -> &'static regex::bytes::Regex {
    static REGEX: std::sync::OnceLock<regex::bytes::Regex> = std::sync::OnceLock::new();
    REGEX.get_or_init(|| {
        regex::bytes::Regex::new(concat!(
            r"\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}",
            r"|3[47]\d{2}[- ]?\d{6}[- ]?\d{5}",
            r"|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}",
            r"|\+81[-0-9]{9,}",
            r"|0\d{1,4}-?\d{1,4}-?\d{3,4}",
        ))
        .expect("static PII regex must compile")
    })
}

pub fn mask(data: &mut [u8]) -> usize {
    let spans: Vec<(usize, usize)> = pii_regex()
        .find_iter(data)
        .map(|matched| (matched.start(), matched.end()))
        .collect();

    for (start, end) in &spans {
        for byte in &mut data[*start..*end] {
            *byte = MASK;
        }
    }
    spans.len()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn masked(input: &str) -> String {
        let mut bytes = input.as_bytes().to_vec();
        mask(&mut bytes);
        String::from_utf8(bytes).expect("masking must not change byte boundaries")
    }

    fn expect_mask(input: &str, token: &str) -> String {
        input.replace(token, &"*".repeat(token.len()))
    }

    #[test]
    fn preserves_length() {
        for input in [
            "taro@example.com",
            "call 090-1234-5678 now",
            "card 4111 1111 1111 1111 end",
            "plain text, nothing here",
        ] {
            assert_eq!(
                masked(input).len(),
                input.len(),
                "length changed for {input:?}"
            );
        }
    }

    #[test]
    fn masks_email_fully() {
        let input = "taro@example.com";
        assert_eq!(masked(input), expect_mask(input, "taro@example.com"));
        assert!(masked(input).bytes().all(|byte| byte == b'*'));
    }

    #[test]
    fn masks_phone_and_keeps_surroundings() {
        let input = "tel: 03-1234-5678.";
        assert_eq!(masked(input), expect_mask(input, "03-1234-5678"));
    }

    #[test]
    fn masks_credit_card() {
        let input = "4111-1111-1111-1111";
        assert_eq!(masked(input), expect_mask(input, "4111-1111-1111-1111"));
    }

    #[test]
    fn masks_amex_card_fully() {
        let input = "card 340000000000009 on file";
        assert_eq!(masked(input), expect_mask(input, "340000000000009"));
    }

    #[test]
    fn masks_freephone() {
        let input = "hotline is 0120-000-000.";
        assert_eq!(masked(input), expect_mask(input, "0120-000-000"));
    }

    #[test]
    fn leaves_non_pii_untouched() {
        let clean = "the quick brown fox";
        assert_eq!(masked(clean), clean);
        assert_eq!(mask(&mut clean.as_bytes().to_vec()), 0);
    }

    #[test]
    fn masks_multiple_spans() {
        let input = "a@b.com and c@d.net";
        let out = masked(input);
        assert_eq!(out.len(), input.len());
        assert_eq!(mask(&mut input.as_bytes().to_vec()), 2);
    }
}
