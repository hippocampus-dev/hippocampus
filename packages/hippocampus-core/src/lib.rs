#![cfg_attr(all(test, nightly), feature(test))]

use std::io::Read;
use std::iter::FromIterator;

pub mod indexer;
pub mod lru;
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
pub struct PostingsList(Vec<Posting>);

impl std::ops::Deref for PostingsList {
    type Target = Vec<Posting>;

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl std::ops::DerefMut for PostingsList {
    fn deref_mut(&mut self) -> &mut Self::Target {
        &mut self.0
    }
}

const RICE_PARAMETER: u8 = 128;

impl PostingsList {
    fn new() -> Self {
        Self(Vec::new())
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

        PostingsList(hm.into_iter().map(|(k, v)| Posting::new(k, v)).collect())
    }

    fn intersection(&self, other: Self) -> Self {
        let mut postings_list = Vec::new();

        for self_posting in self {
            for other_posting in &other {
                match self_posting.document_uuid.cmp(&other_posting.document_uuid) {
                    std::cmp::Ordering::Less => break,
                    std::cmp::Ordering::Equal => {
                        let mut posting = self_posting.clone();
                        posting.positions.extend(other_posting.positions.clone());
                        postings_list.push(posting);
                        break;
                    }
                    std::cmp::Ordering::Greater => {}
                }
            }
        }

        PostingsList(postings_list)
    }

    fn difference(&self, other: Self) -> Self {
        let mut hm = rustc_hash::FxHashMap::default();
        for self_posting in self {
            hm.insert(self_posting.document_uuid, self_posting.positions.clone());
        }
        for other_posting in &other {
            hm.remove(&other_posting.document_uuid);
        }

        PostingsList(hm.into_iter().map(|(k, v)| Posting::new(k, v)).collect())
    }

    fn as_bytes(&self) -> Vec<u8> {
        let mut bytes = Vec::new();

        for postings in &self.0 {
            bytes.extend_from_slice(&postings.document_uuid.to_be_bytes());
            bytes.extend_from_slice(&postings.positions.len().to_be_bytes());
            bytes.extend_from_slice(&RICE_PARAMETER.to_be_bytes());
            let mut previous_position = 0;
            for position in &postings.positions {
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

        bytes
    }

    fn from_bytes(bytes: Vec<u8>) -> Self {
        let mut postings_list = Vec::new();

        let bytes_len = bytes.len();
        let mut cursor = std::io::Cursor::new(bytes);
        while cursor.position() < bytes_len as u64 {
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

            postings_list.push(Posting::new(
                document_uuid,
                std::collections::BTreeSet::from_iter(positions),
            ));
        }

        Self(postings_list)
    }
}

impl std::ops::Index<usize> for PostingsList {
    type Output = Posting;

    fn index(&self, index: usize) -> &Self::Output {
        &self.0[index]
    }
}

impl IntoIterator for PostingsList {
    type Item = Posting;
    type IntoIter = std::vec::IntoIter<Self::Item>;

    fn into_iter(self) -> Self::IntoIter {
        self.0.into_iter()
    }
}

impl<'a> IntoIterator for &'a PostingsList {
    type Item = &'a Posting;
    type IntoIter = std::slice::Iter<'a, Posting>;

    fn into_iter(self) -> Self::IntoIter {
        self.0.iter()
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
        let mut postings_list = PostingsList(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )])
        .union(PostingsList(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ]));
        let expect = PostingsList(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![1, 2])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ]);
        postings_list.sort();
        assert_eq!(postings_list, expect);
    }

    #[test]
    fn intersection() {
        let postings_list = PostingsList(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )])
        .intersection(PostingsList(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![2])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ]));
        let expect = PostingsList(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1, 2]),
        )]);
        assert_eq!(postings_list, expect);
    }

    #[test]
    fn difference() {
        let postings_list = PostingsList(vec![
            Posting::new(1, std::collections::BTreeSet::from_iter(vec![1])),
            Posting::new(2, std::collections::BTreeSet::from_iter(vec![1])),
        ])
        .difference(PostingsList(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![2]),
        )]));
        let expect = PostingsList(vec![Posting::new(
            2,
            std::collections::BTreeSet::from_iter(vec![1]),
        )]);
        assert_eq!(postings_list, expect);
    }

    #[test]
    fn as_bytes() {
        let postings_list = PostingsList(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )]);
        let expect: Vec<u8> = vec![
            0,
            0,
            0,
            0,
            0,
            0,
            0,
            1,
            0,
            0,
            0,
            0,
            0,
            0,
            0,
            1,
            RICE_PARAMETER,
            1,
            1,
        ];
        assert_eq!(postings_list.as_bytes(), expect);
    }

    #[test]
    fn from_bytes() {
        let bytes: Vec<u8> = vec![
            0,
            0,
            0,
            0,
            0,
            0,
            0,
            1,
            0,
            0,
            0,
            0,
            0,
            0,
            0,
            1,
            RICE_PARAMETER,
            1,
            1,
        ];
        let expect = PostingsList(vec![Posting::new(
            1,
            std::collections::BTreeSet::from_iter(vec![1]),
        )]);
        assert_eq!(PostingsList::from_bytes(bytes), expect);
    }
}
