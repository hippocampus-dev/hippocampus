use std::io::Read;

#[test]
fn ok_64bit() -> Result<(), error::Error> {
    let mut file = std::fs::File::open("tests/fixtures/sample64")?;
    let mut v: Vec<u8> = Vec::new();
    let _ = file.read_to_end(&mut v);
    let e = elf::parse(&v)?;

    if let elf::elf_header::Header::Header64(header) = e.elf_header {
        assert_eq!(header.ident.class, elf::elf_ident::Class::ELFCLASS64);
        assert_eq!(header.ident.data, elf::elf_ident::Data::ELFDATA2LSB);
        assert_eq!(header.ident.version, elf::elf_ident::Version::EV_CURRENT);
        assert_eq!(header.ident.osabi, 0);
        assert_eq!(header.ident.abi_version, 0);
        assert_eq!(header._type, elf::elf_header::Type::DYN);
        assert_eq!(header.machine, elf::elf_header::Machine::EM_X86_64);
        assert_eq!(header.version, elf::elf_header::Version::EV_CURRENT);
        assert_eq!(header.entry, 0x1040);
        assert_eq!(header.phoff, 64);
        assert_eq!(header.shoff, 14016);
        assert_eq!(header.flags, 0);
        assert_eq!(header.ehsize, 64);
        assert_eq!(header.phentsize, 56);
        assert_eq!(header.phnum, 13);
        assert_eq!(header.shentsize, 64);
        assert_eq!(header.shnum, 30);
        assert_eq!(header.shstrndx, 29);
    } else {
        panic!("elf_header should be Header64")
    }

    if let Some(elf::section_header::Header::Header64(section_header)) =
        e.section_table.get(".symtab")
    {
        assert_eq!(section_header._type, elf::section_header::Type::SHT_SYMTAB);
        assert_eq!(section_header.addr, 0);
        assert_eq!(section_header.offset, 0x3048);
        assert_eq!(section_header.size, 0x378);
        assert_eq!(section_header.entsize, 0x18);
        assert_eq!(section_header.link, 28);
        assert_eq!(section_header.info, 19);
        assert_eq!(section_header.addralign, 8);
        assert_eq!(section_header.flags, elf::section_header::Flags::Unknown);
    } else {
        panic!(".symtab is not found");
    }

    if let Some(elf::symbol::Symbol::Symbol64(symbol)) = e.symbol_table.get("main") {
        assert_eq!(symbol.value, 0x1139);
        assert_eq!(symbol.size, 26);
        assert_eq!(symbol._type, elf::symbol::Type::STT_FUNC);
        assert_eq!(symbol.bind, elf::symbol::Bind::STB_GLOBAL);
        assert_eq!(symbol.visibility, elf::symbol::Visibility::STV_DEFAULT);
        assert_eq!(symbol.shndx, 14);
    } else {
        panic!("main is not found");
    }
    Ok(())
}

#[test]
fn ok_32bit() -> Result<(), error::Error> {
    let mut file = std::fs::File::open("tests/fixtures/sample32")?;
    let mut v: Vec<u8> = Vec::new();
    let _ = file.read_to_end(&mut v);
    let e = elf::parse(&v)?;

    if let elf::elf_header::Header::Header32(header) = e.elf_header {
        assert_eq!(header.ident.class, elf::elf_ident::Class::ELFCLASS32);
        assert_eq!(header.ident.data, elf::elf_ident::Data::ELFDATA2LSB);
        assert_eq!(header.ident.version, elf::elf_ident::Version::EV_CURRENT);
        assert_eq!(header.ident.osabi, 0);
        assert_eq!(header.ident.abi_version, 0);
        assert_eq!(header._type, elf::elf_header::Type::DYN);
        assert_eq!(header.machine, elf::elf_header::Machine::EM_386);
        assert_eq!(header.version, elf::elf_header::Version::EV_CURRENT);
        assert_eq!(header.entry, 0x1060);
        assert_eq!(header.phoff, 52);
        assert_eq!(header.shoff, 13828);
        assert_eq!(header.flags, 0);
        assert_eq!(header.ehsize, 52);
        assert_eq!(header.phentsize, 32);
        assert_eq!(header.phnum, 12);
        assert_eq!(header.shentsize, 40);
        assert_eq!(header.shnum, 30);
        assert_eq!(header.shstrndx, 29);
    } else {
        panic!("elf_header should be Header64")
    }

    if let Some(elf::section_header::Header::Header32(section_header)) =
        e.section_table.get(".symtab")
    {
        assert_eq!(section_header._type, elf::section_header::Type::SHT_SYMTAB);
        assert_eq!(section_header.addr, 0);
        assert_eq!(section_header.offset, 0x3030);
        assert_eq!(section_header.size, 0x290);
        assert_eq!(section_header.entsize, 0x10);
        assert_eq!(section_header.link, 28);
        assert_eq!(section_header.info, 19);
        assert_eq!(section_header.addralign, 4);
        assert_eq!(section_header.flags, elf::section_header::Flags::Unknown);
    } else {
        panic!(".symtab is not found");
    }

    if let Some(elf::symbol::Symbol::Symbol32(symbol)) = e.symbol_table.get("main") {
        assert_eq!(symbol.value, 0x118d);
        assert_eq!(symbol.size, 60);
        assert_eq!(symbol._type, elf::symbol::Type::STT_FUNC);
        assert_eq!(symbol.bind, elf::symbol::Bind::STB_GLOBAL);
        assert_eq!(symbol.visibility, elf::symbol::Visibility::STV_DEFAULT);
        assert_eq!(symbol.shndx, 14);
    } else {
        panic!("main is not found");
    }
    Ok(())
}

#[test]
fn invalid() -> Result<(), error::Error> {
    let mut file = std::fs::File::open("tests/fixtures/invalid")?;
    let mut v: Vec<u8> = Vec::new();
    let _ = file.read_to_end(&mut v);
    assert!(elf::parse(&v).is_err());
    Ok(())
}
