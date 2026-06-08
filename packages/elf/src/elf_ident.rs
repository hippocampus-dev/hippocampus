/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L347-L350
#[allow(clippy::upper_case_acronyms)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Class {
    ELFCLASSNONE,
    ELFCLASS32,
    ELFCLASS64,
    ELFCLASSNUM,
    Unknown,
}

impl From<u8> for Class {
    fn from(value: u8) -> Self {
        match value {
            0 => Self::ELFCLASSNONE,
            1 => Self::ELFCLASS32,
            2 => Self::ELFCLASS64,
            3 => Self::ELFCLASSNUM,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L352-L354
#[allow(clippy::upper_case_acronyms)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Data {
    ELFDATANONE,
    ELFDATA2LSB,
    ELFDATA2MSB,
    Unknown,
}

impl From<u8> for Data {
    fn from(value: u8) -> Self {
        match value {
            0 => Self::ELFDATANONE,
            1 => Self::ELFDATA2LSB,
            2 => Self::ELFDATA2MSB,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L356-L358
#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Version {
    EV_NONE,
    EV_CURRENT,
    EV_NUM,
    Unknown,
}

impl From<u8> for Version {
    fn from(value: u8) -> Self {
        match value {
            0 => Self::EV_NONE,
            1 => Self::EV_CURRENT,
            2 => Self::EV_NUM,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L330-L338
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Ident {
    pub class: Class,
    pub data: Data,
    pub version: Version,
    pub osabi: u8,
    pub abi_version: u8,
}

pub fn parse(b: &[u8; 16]) -> Result<Ident, error::Error> {
    if b[0] != 0x7f || b[1] != b'E' || b[2] != b'L' || b[3] != b'F' {
        return Err(crate::ParseError.into());
    }
    let class = Class::from(b[4]);
    let data = Data::from(b[5]);
    let version = Version::from(b[6]);
    let osabi = b[7];
    let abi_version = b[8];
    let _pad: [u8; 7] = b[9..16].try_into()?;
    Ok(Ident {
        class,
        data,
        version,
        osabi,
        abi_version,
    })
}
