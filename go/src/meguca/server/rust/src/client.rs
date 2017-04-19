use std::io;
use std::net::{SocketAddr, TcpStream};
use websocket;

macro_rules! stderr(
    ($($arg:tt)*) => { {
        let r = writeln!(&mut ::std::io::stderr(), $($arg)*);
        r.expect("failed printing to stderr");
    } }
);

// Handler for websocket communications for each individual client
pub struct Client {
	ip: SocketAddr,
	ws: websocket::Client<TcpStream>,
}

impl Client {
	pub fn new(ws: websocket::Client<TcpStream>) -> Option<io::Error> {
		let ip = match ws.peer_addr() {
			Ok(ip) => ip,
			Err(err) => return Some(err),
		};
		Client { ws: ws, ip: ip };
		None
	}
}

// impl for Client {
// 	fn on_message(&mut self, msg: ws::Message) -> ws::Result<()> {
// 		stderr!("{}", msg);

// 		Ok(())
// 	}

// 	fn on_close(&mut self, _: ws::CloseCode, _: &str) {
// 		if let Err(e) = self.ws.shutdown() {
// 			stderr!("{}", e);
// 		}
// 	}
// }
