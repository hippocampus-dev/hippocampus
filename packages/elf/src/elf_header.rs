/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L66-L72
#[allow(clippy::upper_case_acronyms)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Type {
    NONE,
    REL,
    EXEC,
    DYN,
    CORE,
    LOPROC,
    HIPROC,
    Unknown,
}

impl From<u16> for Type {
    fn from(value: u16) -> Self {
        match value {
            0 => Self::NONE,
            1 => Self::REL,
            2 => Self::EXEC,
            3 => Self::DYN,
            4 => Self::CORE,
            0xff00 => Self::LOPROC,
            0xffff => Self::HIPROC,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf-em.h
#[allow(non_camel_case_types)]
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Machine {
    EM_NONE,
    EM_M32,
    EM_SPARC,
    EM_386,
    EM_68K,
    EM_88K,
    EM_486,
    EM_860,
    EM_MIPS,
    // EM_MIPS_RS3_LE,
    // EM_MIPS_RS4_BE,
    EM_PARISC,
    EM_SPARC32PLUS,
    EM_PPC,
    EM_PPC64,
    EM_S390,
    EM_SPU,
    EM_ARM,
    EM_SH,
    EM_SPARCV9,
    EM_H8_300,
    EM_IA_64,
    EM_X86_64,
    EM_CRIS,
    EM_M32R,
    EM_MN10300,
    EM_OPENRISC,
    EM_ARCOMPACT,
    EM_XTENSA,
    EM_BLACKFIN,
    EM_UNICORE,
    EM_ALTERA_NIOS2,
    EM_TI_C6000,
    EM_HEXAGON,
    EM_NDS32,
    EM_AARCH64,
    EM_TILEPRO,
    EM_MICROBLAZE,
    EM_TILEGX,
    EM_ARCV2,
    EM_RISCV,
    EM_BPF,
    EM_CSKY,
    EM_FRV,
    EM_ALPHA,
    EM_CYGNUS_M32R,
    EM_S390_OLD,
    EM_CYGNUS_MN10300,
    Unknown,
}

impl From<u16> for Machine {
    fn from(value: u16) -> Self {
        match value {
            0 => Self::EM_NONE,
            1 => Self::EM_M32,
            2 => Self::EM_SPARC,
            3 => Self::EM_386,
            4 => Self::EM_68K,
            5 => Self::EM_88K,
            6 => Self::EM_486,
            7 => Self::EM_860,
            8 => Self::EM_MIPS,
            // 10 => Self::EM_MIPS_RS3_LE,
            // 10 => Self::EM_MIPS_RS4_BE,
            15 => Self::EM_PARISC,
            18 => Self::EM_SPARC32PLUS,
            20 => Self::EM_PPC,
            21 => Self::EM_PPC64,
            22 => Self::EM_S390,
            23 => Self::EM_SPU,
            40 => Self::EM_ARM,
            42 => Self::EM_SH,
            43 => Self::EM_SPARCV9,
            46 => Self::EM_H8_300,
            50 => Self::EM_IA_64,
            62 => Self::EM_X86_64,
            76 => Self::EM_CRIS,
            88 => Self::EM_M32R,
            89 => Self::EM_MN10300,
            92 => Self::EM_OPENRISC,
            93 => Self::EM_ARCOMPACT,
            94 => Self::EM_XTENSA,
            106 => Self::EM_BLACKFIN,
            110 => Self::EM_UNICORE,
            113 => Self::EM_ALTERA_NIOS2,
            140 => Self::EM_TI_C6000,
            164 => Self::EM_HEXAGON,
            167 => Self::EM_NDS32,
            183 => Self::EM_AARCH64,
            188 => Self::EM_TILEPRO,
            189 => Self::EM_MICROBLAZE,
            191 => Self::EM_TILEGX,
            195 => Self::EM_ARCV2,
            243 => Self::EM_RISCV,
            247 => Self::EM_BPF,
            252 => Self::EM_CSKY,
            0x5441 => Self::EM_FRV,
            0x9026 => Self::EM_ALPHA,
            0x9041 => Self::EM_CYGNUS_M32R,
            0xA390 => Self::EM_S390_OLD,
            0xbeef => Self::EM_CYGNUS_MN10300,
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

impl From<u32> for Version {
    fn from(value: u32) -> Self {
        match value {
            0 => Self::EV_NONE,
            1 => Self::EV_CURRENT,
            2 => Self::EV_NUM,
            _ => Self::Unknown,
        }
    }
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L205-L220
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Header32 {
    pub ident: crate::elf_ident::Ident,
    pub _type: Type,
    pub machine: Machine,
    pub version: Version,
    pub entry: u32,
    pub phoff: u32,
    pub shoff: u32,
    pub flags: u32,
    pub ehsize: u16,
    pub phentsize: u16,
    pub phnum: u16,
    pub shentsize: u16,
    pub shnum: u16,
    pub shstrndx: u16,
}

pub fn parse32(ident: crate::elf_ident::Ident, b: &[u8; 52]) -> Result<Header32, error::Error> {
    let (
        _type,
        machine,
        version,
        entry,
        phoff,
        shoff,
        flags,
        ehsize,
        phentsize,
        phnum,
        shentsize,
        shnum,
        shstrndx,
    ) = match ident.data {
        crate::elf_ident::Data::ELFDATA2LSB => {
            let _type = Type::from(u16::from_le_bytes(b[16..18].try_into()?));
            let machine = Machine::from(u16::from_le_bytes(b[18..20].try_into()?));
            let version = Version::from(u32::from_le_bytes(b[20..24].try_into()?));
            let entry = u32::from_le_bytes(b[24..28].try_into()?);
            let phoff = u32::from_le_bytes(b[28..32].try_into()?);
            let shoff = u32::from_le_bytes(b[32..36].try_into()?);
            let flags = u32::from_le_bytes(b[36..40].try_into()?);
            let ehsize = u16::from_le_bytes(b[40..42].try_into()?);
            let phentsize = u16::from_le_bytes(b[42..44].try_into()?);
            let phnum = u16::from_le_bytes(b[44..46].try_into()?);
            let shentsize = u16::from_le_bytes(b[46..48].try_into()?);
            let shnum = u16::from_le_bytes(b[48..50].try_into()?);
            let shstrndx = u16::from_le_bytes(b[50..52].try_into()?);
            (
                _type, machine, version, entry, phoff, shoff, flags, ehsize, phentsize, phnum,
                shentsize, shnum, shstrndx,
            )
        }
        crate::elf_ident::Data::ELFDATA2MSB => {
            let _type = Type::from(u16::from_be_bytes(b[16..18].try_into()?));
            let machine = Machine::from(u16::from_be_bytes(b[18..20].try_into()?));
            let version = Version::from(u32::from_be_bytes(b[20..24].try_into()?));
            let entry = u32::from_be_bytes(b[24..28].try_into()?);
            let phoff = u32::from_be_bytes(b[28..32].try_into()?);
            let shoff = u32::from_be_bytes(b[32..36].try_into()?);
            let flags = u32::from_be_bytes(b[36..40].try_into()?);
            let ehsize = u16::from_be_bytes(b[40..42].try_into()?);
            let phentsize = u16::from_be_bytes(b[42..44].try_into()?);
            let phnum = u16::from_be_bytes(b[44..46].try_into()?);
            let shentsize = u16::from_be_bytes(b[46..48].try_into()?);
            let shnum = u16::from_be_bytes(b[48..50].try_into()?);
            let shstrndx = u16::from_be_bytes(b[50..52].try_into()?);
            (
                _type, machine, version, entry, phoff, shoff, flags, ehsize, phentsize, phnum,
                shentsize, shnum, shstrndx,
            )
        }
        _ => unimplemented!(),
    };
    Ok(Header32 {
        ident,
        _type,
        machine,
        version,
        entry,
        phoff,
        shoff,
        flags,
        ehsize,
        phentsize,
        phnum,
        shentsize,
        shnum,
        shstrndx,
    })
}

/// https://github.com/torvalds/linux/blob/v5.16/include/uapi/linux/elf.h#L222-L237
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub struct Header64 {
    pub ident: crate::elf_ident::Ident,
    pub _type: Type,
    pub machine: Machine,
    pub version: Version,
    pub entry: u64,
    pub phoff: u64,
    pub shoff: u64,
    pub flags: u32,
    pub ehsize: u16,
    pub phentsize: u16,
    pub phnum: u16,
    pub shentsize: u16,
    pub shnum: u16,
    pub shstrndx: u16,
}

pub fn parse64(ident: crate::elf_ident::Ident, b: &[u8; 64]) -> Result<Header64, error::Error> {
    let (
        _type,
        machine,
        version,
        entry,
        phoff,
        shoff,
        flags,
        ehsize,
        phentsize,
        phnum,
        shentsize,
        shnum,
        shstrndx,
    ) = match ident.data {
        crate::elf_ident::Data::ELFDATA2LSB => {
            let _type = Type::from(u16::from_le_bytes(b[16..18].try_into()?));
            let machine = Machine::from(u16::from_le_bytes(b[18..20].try_into()?));
            let version = Version::from(u32::from_le_bytes(b[20..24].try_into()?));
            let entry = u64::from_le_bytes(b[24..32].try_into()?);
            let phoff = u64::from_le_bytes(b[32..40].try_into()?);
            let shoff = u64::from_le_bytes(b[40..48].try_into()?);
            let flags = u32::from_le_bytes(b[48..52].try_into()?);
            let ehsize = u16::from_le_bytes(b[52..54].try_into()?);
            let phentsize = u16::from_le_bytes(b[54..56].try_into()?);
            let phnum = u16::from_le_bytes(b[56..58].try_into()?);
            let shentsize = u16::from_le_bytes(b[58..60].try_into()?);
            let shnum = u16::from_le_bytes(b[60..62].try_into()?);
            let shstrndx = u16::from_le_bytes(b[62..64].try_into()?);
            (
                _type, machine, version, entry, phoff, shoff, flags, ehsize, phentsize, phnum,
                shentsize, shnum, shstrndx,
            )
        }
        crate::elf_ident::Data::ELFDATA2MSB => {
            let _type = Type::from(u16::from_be_bytes(b[16..18].try_into()?));
            let machine = Machine::from(u16::from_be_bytes(b[18..20].try_into()?));
            let version = Version::from(u32::from_be_bytes(b[20..24].try_into()?));
            let entry = u64::from_be_bytes(b[24..32].try_into()?);
            let phoff = u64::from_be_bytes(b[32..40].try_into()?);
            let shoff = u64::from_be_bytes(b[40..48].try_into()?);
            let flags = u32::from_be_bytes(b[48..52].try_into()?);
            let ehsize = u16::from_be_bytes(b[52..54].try_into()?);
            let phentsize = u16::from_be_bytes(b[54..56].try_into()?);
            let phnum = u16::from_be_bytes(b[56..58].try_into()?);
            let shentsize = u16::from_be_bytes(b[58..60].try_into()?);
            let shnum = u16::from_be_bytes(b[60..62].try_into()?);
            let shstrndx = u16::from_be_bytes(b[62..64].try_into()?);
            (
                _type, machine, version, entry, phoff, shoff, flags, ehsize, phentsize, phnum,
                shentsize, shnum, shstrndx,
            )
        }
        _ => unimplemented!(),
    };
    Ok(Header64 {
        ident,
        _type,
        machine,
        version,
        entry,
        phoff,
        shoff,
        flags,
        ehsize,
        phentsize,
        phnum,
        shentsize,
        shnum,
        shstrndx,
    })
}

#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, std::hash::Hash)]
pub enum Header {
    Header32(Header32),
    Header64(Header64),
}
