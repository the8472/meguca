all: build

build:
	cargo rustc --release
	cp target/release/libimager.a .

clean:
	rm -rf target libimager.a src/bindings/mod.rs

test:
	cargo test

debug:
	cargo rustc
