/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L124-L130
#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Type {
    STT_NOTYPE,
    STT_OBJECT,
    STT_FUNC,
    STT_SECTION,
    STT_FILE,
    STT_COMMON,
    STT_TLS,
    Unknown,
}

impl From<u8> for Type {
    fn from(value: u8) -> Self {
        match value {
            0 => Self::STT_NOTYPE,
            1 => Self::STT_OBJECT,
            2 => Self::STT_FUNC,
            3 => Self::STT_SECTION,
            4 => Self::STT_FILE,
            5 => Self::STT_COMMON,
            6 => Self::STT_TLS,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L120-L122
#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Bind {
    STB_LOCAL,
    STB_GLOBAL,
    STB_WEAK,
    Unknown,
}

impl From<u8> for Bind {
    fn from(value: u8) -> Self {
        match value {
            0 => Self::STB_LOCAL,
            1 => Self::STB_GLOBAL,
            2 => Self::STB_WEAK,
            _ => Self::Unknown,
        }
    }
}

#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Visibility {
    STV_DEFAULT,
    STV_INTERNAL,
    STV_HIDDEN,
    STV_PROTECTED,
    Unknown,
}

impl From<u8> for Visibility {
    fn from(value: u8) -> Self {
        match value {
            0 => Self::STV_DEFAULT,
            1 => Self::STV_INTERNAL,
            2 => Self::STV_HIDDEN,
            3 => Self::STV_PROTECTED,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L184-L191
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Symbol32 {
    pub name: u32,
    pub value: u32,
    pub size: u32,
    pub _type: Type,
    pub bind: Bind,
    pub visibility: Visibility,
    pub shndx: u16,
}

pub fn parse32(
    elf_header: &crate::elf_header::Header32,
    b: &[u8; 16],
) -> Result<Symbol32, error::Error> {
    let (name, value, size, shndx) = match elf_header.ident.data {
        crate::elf_ident::Data::ELFDATA2LSB => {
            let name = u32::from_le_bytes(b[0..4].try_into()?);
            let value = u32::from_le_bytes(b[4..8].try_into()?);
            let size = u32::from_le_bytes(b[8..12].try_into()?);
            let shndx = u16::from_le_bytes(b[14..16].try_into()?);
            (name, value, size, shndx)
        }
        crate::elf_ident::Data::ELFDATA2MSB => {
            let name = u32::from_be_bytes(b[0..4].try_into()?);
            let value = u32::from_be_bytes(b[4..8].try_into()?);
            let size = u32::from_be_bytes(b[8..12].try_into()?);
            let shndx = u16::from_be_bytes(b[14..16].try_into()?);
            (name, value, size, shndx)
        }
        _ => unimplemented!(),
    };
    let info = b[12];
    let _type = Type::from(info & 0xf);
    let bind = Bind::from(info >> 4);
    let other = b[13];
    let visibility = Visibility::from(other & 0x3);
    Ok(Symbol32 {
        name,
        value,
        size,
        _type,
        bind,
        visibility,
        shndx,
    })
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L193-L200
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Symbol64 {
    pub name: u32,
    pub _type: Type,
    pub bind: Bind,
    pub visibility: Visibility,
    pub shndx: u16,
    pub value: u64,
    pub size: u64,
}

pub fn parse64(
    elf_header: &crate::elf_header::Header64,
    b: &[u8; 24],
) -> Result<Symbol64, error::Error> {
    let (name, shndx, value, size) = match elf_header.ident.data {
        crate::elf_ident::Data::ELFDATA2LSB => {
            let name = u32::from_le_bytes(b[0..4].try_into()?);
            let shndx = u16::from_le_bytes(b[6..8].try_into()?);
            let value = u64::from_le_bytes(b[8..16].try_into()?);
            let size = u64::from_le_bytes(b[16..24].try_into()?);
            (name, shndx, value, size)
        }
        crate::elf_ident::Data::ELFDATA2MSB => {
            let name = u32::from_be_bytes(b[0..4].try_into()?);
            let shndx = u16::from_be_bytes(b[6..8].try_into()?);
            let value = u64::from_be_bytes(b[8..16].try_into()?);
            let size = u64::from_be_bytes(b[16..24].try_into()?);
            (name, shndx, value, size)
        }
        _ => unimplemented!(),
    };
    let info = b[4];
    let _type = Type::from(info & 0xf);
    let bind = Bind::from(info >> 4);
    let other = b[5];
    let visibility = Visibility::from(other & 0x3);
    Ok(Symbol64 {
        name,
        _type,
        bind,
        visibility,
        shndx,
        value,
        size,
    })
}

#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Symbol {
    Symbol32(Symbol32),
    Symbol64(Symbol64),
}
