//! A lightweight full-text search library with pluggable storage backends and tokenizers.
//!
//! # Main API
//!
//! - [`indexer::DocumentIndexer`]: Indexes documents with tokenization and inverted index creation
//! - [`searcher::DocumentSearcher`]: Executes queries with TF-IDF scoring and result highlighting
//! - [`storage`]: Pluggable storage backends (File, SQLite, Cassandra, GCS)
//! - [`tokenizer`]: Text tokenization (Lindera for Japanese/Korean, Whitespace)
//!
//! # Examples
//!
//! ```rust,ignore
//! // 1. Define schema
//! let mut schema = Schema::new();
//! schema.add_field(Field {
//!     name: "content".to_string(),
//!     field_type: FieldType::String(StringOption { indexeing: true }),
//! });
//!
//! // 2. Create storage backends
//! let document_storage = storage::file::File::new(path, hasher);
//! let token_storage = storage::file::File::new(path, hasher);
//!
//! // 3. Index documents
//! let tokenizer = tokenizer::lindera::Lindera::new()?;
//! let indexer = indexer::DocumentIndexer::new(document_storage, token_storage, tokenizer, schema);
//! indexer.index(documents).await?;
//!
//! // 4. Search
//! let searcher = searcher::DocumentSearcher::new(document_storage, token_storage, tokenizer, scorer, schema);
//! let results = searcher.search(&query, SearchOption::default()).await?;
//! ```

#![cfg_attr(all(test, nightly), feature(test))]

use std::io::Read;
use std::iter::FromIterator;

pub mod indexer;
pub mod scorer;
pub mod searcher;
pub mod storage;
pub mod tokenizer;
pub mod types;

#[derive(Clone, Debug, Default)]
pub struct Schema(pub std::collections::HashMap<String, Field>);

impl Schema {
    pub fn new() -> Self {
        Default::default()
    }

    pub fn add_field(&mut self, field: Field) {
        self.0.insert(field.name.clone(), field);
    }

    pub fn get_field<S>(&self, name: S) -> Option<&Field>
    where
        S: AsRef<str>,
    {
        self.0.get(name.as_ref())
    }
}

#[derive(Clone, Debug)]
pub struct Field {
    pub name: String,
    pub field_type: FieldType,
}

impl Field {
    fn is_indexed(&self) -> bool {
        match &self.field_type {
            FieldType::String(option) => option.indexeing,
        }
    }
}

#[derive(Clone, Debug)]
pub enum FieldType {
    String(StringOption),
}

#[derive(Clone, Debug)]
pub struct StringOption {
    pub indexeing: bool,
}

#[derive(Clone, Debug, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub enum Value {
    String(String),
}

type FieldValues = std::collections::BTreeMap<String, Value>;

