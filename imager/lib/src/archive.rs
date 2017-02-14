use common::{CheckResult, FileType};
use libflate::gzip;
use lzma::LzmaReader;
use std::io::Read;
use std::marker::Sized;

// Detect if file is a TAR archive compressed with GZIP
pub fn is_tar_gz(buf: &[u8]) -> CheckResult {
	if !buf.starts_with(b"\x1f\x8b\x08") {
		return Ok(None);
	}
	is_tar(gzip::Decoder::new(buf)?, FileType::TGZ)
}

// Read the start of the file and determine, if it is a TAR archive
fn is_tar<D: Read + Sized>(decoder: D, typ: FileType) -> CheckResult {
	let mut decoded = Vec::with_capacity(262);
	decoder.take(262).read_to_end(&mut decoded)?;
	Ok(if decoded[257..].starts_with(b"ustar") {
		Some(typ)
	} else {
		None
	})
}

// Detect if file is a TAR archive compressed with XZ
pub fn is_tar_xz(buf: &[u8]) -> CheckResult {
	if !buf.starts_with(b"\xFD7zXZ\x00") {
		return Ok(None);
	}
	is_tar(LzmaReader::new_decompressor(buf)?, FileType::TXZ)
}
