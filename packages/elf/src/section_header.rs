/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L268-L284
#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Type {
    SHT_NULL,
    SHT_PROGBITS,
    SHT_SYMTAB,
    SHT_STRTAB,
    SHT_RELA,
    SHT_HASH,
    SHT_DYNAMIC,
    SHT_NOTE,
    SHT_NOBITS,
    SHT_REL,
    SHT_SHLIB,
    SHT_DYNSYM,
    SHT_INIT_ARRAY,
    SHT_FINI_ARRAY,
    SHT_PREINIT_ARRAY,
    SHT_GROUP,
    SHT_SYMTAB_SHNDX,
    SHT_LOOS,
    SHT_HIOS,
    SHT_LOPROC,
    SHT_HIPROC,
    SHT_LOUSER,
    SHT_HIUSER,
    Unknown,
}

impl From<u32> for Type {
    fn from(value: u32) -> Self {
        match value {
            0 => Self::SHT_NULL,
            1 => Self::SHT_PROGBITS,
            2 => Self::SHT_SYMTAB,
            3 => Self::SHT_STRTAB,
            4 => Self::SHT_RELA,
            5 => Self::SHT_HASH,
            6 => Self::SHT_DYNAMIC,
            7 => Self::SHT_NOTE,
            8 => Self::SHT_NOBITS,
            9 => Self::SHT_REL,
            10 => Self::SHT_SHLIB,
            11 => Self::SHT_DYNSYM,
            14 => Self::SHT_INIT_ARRAY,
            15 => Self::SHT_FINI_ARRAY,
            16 => Self::SHT_PREINIT_ARRAY,
            17 => Self::SHT_GROUP,
            18 => Self::SHT_SYMTAB_SHNDX,
            0x60000000 => Self::SHT_LOOS,
            0x6fffffff => Self::SHT_HIOS,
            0x70000000 => Self::SHT_LOPROC,
            0x7fffffff => Self::SHT_HIPROC,
            0x80000000 => Self::SHT_LOUSER,
            0xffffffff => Self::SHT_HIUSER,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L287-L292
#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Flags {
    SHF_WRITE,
    SHF_ALLOC,
    SHF_EXECINSTR,
    SHF_RELA_LIVEPATCH,
    SHF_RO_AFTER_INIT,
    SHF_MASKPROC,
    Unknown,
}

impl From<u32> for Flags {
    fn from(value: u32) -> Self {
        match value {
            0x1 => Self::SHF_WRITE,
            0x2 => Self::SHF_ALLOC,
            0x4 => Self::SHF_EXECINSTR,
            0x00100000 => Self::SHF_RELA_LIVEPATCH,
            0x00200000 => Self::SHF_RO_AFTER_INIT,
            0xf0000000 => Self::SHF_MASKPROC,
            _ => Self::Unknown,
        }
    }
}

impl From<u64> for Flags {
    fn from(value: u64) -> Self {
        match value {
            0x1 => Self::SHF_WRITE,
            0x2 => Self::SHF_ALLOC,
            0x4 => Self::SHF_EXECINSTR,
            0x00100000 => Self::SHF_RELA_LIVEPATCH,
            0x00200000 => Self::SHF_RO_AFTER_INIT,
            0xf0000000 => Self::SHF_MASKPROC,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L304-L315
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Header32 {
    pub name: u32,
    pub _type: Type,
    pub flags: Flags,
    pub addr: u32,
    pub offset: u32,
    pub size: u32,
    pub link: u32,
    pub info: u32,
    pub addralign: u32,
    pub entsize: u32,
}

pub fn parse32(
    elf_header: &crate::elf_header::Header32,
    b: &[u8; 40],
) -> Result<Header32, error::Error> {
    let (name, _type, flags, addr, offset, size, link, info, addralign, entsize) =
        match elf_header.ident.data {
            crate::elf_ident::Data::ELFDATA2LSB => {
                let name = u32::from_le_bytes(b[0..4].try_into()?);
                let _type = Type::from(u32::from_le_bytes(b[4..8].try_into()?));
                let flags = Flags::from(u32::from_le_bytes(b[8..12].try_into()?));
                let addr = u32::from_le_bytes(b[12..16].try_into()?);
                let offset = u32::from_le_bytes(b[16..20].try_into()?);
                let size = u32::from_le_bytes(b[20..24].try_into()?);
                let link = u32::from_le_bytes(b[24..28].try_into()?);
                let info = u32::from_le_bytes(b[28..32].try_into()?);
                let addralign = u32::from_le_bytes(b[32..36].try_into()?);
                let entsize = u32::from_le_bytes(b[36..40].try_into()?);
                (
                    name, _type, flags, addr, offset, size, link, info, addralign, entsize,
                )
            }
            crate::elf_ident::Data::ELFDATA2MSB => {
                let name = u32::from_be_bytes(b[0..4].try_into()?);
                let _type = Type::from(u32::from_be_bytes(b[4..8].try_into()?));
                let flags = Flags::from(u32::from_be_bytes(b[8..12].try_into()?));
                let addr = u32::from_be_bytes(b[12..16].try_into()?);
                let offset = u32::from_be_bytes(b[16..20].try_into()?);
                let size = u32::from_be_bytes(b[20..24].try_into()?);
                let link = u32::from_be_bytes(b[24..28].try_into()?);
                let info = u32::from_be_bytes(b[28..32].try_into()?);
                let addralign = u32::from_be_bytes(b[32..36].try_into()?);
                let entsize = u32::from_be_bytes(b[36..40].try_into()?);
                (
                    name, _type, flags, addr, offset, size, link, info, addralign, entsize,
                )
            }
            _ => unimplemented!(),
        };
    Ok(Header32 {
        name,
        _type,
        flags,
        addr,
        offset,
        size,
        link,
        info,
        addralign,
        entsize,
    })
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L317-L328
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Header64 {
    pub name: u32,
    pub _type: Type,
    pub flags: Flags,
    pub addr: u64,
    pub offset: u64,
    pub size: u64,
    pub link: u32,
    pub info: u32,
    pub addralign: u64,
    pub entsize: u64,
}

pub fn parse64(
    elf_header: &crate::elf_header::Header64,
    b: &[u8; 64],
) -> Result<Header64, error::Error> {
    let (name, _type, flags, addr, offset, size, link, info, addralign, entsize) =
        match elf_header.ident.data {
            crate::elf_ident::Data::ELFDATA2LSB => {
                let name = u32::from_le_bytes(b[0..4].try_into()?);
                let _type = Type::from(u32::from_le_bytes(b[4..8].try_into()?));
                let flags = Flags::from(u64::from_le_bytes(b[8..16].try_into()?));
                let addr = u64::from_le_bytes(b[16..24].try_into()?);
                let offset = u64::from_le_bytes(b[24..32].try_into()?);
                let size = u64::from_le_bytes(b[32..40].try_into()?);
                let link = u32::from_le_bytes(b[40..44].try_into()?);
                let info = u32::from_le_bytes(b[44..48].try_into()?);
                let addralign = u64::from_le_bytes(b[48..56].try_into()?);
                let entsize = u64::from_le_bytes(b[56..64].try_into()?);
                (
                    name, _type, flags, addr, offset, size, link, info, addralign, entsize,
                )
            }
            crate::elf_ident::Data::ELFDATA2MSB => {
                let name = u32::from_be_bytes(b[0..4].try_into()?);
                let _type = Type::from(u32::from_be_bytes(b[4..8].try_into()?));
                let flags = Flags::from(u64::from_be_bytes(b[8..16].try_into()?));
                let addr = u64::from_be_bytes(b[16..24].try_into()?);
                let offset = u64::from_be_bytes(b[24..32].try_into()?);
                let size = u64::from_be_bytes(b[32..40].try_into()?);
                let link = u32::from_be_bytes(b[40..44].try_into()?);
                let info = u32::from_be_bytes(b[44..48].try_into()?);
                let addralign = u64::from_be_bytes(b[48..56].try_into()?);
                let entsize = u64::from_be_bytes(b[56..64].try_into()?);
                (
                    name, _type, flags, addr, offset, size, link, info, addralign, entsize,
                )
            }
            _ => unimplemented!(),
        };
    Ok(Header64 {
        name,
        _type,
        flags,
        addr,
        offset,
        size,
        link,
        info,
        addralign,
        entsize,
    })
}

#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Header {
    Header32(Header32),
    Header64(Header64),
}
