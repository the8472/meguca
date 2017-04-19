use libc::c_char;
use std::ffi::CStr;
use std::io;
use std::io::Write;
use std::thread;
use ws;

macro_rules! stderr(
    ($($arg:tt)*) => { {
        let r = writeln!(&mut io::stderr(), $($arg)*);
        r.expect("failed printing to stderr");
    } }
);

// struct Server {
// 	ws: ws::Sender,
// }

// impl ws::Handler for Server {
// 	fn on_message(&mut self, msg: ws::Message) -> ws::Result<()> {
// 		println!("got message '{}'", msg);

// 		Ok(())
// 	}

// 	fn on_close(&mut self, _: ws::CloseCode, _: &str) {
// 		self.ws.shutdown().unwrap();
// 	}
// }

// Start the websocket server
#[no_mangle]
pub extern "C" fn start(addr: *const c_char) {
	let a = unsafe { CStr::from_ptr(addr).to_str().unwrap() };
	thread::spawn(move || {
		ws::listen(a, |out| {
				move |msg| {

					// Handle messages received on this connection
					stderr!("Server got message '{}'. ", msg);

					// Use the out channel to send messages back
					out.send(msg)
				}
			})
			.unwrap();
	});
}
