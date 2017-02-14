use bindings as C;
use std::borrow::BorrowMut;
use std::error::Error;
use std::ffi::CString;
use std::fmt;
use std::sync::{ONCE_INIT, Once};
use libc::{malloc, free, c_void};

const BUFFER_SIZE: usize = 4 << 10;
static INIT: Once = ONCE_INIT;

fn init() {
	unsafe {
		C::av_register_all();
		C::avcodec_register_all();
		C::av_log_set_level(16);
	};
}

// Converts FFMPEG errors to Rust errors
fn format_error(e: i32) -> FFError {
	let mut buf = Box::new([0i8; 1 << 10]);
	unsafe {
		let raw = Box::into_raw(buf) as *mut i8;
		C::av_strerror(e, raw, 1 << 10);
		FFError(CString::from_raw(raw).to_str().unwrap().to_string())
	}
}

#[derive(Debug, Clone)]
struct FFError(String);

impl fmt::Display for FFError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Error for FFError {
    fn description(&self) -> &str {
        self.0.as_str()
    }
}

pub struct Context<'a> {
	avfc: C::AVFormatContext,
	data: &'a [u8],
	buf: *mut c_void,
}

impl<'a> Context<'a> {
	fn new(src: &[u8]) -> Result<Self, String> {
		INIT.call_once(init);

		let c = Context{
			avfc: *C::avformat_alloc_context(),
			data: src,
			buf:  malloc(BUFFER_SIZE),
		};
		c.avfc.pb = C::avio_alloc_context(c.buf, BUFFER_SIZE, c.avfc, 0, Some(c.read), None, Some(c.seek));
	}

	unsafe extern fn read(opaque: *mut c_void, buf: *mut u8, size: i32) -> usize {
		unimplemented!();
	}

	unsafe extern fn seek(opaque: *mut c_void, offset: i64, whence: i32) -> usize {
		unimplemented!();
	}
}

