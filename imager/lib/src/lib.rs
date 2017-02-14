extern crate libflate;
extern crate lzma;
extern crate libc;

mod archive;
mod common;
mod magic;
mod bindings;
mod ffmpeg;

use archive::{is_tar_gz, is_tar_xz};
use common::CheckResult;
use magic::match_by_magic;
use std::ffi::CString;
use std::os::raw::c_char;
use std::slice;

// TODO: SVG
static MATCHERS: [fn(&[u8]) -> CheckResult; 2] = [is_tar_gz, is_tar_xz];

#[repr(C)]
pub struct Buffer {
	data: *mut u8,
	size: usize,
}

#[repr(C)]
pub struct Response {
	typ: u8,
	err: *mut c_char,
}

// Detects if the upload is of a supported file type, by reading its first 512
// bytes (or decoding in case of MP3 and MP4)
#[no_mangle]
pub extern "C" fn detect_file_type(src: *const Buffer,
                                   res: *mut Response)
                                   -> isize {
	let buf = unsafe { slice::from_raw_parts((*src).data, (*src).size) };

	macro_rules! unwrap_type  {
		($t:expr) => (
			match $t {
				Some(t) => {
					unsafe { (*res).typ = t as u8 };
					return 0;
				}
				None => (),
			}
		)
	}

	macro_rules! error {
		($s:expr) => ({
			unsafe { (*res).err = CString::new($s).unwrap().into_raw() };
			return 1;
		})
	}

	unwrap_type!(match_by_magic(buf));
	for m in MATCHERS.iter() {
		match m(buf) {
			Ok(t) => unwrap_type!(t),
			Err(e) => error!(e.description()),
		};
	}
	error!("unsupported file type");
}
