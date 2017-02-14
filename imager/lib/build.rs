extern crate bindgen;

use std::path::Path;

const OVERRIDES: &'static str = "
#![allow(non_upper_case_globals)]
#![allow(non_camel_case_types)]
#![allow(non_snake_case)]
#![allow(dead_code)]
";

fn main() {
	for d in ["avformat", "avcodec", "avutil"].iter() {
		println!("cargo:rustc-link-lib={}", d);
	}

	let dir = Path::new("src/bindings");

	bindgen::Builder::default()
		.no_unstable_rust()
		.header(dir.join("wrapper.h").to_str().unwrap())
		.raw_line(OVERRIDES)
		.generate()
		.expect("unable to generate bindings")
		.write_to_file(dir.join("mod.rs").to_str().unwrap())
		.expect("couldn't write bindings!");
}
