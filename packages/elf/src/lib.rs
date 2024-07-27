use std::io::BufRead;

pub mod elf_header;
pub mod elf_ident;
pub mod section_header;
pub mod symbol;

#[derive(Debug)]
pub struct ParseError;

impl std::error::Error for ParseError {}
impl std::fmt::Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "parse error")
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Elf {
    pub elf_header: elf_header::Header,
    pub section_table: std::collections::HashMap<String, section_header::Header>,
    pub symbol_table: std::collections::HashMap<String, symbol::Symbol>,
}

pub fn parse(b: &[u8]) -> Result<Elf, error::Error> {
    let mut section_table = std::collections::HashMap::new();
    let mut symbol_table = std::collections::HashMap::new();

    let ident = elf_ident::parse(b[..16].try_into()?)?;
    let elf_header = match ident.class {
        elf_ident::Class::ELFCLASS32 => {
            let elf_header = elf_header::parse32(ident, b[0..52].try_into()?)?;

            let section_headers = (0..elf_header.shnum as usize)
                .map(|i| {
                    let size = 40;
                    let offset = elf_header.shoff as usize + size * i;
                    let section_header =
                        section_header::parse32(&elf_header, b[offset..size + offset].try_into()?)?;
                    Ok(section_header)
                })
                .collect::<Result<Vec<section_header::Header32>, error::Error>>()?;

            let string_table_offset = section_headers[elf_header.shstrndx as usize].offset as usize;

            for i in 0..section_headers.len() {
                let section_header = &section_headers[i];
                match section_header._type {
                    section_header::Type::SHT_SYMTAB | section_header::Type::SHT_DYNSYM => {
                        let string_table_offset =
                            section_headers[section_header.link as usize].offset as usize;

                        let entsize = section_header.entsize as usize;
                        let entry_number = section_header.size as usize / entsize;

                        let offset = section_header.offset as usize;
                        let raw = &b[offset..offset + section_header.size as usize];
                        for i in 0..entry_number {
                            let start = i * entsize;
                            let end = (i + 1) * entsize;
                            let symbol =
                                symbol::parse32(&elf_header, &raw[start..end].try_into()?)?;
                            let symbol_name = read_from_string_table(
                                &b[string_table_offset + symbol.name as usize..],
                            )?;
                            symbol_table.insert(symbol_name, symbol::Symbol::Symbol32(symbol));
                        }
                    }
                    _ => {}
                };
                let section_name = read_from_string_table(
                    &b[string_table_offset + section_header.name as usize..],
                )?;
                section_table.insert(
                    section_name,
                    section_header::Header::Header32(section_header.clone()),
                );
            }
            elf_header::Header::Header32(elf_header)
        }
        elf_ident::Class::ELFCLASS64 => {
            let elf_header = elf_header::parse64(ident, b[0..64].try_into()?)?;

            let section_headers = (0..elf_header.shnum as usize)
                .map(|i| {
                    let size = 64;
                    let offset = elf_header.shoff as usize + size * i;
                    let section_header =
                        section_header::parse64(&elf_header, b[offset..size + offset].try_into()?)?;
                    Ok(section_header)
                })
                .collect::<Result<Vec<section_header::Header64>, error::Error>>()?;

            let str_offset = section_headers[elf_header.shstrndx as usize].offset as usize;

            for i in 0..section_headers.len() {
                let section_header = &section_headers[i];
                match section_header._type {
                    section_header::Type::SHT_SYMTAB | section_header::Type::SHT_DYNSYM => {
                        let str_offset =
                            section_headers[section_header.link as usize].offset as usize;

                        let entsize = section_header.entsize as usize;
                        let entry_number = section_header.size as usize / entsize;

                        let offset = section_header.offset as usize;
                        let raw = &b[offset..offset + section_header.size as usize];
                        for i in 0..entry_number {
                            let start = i * entsize;
                            let end = (i + 1) * entsize;
                            let symbol =
                                symbol::parse64(&elf_header, &raw[start..end].try_into()?)?;
                            let symbol_name =
                                read_from_string_table(&b[str_offset + symbol.name as usize..])?;
                            symbol_table.insert(symbol_name, symbol::Symbol::Symbol64(symbol));
                        }
                    }
                    _ => {}
                };
                let section_name =
                    read_from_string_table(&b[str_offset + section_header.name as usize..])?;
                section_table.insert(
                    section_name,
                    section_header::Header::Header64(section_header.clone()),
                );
            }
            elf_header::Header::Header64(elf_header)
        }
        _ => unimplemented!(),
    };
    Ok(Elf {
        elf_header,
        section_table,
        symbol_table,
    })
}

fn read_from_string_table(b: &[u8]) -> Result<String, error::Error> {
    let mut buf = Vec::new();
    let _ = std::io::BufReader::new(b).read_until(b'\x00', &mut buf)?;
    buf.remove(buf.len() - 1); // \x00
    let s = String::from_utf8(buf)?;
    Ok(s)
}
