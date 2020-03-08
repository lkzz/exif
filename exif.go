package exif

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
)

// JPEG图片exif格式如下：
// ---------------
// |  soi        | JPEG 数据起始标志,两字节:ff,d8
// |  app1       | APP1 标记,两字节:ff,e1
// |  size       | APP1 数据大小,两字节(包含标识符本身)
// |  exif       | Exif 固定开头：45,78,69,66,00,00
// |  order      | 字节序开头,四字节(大端:4d,4d,00,2a,小端:49,49,2a,00),
// |  offset     | 到第一个IFD(图像文件目录)的偏移量(包含标识符本身),四字节
// |  ifd0       | 第一个IFD
// |  ifdN       | 第N个IFD
// ---------------
//
// 文件目录项说明
// ---------------
// | tag number | IFD下标签个数标识,两字节
// |------------| 每个标签大小12字节
// | tag id     | 标签号码,两字节
// | data format| 数据格式,两字节,如 byte,ascii,short,long etc
// | elem number| 组件个数,四字节
// | tag value  | 标签值,四字节

// const variable used in exif package
const (
	markerSOI      = 0xffd8
	markerAPP1     = 0xffe1
	byteHeader     = 0x45786966
	byteHeaderExt  = 0x0000
	byteOrderBE    = 0x4d4d
	byteOrderLE    = 0x4949
	byteOrderExt   = 0x002a
	orientationTag = 0x0112
)

// exif errors
var (
	ErrMissSOIMarker    = errors.New("missing JPEG SOI marker")
	ErrNoExif           = errors.New("exif not exist")
	ErrInvalidHeader    = errors.New("invalid exif header")
	ErrInvalidBlockSize = errors.New("invalid block size")
	ErrInvalidOrderFlag = errors.New("invalid byte order flag")
	ErrInvalidOffset    = errors.New("invalid offset value")
	ErrInvalidTagValue  = errors.New("invalid tag value")
)