impl std::fmt::Display for Value {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Value::String(string) => write!(f, "{string}"),
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct Document(pub FieldValues);

impl std::ops::Deref for Document {
    type Target = FieldValues;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl std::ops::DerefMut for Document {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Posting {
    document_uuid: u64,
    positions: std::collections::BTreeSet<u64>,
}

impl Posting {
    fn new(document_uuid: u64, positions: std::collections::BTreeSet<u64>) -> Self {
        Self {
            document_uuid,
            positions,
        }
    }
}

impl PartialOrd<Self> for Posting {
    fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for Posting {
    fn cmp(&self, other: &Self) -> std::cmp::Ordering {
        self.document_uuid.cmp(&other.document_uuid)
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PostingsList {
    postings: Vec<Posting>,
    skip_pointers: types::skip_pointer::SkipPointers,
}

impl std::ops::Deref for PostingsList {
    type Target = Vec<Posting>;

    fn deref(&self) -> &Self::Target {
        &self.postings
    }
}

impl std::ops::DerefMut for PostingsList {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.postings
    }
}

const RICE_PARAMETER: u8 = 128;

const SIZE_RATIO_THRESHOLD: usize = 10;

fn galloping_search<T, F>(items: &[T], target: u64, start: usize, extract_key: F) -> usize
where
    F: Fn(&T) -> u64,
{
    if start >= items.len() {
        return items.len();
    }

    let mut bound = 1;
    let mut low = start;

    while low + bound < items.len() && extract_key(&items[low + bound]) < target {
        low += bound;
        bound *= 2;
    }

    let high = std::cmp::min(low + bound, items.len());

    let result = items[low..high].binary_search_by(|item| extract_key(item).cmp(&target));
    match result {
        Ok(index) => low + index,
        Err(index) => low + index,
    }
}

impl PostingsList {
    fn new() -> Self {
        Self {
            postings: Vec::new(),
            skip_pointers: types::skip_pointer::SkipPointers::new(),
        }
    }

    fn from_postings(mut postings: Vec<Posting>) -> Self {
        postings.sort();
        let skip_pointers = types::skip_pointer::SkipPointers::build(&postings, |p| p.document_uuid);
        Self {
            postings,
            skip_pointers,
        }
    }

    fn union(&self, other: Self) -> Self {
        let mut hm = rustc_hash::FxHashMap::default();
        for self_posting in self {
            hm.insert(self_posting.document_uuid, self_posting.positions.clone());
        }
        for other_posting in &other {
            if let Some(positions) = hm.get_mut(&other_posting.document_uuid) {
                positions.extend(other_posting.positions.clone());
            } else {
                hm.insert(other_posting.document_uuid, other_posting.positions.clone());
            }
        }

        Self::from_postings(hm.into_iter().map(|(k, v)| Posting::new(k, v)).collect())
    }

    fn intersection(&self, other: Self) -> Self {
        let (smaller, larger) = if self.len() <= other.len() {
            (self, &other)
        } else {
            (&other, self)
        };

        if smaller.is_empty() || larger.is_empty() {
            return Self::new();
        }

        let size_ratio = larger.len() / std::cmp::max(smaller.len(), 1);
        let use_skip_pointers =
            size_ratio >= SIZE_RATIO_THRESHOLD && !larger.skip_pointers.is_empty();

        let mut postings_list = Vec::new();

        if use_skip_pointers {
            let mut j = 0;
            for small_posting in smaller.iter() {
                let target = small_posting.document_uuid;
                j = larger.skip_pointers.advance_to(target, j);
                j = galloping_search(&larger.postings, target, j, |p| p.document_uuid);

                if j < larger.len() && larger.postings[j].document_uuid == target {
                    let mut posting = small_posting.clone();
                    posting
                        .positions
                        .extend(larger.postings[j].positions.clone());
                    postings_list.push(posting);
                    j += 1;
                }
            }
        } else {
            let mut i = 0;
            let mut j = 0;
            while i < smaller.len() && j < larger.len() {
                match smaller.postings[i]
                    .document_uuid
                    .cmp(&larger.postings[j].document_uuid)
                {
                    std::cmp::Ordering::Less => i += 1,
                    std::cmp::Ordering::Greater => j += 1,
                    std::cmp::Ordering::Equal => {
                        let mut posting = smaller.postings[i].clone();
                        posting
                            .positions
                            .extend(larger.postings[j].positions.clone());
                        postings_list.push(posting);
                        i += 1;
                        j += 1;
                    }
                }
            }
        }

        Self::from_postings(postings_list)
    }

    fn difference(&self, other: Self) -> Self {
        let mut hm = rustc_hash::FxHashMap::default();
        for self_posting in self {
            hm.insert(self_posting.document_uuid, self_posting.positions.clone());
        }
        for other_posting in &other {
            hm.remove(&other_posting.document_uuid);
        }

        Self::from_postings(hm.into_iter().map(|(k, v)| Posting::new(k, v)).collect())
    }

    fn as_bytes(&self) -> Vec<u8> {
        let mut bytes = Vec::new();

        for posting in &self.postings {
            bytes.extend_from_slice(&posting.document_uuid.to_be_bytes());
            bytes.extend_from_slice(&posting.positions.len().to_be_bytes());
            bytes.extend_from_slice(&RICE_PARAMETER.to_be_bytes());
            let mut previous_position = 0;
            for position in &posting.positions {
                let gap = position - previous_position;
                let mut quotient = gap / RICE_PARAMETER as u64;
                let remainder = gap % RICE_PARAMETER as u64;
                while quotient > 0 {
                    bytes.push(0);
                    quotient -= 1;
                }
                bytes.push(1);
                bytes.push(remainder as u8);
                previous_position = *position;
            }
        }

        let skip_pointers_bytes = self.skip_pointers.as_bytes();
        let skip_pointers_len = skip_pointers_bytes.len();
        bytes.extend(skip_pointers_bytes);
        bytes.extend_from_slice(&skip_pointers_len.to_be_bytes());

        bytes
    }

    fn from_bytes(bytes: Vec<u8>) -> Self {
        let bytes_len = bytes.len();
        if bytes_len < std::mem::size_of::<usize>() {
            return Self::new();
        }

        let size_of_usize = std::mem::size_of::<usize>();
        let skip_pointers_size = usize::from_be_bytes(std::array::from_fn(|i| {
            bytes[bytes_len - size_of_usize + i]
        }));
        let postings_end = bytes_len - size_of_usize - skip_pointers_size;
        let skip_pointers =
            types::skip_pointer::SkipPointers::from_bytes(&bytes[postings_end..bytes_len - size_of_usize]);

        let mut postings = Vec::new();
        let mut cursor = std::io::Cursor::new(&bytes[..postings_end]);
        while cursor.position() < postings_end as u64 {
            let mut document_uuid_bytes = [0; 8];
            cursor.read_exact(&mut document_uuid_bytes).unwrap();
            let document_uuid = u64::from_be_bytes(document_uuid_bytes);

            let mut positions_size_bytes = [0; 8];
            cursor.read_exact(&mut positions_size_bytes).unwrap();
            let positions_size = usize::from_be_bytes(positions_size_bytes);

            let mut rice_parameter_bytes = [0; 1];
            cursor.read_exact(&mut rice_parameter_bytes).unwrap();
            let rice_parameter = u8::from_be_bytes(rice_parameter_bytes);

            let mut positions = Vec::new();
            let mut previous_position = 0;
            for _ in 0..positions_size {
                let mut gap = 0;
                loop {
                    let mut byte = [0; 1];
                    cursor.read_exact(&mut byte).unwrap();
                    if byte[0] == 1 {
                        break;
                    }
                    gap += rice_parameter as u64;
                }
                let mut remainder = [0; 1];
                cursor.read_exact(&mut remainder).unwrap();
                gap += remainder[0] as u64;
                let position = previous_position + gap;
                previous_position = position;
                positions.push(position);
            }

            postings.push(Posting::new(
                document_uuid,
                std::collections::BTreeSet::from_iter(positions),
            ));
        }

        Self {
            postings,
            skip_pointers,
        }
    }
}

impl std::ops::Index<usize> for PostingsList {
    type Output = Posting;

    fn index(&self, index: usize) -> &Self::Output {
        &self.postings[index]
    }
}

impl IntoIterator for PostingsList {
    type Item = Posting;
    type IntoIter = std::vec::IntoIter<Self::Item>;

    fn into_iter(self) -> Self::IntoIter {
        self.postings.into_iter()
    }
}

impl<'a> IntoIterator for &'a PostingsList {
    type Item = &'a Posting;
    type IntoIter = std::slice::Iter<'a, Posting>;

    fn into_iter(self) -> Self::IntoIter {
        self.postings.iter()
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct InvertedIndex {
    token: String,
    postings_list: crate::PostingsList,
}

impl InvertedIndex {
    fn new(token: String, postings_list: crate::PostingsList) -> Self {
        Self {
            token,
            postings_list,
        }
    }
}

#[cfg(test)]
mod test {
    use std::iter::FromIterator;

    use super::*;

    #[test]
    fn union() {
        let mut postings_list = PostingsList::from_postings(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )])
        .union(PostingsList::from_postings(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ]));
        let expect = PostingsList::from_postings(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![1, 2])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ]);
        postings_list.sort();
        assert_eq!(postings_list.postings, expect.postings);
    }

    #[test]
    fn intersection() {
        let postings_list = PostingsList::from_postings(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )])
        .intersection(PostingsList::from_postings(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ]));
        let expect = PostingsList::from_postings(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1, 2]),
        )]);
        assert_eq!(postings_list.postings, expect.postings);
    }

    #[test]
    fn intersection_with_skip_pointers() {
        let postings_list = PostingsList::from_postings(
            (0..10)
                .map(|i| Posting::new(i * 100, std::collections::BTreeSet::from_iter(vec![i])))
                .collect(),
        )
        .intersection(PostingsList::from_postings(
            (0..1000)
                .map(|i| Posting::new(i, std::collections::BTreeSet::from_iter(vec![i])))
                .collect(),
        ));
        let expect = PostingsList::from_postings(
            (0..10)
                .map(|i| {
                    Posting::new(
                        i * 100,
                        std::collections::BTreeSet::from_iter(vec![i, i * 100]),
                    )
                })
                .collect(),
        );
        assert_eq!(postings_list.postings, expect.postings);
    }

    #[test]
    fn difference() {
        let postings_list = PostingsList::from_postings(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ])
        .difference(PostingsList::from_postings(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![2]),
        )]));
        let expect = PostingsList::from_postings(vec![Posting::new(
            2,
            std::collections::BTreeSet::from_iter(vec![1]),
        )]);
        assert_eq!(postings_list.postings, expect.postings);
    }

    #[test]
    fn as_bytes() {
        let postings_list = PostingsList::from_postings(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )]);
        #[rustfmt::skip]
        let expected: Vec<u8> = vec![
            // postings
            0, 0, 0, 0, 0, 0, 0, 1,  // document_uuid = 1
            0, 0, 0, 0, 0, 0, 0, 1,  // positions.len() = 1
            128,                     // rice_parameter
            1, 1,                    // position 1: end marker + remainder
            // skip_pointers
            0, 0, 0, 0,              // count = 0
            // skip_pointers_byte_size
            0, 0, 0, 0, 0, 0, 0, 4,  // 4 bytes
        ];
        assert_eq!(postings_list.as_bytes(), expected);
    }

    #[test]
    fn galloping_search_found() {
        let items: Vec<u64> = (0..1000).step_by(2).collect();

        let index = galloping_search(&items, 500, 0, |item| *item);
        assert_eq!(items[index], 500);

        let index = galloping_search(&items, 501, 0, |item| *item);
        assert_eq!(items[index], 502);

        let index = galloping_search(&items, 100, 100, |item| *item);
        assert_eq!(items[index], 200);
    }

    #[test]
    fn galloping_search_not_found() {
        let items: Vec<u64> = (0..100).step_by(2).collect();

        let index = galloping_search(&items, 1000, 0, |item| *item);
        assert_eq!(index, items.len());
    }

    #[test]
    fn from_bytes() {
        #[rustfmt::skip]
        let bytes: Vec<u8> = vec![
            // postings
            0, 0, 0, 0, 0, 0, 0, 1,  // document_uuid = 1
            0, 0, 0, 0, 0, 0, 0, 1,  // positions.len() = 1
            128,                     // rice_parameter
            1, 1,                    // position 1: end marker + remainder
            // skip_pointers
            0, 0, 0, 0,              // count = 0
            // skip_pointers_byte_size
            0, 0, 0, 0, 0, 0, 0, 4,  // 4 bytes
        ];
        let expected = PostingsList::from_postings(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )]);
        assert_eq!(PostingsList::from_bytes(bytes).postings, expected.postings);
    }
}
