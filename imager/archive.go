package imager

// #cgo LDFLAGS: -L${SRCDIR}/lib -l:libimager.a -ldl -llzma
// #include <stdlib.h>
// #include "lib/imager.h"
import "C"
import (
	"bytes"
	"io/ioutil"
	"path/filepath"
)

// Detect if file is a TAR archive compressed with GZIP
func detectTarGZ(buf []byte) (bool, error) {
	b := C.CBytes(buf)
	defer C.free(b)
	is := C.is_tar_gz((*C.uint8_t)(b), C.size_t(len(buf)))
	return bool(is), nil
}

// Detect if file is a TAR archive compressed with XZ
func detectTarXZ(buf []byte) (bool, error) {
	b := C.CBytes(buf)
	defer C.free(b)
	is := C.is_tar_xz((*C.uint8_t)(b), C.size_t(len(buf)))
	return bool(is), nil
}

// Detect if file is a 7zip archive
func detect7z(buf []byte) (bool, error) {
	return bytes.HasPrefix(buf, []byte{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C}), nil
}

// Attach thumbnail to archive uploads and return
func processArchive() (res thumbResponse) {
	path := filepath.Join(assetRoot, "archive-thumb.png")
	res.thumb, res.err = ioutil.ReadFile(path)
	res.dims = [4]uint16{150, 150, 150, 150}
	return res
}
