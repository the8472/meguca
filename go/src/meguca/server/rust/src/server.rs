use client;
use libc::c_char;
use std::ffi::CStr;
use std::io::{Write, stderr};
use std::thread;
use websocket::Server;

// Start the websocket server
#[no_mangle]
pub extern "C" fn start(addr: *const c_char) {
	let _addr = unsafe { CStr::from_ptr(addr).to_str().unwrap() };
	thread::spawn(move || {
		let server = Server::bind(_addr).unwrap();
		for req in server.filter_map(Result::ok) {
			if let Ok(cl) = req.accept() {
				if let Some(err) = client::Client::new(cl) {
					writeln!(&mut stderr(), "{}", err).unwrap();
				}
			}
		}
	});
}
