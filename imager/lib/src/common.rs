use std::error::Error;

// Mirrors common/images.go
#[derive(Clone,Copy)]
pub enum FileType {
	JPEG,
	PNG,
	GIF,
	WEBM,
	PDF,
	SVG,
	MP4,
	MP3,
	OGG,
	ZIP,
	SevenZip,
	TGZ,
	TXZ,
}

pub type CheckResult = Result<Option<FileType>, Box<Error>>;
