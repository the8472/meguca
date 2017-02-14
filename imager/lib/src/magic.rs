use common::FileType;

static MATCHERS: [Matcher; 9] = [Matcher {
	                                 typ: FileType::JPEG,
	                                 prefix: b"\xFF\xD8\xFF",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::PNG,
	                                 prefix:
		                                 b"\x89\x50\x4E\x47\x0D\x0A\x1A\x0A",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::GIF,
	                                 prefix: b"GIF87a",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::GIF,
	                                 prefix: b"GIF89a",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::WEBM,
	                                 prefix: b"\x1A\x45\xDF\xA3",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::PDF,
	                                 prefix: b"%PDF-",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::OGG,
	                                 prefix: b"\x4F\x67\x67\x53\x00",
	                                 mask: Some(b"OggS\x00"),
                                 },
                                 Matcher {
	                                 typ: FileType::ZIP,
	                                 prefix: b"\x50\x4B\x03\x04",
	                                 mask: None,
                                 },
                                 Matcher {
	                                 typ: FileType::SevenZip,
	                                 prefix: b"7z\xBC\xAF\x27\x1C",
	                                 mask: None,
                                 }];

struct Matcher {
	typ: FileType,
	prefix: &'static [u8],
	mask: Option<&'static [u8]>,
}

impl Matcher {
	fn check(&self, src: &[u8]) -> bool {
		match self.mask {
			None => src.starts_with(self.prefix),
			Some(mask) => {
				if src.len() < mask.len() {
					return false;
				}
				for (i, m) in src.iter().enumerate() {
					if (src[i] & m) != self.prefix[i] {
						return false;
					}
				}
				return true;;
			}
		}
	}
}

// Match a file type by its magic number sequence
pub fn match_by_magic(buf: &[u8]) -> Option<FileType> {
	for m in MATCHERS.iter() {
		if m.check(buf) {
			return Some(m.typ);
		}
	}
	None
}
