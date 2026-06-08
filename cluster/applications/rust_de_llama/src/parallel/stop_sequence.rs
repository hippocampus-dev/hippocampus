//! Stop sequence detection utilities.
//!
//! This module provides llama.cpp's stop sequence detection algorithm.

/// Find partial stop sequence at the end of text.
///
/// This function detects if the text ends with a partial match of the stop pattern.
/// For example, if the stop pattern is `"</s>"` and the text ends with `"<"` or `"</"`,
/// this will return the position where the partial match starts.
///
/// Returns the position where the partial match starts, or None if no match is found.
///
/// # Algorithm
///
/// Based on llama.cpp's `string_find_partial_stop`:
/// 1. Get the last character of the text
/// 2. Search backwards through the stop string for that character
/// 3. If found, check if the text ends with the substring up to that position
/// 4. Return the position where the match starts
///
/// This is more efficient than checking all possible prefixes.
pub fn find_partial_stop(text: &str, stop: &str) -> Option<usize> {
    if text.is_empty() || stop.is_empty() {
        return None;
    }

    let text_last_char = text.chars().last()?;
    let stop_chars: Vec<char> = stop.chars().collect();

    // Search backwards through the stop string for the last character of text
    for char_index in (0..stop_chars.len()).rev() {
        if stop_chars[char_index] == text_last_char {
            let current_partial: String = stop_chars[..=char_index].iter().collect();
            if text.ends_with(&current_partial) {
                return Some(text.len() - current_partial.len());
            }
        }
    }

    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_find_partial_stop_exact_match() {
        let text = "Hello world</s>";
        let stop = "</s>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, Some(11));
    }

    #[test]
    fn test_find_partial_stop_partial_match() {
        let text = "Hello world<";
        let stop = "</s>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, Some(11));
    }

    #[test]
    fn test_find_partial_stop_longer_partial() {
        let text = "Hello world</";
        let stop = "</s>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, Some(11));
    }

    #[test]
    fn test_find_partial_stop_no_match() {
        let text = "Hello world";
        let stop = "</s>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, None);
    }

    #[test]
    fn test_find_partial_stop_empty_text() {
        let text = "";
        let stop = "</s>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, None);
    }

    #[test]
    fn test_find_partial_stop_empty_stop() {
        let text = "Hello world";
        let stop = "";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, None);
    }

    #[test]
    fn test_find_partial_stop_single_char() {
        let text = "Hello <";
        let stop = "<|endoftext|>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, Some(6));
    }

    #[test]
    fn test_find_partial_stop_multiple_chars() {
        let text = "Hello <|end";
        let stop = "<|endoftext|>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, Some(6));
    }

    #[test]
    fn test_find_partial_stop_no_partial_in_middle() {
        let text = "Hello < world";
        let stop = "</s>";
        let result = find_partial_stop(text, stop);
        assert_eq!(result, None);
    }
}
