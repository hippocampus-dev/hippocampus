//! Prompt-lookup (n-gram) draft proposal for speculative decoding.
//!
//! Tracks the token history of a single sequence (prompt + generated) and, for
//! the current suffix n-gram, proposes the tokens that followed the same n-gram
//! earlier in the history. No draft model is involved: the drafts are copied
//! from the sequence itself, which is why this only pays off on extractive,
//! summarization, and code workloads where the output echoes the input.

/// Per-sequence n-gram lookup index.
pub(crate) struct NgramLookup {
    ngram: usize,
    max_draft: usize,
    history: Vec<i32>,
    /// Maps an n-gram to the position immediately after its most recent prior
    /// occurrence, so a repeated n-gram proposes the tokens that followed it.
    index: std::collections::HashMap<Box<[i32]>, usize>,
}

impl NgramLookup {
    pub fn new(ngram: usize, max_draft: usize) -> Self {
        Self {
            ngram: ngram.max(1),
            max_draft,
            history: Vec::new(),
            index: std::collections::HashMap::new(),
        }
    }

    /// Append one committed token, recording the n-gram that ends at the
    /// previous last token so future suffixes can discover it without matching
    /// the suffix currently being queried.
    pub fn push(&mut self, token: i32) {
        if self.history.len() >= self.ngram {
            let start = self.history.len() - self.ngram;
            let key: Box<[i32]> = self.history[start..].into();
            // Next position after this n-gram occurrence is the slot the new
            // token is about to occupy.
            self.index.insert(key, self.history.len());
        }
        self.history.push(token);
    }

    pub fn extend(&mut self, tokens: &[i32]) {
        for &token in tokens {
            self.push(token);
        }
    }

    /// Propose up to `max_draft` continuation tokens for the current suffix
    /// n-gram, or an empty slice when the suffix has no prior occurrence.
    pub fn propose(&self) -> Vec<i32> {
        if self.max_draft == 0 || self.history.len() < self.ngram {
            return Vec::new();
        }
        let start = self.history.len() - self.ngram;
        let key = &self.history[start..];
        match self.index.get(key) {
            Some(&next) if next < self.history.len() => {
                let end = (next + self.max_draft).min(self.history.len());
                self.history[next..end].to_vec()
            }
            _ => Vec::new(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_no_proposal_before_ngram_filled() {
        let mut lookup = NgramLookup::new(2, 4);
        lookup.push(1);
        assert!(lookup.propose().is_empty());
    }

    #[test]
    fn test_no_proposal_without_repeat() {
        let mut lookup = NgramLookup::new(2, 4);
        lookup.extend(&[1, 2, 3, 4]);
        assert!(lookup.propose().is_empty());
    }

    #[test]
    fn test_proposes_continuation_of_repeated_ngram() {
        // History: 1 2 3 4 ... 1 2  -> suffix "1 2" occurred before, followed by 3 4.
        let mut lookup = NgramLookup::new(2, 4);
        lookup.extend(&[1, 2, 3, 4, 9, 1, 2]);
        assert_eq!(lookup.propose(), vec![3, 4, 9, 1]);
    }

    #[test]
    fn test_proposal_capped_by_max_draft() {
        let mut lookup = NgramLookup::new(2, 2);
        lookup.extend(&[1, 2, 3, 4, 9, 1, 2]);
        assert_eq!(lookup.propose(), vec![3, 4]);
    }

    #[test]
    fn test_uses_most_recent_prior_occurrence() {
        // "1 2" occurs followed by 3, then later followed by 7; the most recent
        // prior occurrence (…1 2 7…) wins for the trailing suffix.
        let mut lookup = NgramLookup::new(2, 3);
        lookup.extend(&[1, 2, 3, 5, 1, 2, 7, 8, 1, 2]);
        assert_eq!(lookup.propose(), vec![7, 8, 1]);
    }

    #[test]
    fn test_max_draft_zero_disables() {
        let mut lookup = NgramLookup::new(2, 0);
        lookup.extend(&[1, 2, 3, 1, 2]);
        assert!(lookup.propose().is_empty());
    }
}
