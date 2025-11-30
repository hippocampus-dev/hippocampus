const DEFAULT_BLOCK_SIZE: usize = 128;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SkipPointer {
    pub document_uuid: u64,
    pub posting_index: u32,
}

impl SkipPointer {
    pub fn new(document_uuid: u64, posting_index: u32) -> Self {
        Self {
            document_uuid,
            posting_index,
        }
    }

    pub fn as_bytes(&self) -> Vec<u8> {
        let size = std::mem::size_of::<u64>() + std::mem::size_of::<u32>();
        let mut bytes = Vec::with_capacity(size);
        bytes.extend_from_slice(&self.document_uuid.to_be_bytes());
        bytes.extend_from_slice(&self.posting_index.to_be_bytes());
        bytes
    }

    pub fn from_bytes(bytes: &[u8]) -> Self {
        let size_of_u64 = std::mem::size_of::<u64>();
        let size_of_u32 = std::mem::size_of::<u32>();
        let document_uuid = u64::from_be_bytes(bytes[0..size_of_u64].try_into().unwrap());
        let posting_index =
            u32::from_be_bytes(bytes[size_of_u64..size_of_u64 + size_of_u32].try_into().unwrap());
        Self {
            document_uuid,
            posting_index,
        }
    }
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct SkipPointers(Vec<SkipPointer>);

impl SkipPointers {
    pub fn new() -> Self {
        Self(Vec::new())
    }

    pub fn build<T, F>(items: &[T], extract_document_uuid: F) -> Self
    where
        F: Fn(&T) -> u64,
    {
        Self::build_with_block_size(items, extract_document_uuid, DEFAULT_BLOCK_SIZE)
    }

    pub fn build_with_block_size<T, F>(
        items: &[T],
        extract_document_uuid: F,
        block_size: usize,
    ) -> Self
    where
        F: Fn(&T) -> u64,
    {
        if items.len() < block_size * 2 {
            return Self::new();
        }

        let mut skip_pointers = Vec::new();
        let mut index = block_size;
        while index < items.len() {
            skip_pointers.push(SkipPointer::new(
                extract_document_uuid(&items[index]),
                index as u32,
            ));
            index += block_size;
        }

        Self(skip_pointers)
    }

    pub fn len(&self) -> usize {
        self.0.len()
    }

    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    pub fn advance_to(&self, target: u64, current_index: usize) -> usize {
        if self.0.is_empty() {
            return current_index;
        }

        let mut low = 0;
        let mut high = self.0.len();

        while low < high {
            let mid = low + (high - low) / 2;
            if self.0[mid].document_uuid < target {
                low = mid + 1;
            } else {
                high = mid;
            }
        }

        if low > 0 {
            let skip_index = self.0[low - 1].posting_index as usize;
            if skip_index > current_index {
                return skip_index;
            }
        }

        current_index
    }

    pub fn as_bytes(&self) -> Vec<u8> {
        let size_of_u32 = std::mem::size_of::<u32>();
        let size_of_skip_pointer = std::mem::size_of::<u64>() + size_of_u32;
        let mut bytes = Vec::with_capacity(size_of_u32 + self.0.len() * size_of_skip_pointer);
        bytes.extend_from_slice(&(self.0.len() as u32).to_be_bytes());
        for skip_pointer in &self.0 {
            bytes.extend(skip_pointer.as_bytes());
        }
        bytes
    }

    pub fn from_bytes(bytes: &[u8]) -> Self {
        let size_of_u32 = std::mem::size_of::<u32>();
        let size_of_skip_pointer = std::mem::size_of::<u64>() + size_of_u32;

        if bytes.len() < size_of_u32 {
            return Self::new();
        }

        let count = u32::from_be_bytes(std::array::from_fn(|i| bytes[i])) as usize;
        let mut skip_pointers = Vec::with_capacity(count);
        let mut offset = size_of_u32;

        for _ in 0..count {
            if offset + size_of_skip_pointer > bytes.len() {
                break;
            }
            skip_pointers.push(SkipPointer::from_bytes(
                &bytes[offset..offset + size_of_skip_pointer],
            ));
            offset += size_of_skip_pointer;
        }

        Self(skip_pointers)
    }
}

pub fn galloping_search<T, F>(items: &[T], target: u64, start: usize, extract_key: F) -> usize
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_skip_pointer_serialization() {
        let skip_pointer = SkipPointer::new(12345, 100);
        let bytes = skip_pointer.as_bytes();
        let deserialized = SkipPointer::from_bytes(&bytes);
        assert_eq!(skip_pointer, deserialized);
    }

    #[test]
    fn test_skip_pointers_build() {
        let items: Vec<u64> = (0..1000).collect();
        let skip_pointers = SkipPointers::build(&items, |item| *item);

        assert!(!skip_pointers.is_empty());
        assert_eq!(skip_pointers.len(), 7);

        assert_eq!(skip_pointers.0[0].document_uuid, 128);
        assert_eq!(skip_pointers.0[0].posting_index, 128);
        assert_eq!(skip_pointers.0[1].document_uuid, 256);
        assert_eq!(skip_pointers.0[1].posting_index, 256);
    }

    #[test]
    fn test_skip_pointers_build_small_list() {
        let items: Vec<u64> = (0..200).collect();
        let skip_pointers = SkipPointers::build(&items, |item| *item);
        assert!(skip_pointers.is_empty());
    }

    #[test]
    fn test_skip_pointers_serialization() {
        let items: Vec<u64> = (0..1000).collect();
        let skip_pointers = SkipPointers::build(&items, |item| *item);
        let bytes = skip_pointers.as_bytes();
        let deserialized = SkipPointers::from_bytes(&bytes);

        assert_eq!(skip_pointers, deserialized);
    }

    #[test]
    fn test_advance_to() {
        let items: Vec<u64> = (0..1000).step_by(2).collect();
        let skip_pointers = SkipPointers::build(&items, |item| *item);

        let new_index = skip_pointers.advance_to(300, 0);
        assert!(new_index >= 128);
        assert!(items[new_index] <= 300);

        let new_index = skip_pointers.advance_to(50, 0);
        assert_eq!(new_index, 0);
    }

    #[test]
    fn test_galloping_search() {
        let items: Vec<u64> = (0..1000).step_by(2).collect();

        let index = galloping_search(&items, 500, 0, |item| *item);
        assert_eq!(items[index], 500);

        let index = galloping_search(&items, 501, 0, |item| *item);
        assert_eq!(items[index], 502);

        let index = galloping_search(&items, 100, 100, |item| *item);
        assert_eq!(items[index], 200);
    }

    #[test]
    fn test_galloping_search_not_found() {
        let items: Vec<u64> = (0..100).step_by(2).collect();

        let index = galloping_search(&items, 1000, 0, |item| *item);
        assert_eq!(index, items.len());
    }

    #[test]
    fn test_empty_skip_pointers() {
        let skip_pointers = SkipPointers::new();
        assert!(skip_pointers.is_empty());
        assert_eq!(skip_pointers.advance_to(100, 0), 0);

        let bytes = skip_pointers.as_bytes();
        let deserialized = SkipPointers::from_bytes(&bytes);
        assert!(deserialized.is_empty());
    }
}