// Strip remove exif except orientation.
func Strip(in []byte) (out []byte, err error) {
	r := bytes.NewReader(in)
	// Check if JPEG SOI marker is present.
	var soi uint16
	if err = binary.Read(r, binary.BigEndian, &soi); err != nil {
		return
	}
	if soi != markerSOI {
		err = ErrMissSOIMarker
		return
	}
	// Find JPEG APP1 marker.
	var (
		index = 2 // app1 marker index position
		esize int // app1 data size,not include 0xff,0xe1
	)
	for {
		var marker, size uint16
		if err = binary.Read(r, binary.BigEndian, &marker); err != nil {
			return
		}
		if err = binary.Read(r, binary.BigEndian, &size); err != nil {
			return
		}
		if marker>>8 != 0xff {
			err = ErrNoExif
			return
		}
		if size < 2 {
			err = ErrInvalidBlockSize
			return
		}
		if marker == markerAPP1 {
			esize = int(size)
			break
		}
		index = index + int(size) + 2
		if _, err = io.CopyN(ioutil.Discard, r, int64(size)-2); err != nil {
			return
		}
	}
	if esize == 0 {
		err = ErrNoExif
		return
	}
	// Check if EXIF header is present.
	var header uint32
	if err = binary.Read(r, binary.BigEndian, &header); err != nil {
		return
	}
	if header != byteHeader {
		err = ErrInvalidHeader
		return
	}
	if _, err = io.CopyN(ioutil.Discard, r, 2); err != nil { // skip two byte header ext
		return
	}
	// Read byte order information.
	var (
		byteOrderTag uint16
		byteOrder    binary.ByteOrder
	)
	if err = binary.Read(r, binary.BigEndian, &byteOrderTag); err != nil { // two byte ByteOrder
		return
	}
	switch byteOrderTag {
	case byteOrderBE:
		byteOrder = binary.BigEndian
	case byteOrderLE:
		byteOrder = binary.LittleEndian
	default:
		err = ErrInvalidOrderFlag
		return
	}
	if _, err = io.CopyN(ioutil.Discard, r, 2); err != nil { // skip two byte ByteOrder ext
		return
	}
	var offset uint32
	if err = binary.Read(r, byteOrder, &offset); err != nil {
		return
	}
	if offset < 8 {
		err = ErrInvalidOffset
		return
	}
	if _, err = io.CopyN(ioutil.Discard, r, int64(offset-8)); err != nil { // seek to IFD0
		return
	}
	var tagNum uint16
	if err = binary.Read(r, byteOrder, &tagNum); err != nil { // read the number of tags
		return
	}
	ow := new(bytes.Buffer) // orientation io writer
	for i := 0; i < int(tagNum); i++ {
		var tag uint16
		if err = binary.Read(r, byteOrder, &tag); err != nil {
			return
		}
		if tag != orientationTag {
			if _, err = io.CopyN(ioutil.Discard, r, 10); err != nil {
				return
			}
			continue
		}
		binary.Write(ow, byteOrder, uint16(orientationTag)) // write orientation tag id
		if _, err = io.CopyN(ow, r, 10); err != nil {
			return
		}
		break
	}
	ew := new(bytes.Buffer) // exif io writer
	if ow.Len() > 0 {       // if there is orientation in exif,remain orientation tag
		binary.Write(ew, binary.BigEndian, uint16(markerAPP1))    // write app1 marker
		binary.Write(ew, binary.BigEndian, uint16(0x001e))        // write app1 size
		binary.Write(ew, binary.BigEndian, uint32(byteHeader))    // write exif header
		binary.Write(ew, binary.BigEndian, uint16(byteHeaderExt)) // write exif header ext
		binary.Write(ew, binary.BigEndian, uint16(byteOrderTag))  // write byte order
		binary.Write(ew, byteOrder, uint16(byteOrderExt))         // write byte order ext
		binary.Write(ew, byteOrder, uint32(0x00000008))           // write offset:0
		binary.Write(ew, byteOrder, uint16(0x0001))               // write tag number:1
		io.Copy(ew, ow)                                           // write orientation tag
	}
	w := new(bytes.Buffer)
	r.Seek(0, io.SeekStart)
	// Write SOI part
	io.CopyN(w, r, int64(index))
	// Skip exif
	io.CopyN(ioutil.Discard, r, int64(esize)+2)
	// Combine SOI part,orientation exif part and data toghter
	out, err = ioutil.ReadAll(io.MultiReader(w, ew, r))
	return
}

// StripAll remove exif.
func StripAll(in []byte) (out []byte, err error) {
	r := bytes.NewReader(in)
	// Check if JPEG SOI marker is present.
	var soi uint16
	if err = binary.Read(r, binary.BigEndian, &soi); err != nil {
		return
	}
	if soi != markerSOI {
		err = ErrMissSOIMarker
		return
	}
	// Find JPEG APP1 marker.
	var (
		index = 2 // app1 marker index position
		esize int // app1 data size,not include 0xff,0xe1
	)
	for {
		var marker, size uint16
		if err = binary.Read(r, binary.BigEndian, &marker); err != nil {
			return
		}
		if err = binary.Read(r, binary.BigEndian, &size); err != nil {
			return
		}
		if marker>>8 != 0xff {
			err = ErrNoExif
			return
		}
		if size < 2 {
			err = ErrInvalidBlockSize
			return
		}
		if marker == markerAPP1 {
			esize = int(size)
			break
		}
		index = index + int(size) + 2
		if _, err = io.CopyN(ioutil.Discard, r, int64(size)-2); err != nil {
			return
		}
	}
	if esize == 0 {
		err = ErrNoExif
		return
	}
	w := new(bytes.Buffer)
	r.Seek(0, io.SeekStart)
	// Write SOI part
	io.CopyN(w, r, int64(index))
	// Skip exif
	io.CopyN(ioutil.Discard, r, int64(esize)+2)
	// Combine SOI part and data toghter
	out, err = ioutil.ReadAll(io.MultiReader(w, r))
	return
}
