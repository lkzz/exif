package exif

import (
	"io/ioutil"
	"os"
	"testing"
)

var filename = "exif_bigEndian.jpg"

func TestStrip(t *testing.T) {
	var (
		err      error
		file     *os.File
		src, dst []byte
	)
	if file, err = os.Open(filename); err != nil {
		t.Fatalf("os.Open(%s) error(%v)", filename, err)
	}
	defer file.Close()
	if src, err = ioutil.ReadAll(file); err != nil {
		t.Fatalf("ioutil.ReadAll error(%v)", err)
	}
	if dst, err = Strip(src); err != nil {
		t.Fatalf("strip exif error(%v)", err)
	}
	if err = ioutil.WriteFile("strip.jpg", dst, 0666); err != nil {
		t.Fatalf("ioutil.WriteFile() error(%v)", err)
	}
}

func TestStripAll(t *testing.T) {
	var (
		err      error
		file     *os.File
		src, dst []byte
	)
	if file, err = os.Open(filename); err != nil {
		t.Fatalf("os.Open(%s) error(%v)", filename, err)
	}
	defer file.Close()
	if src, err = ioutil.ReadAll(file); err != nil {
		t.Fatalf("ioutil.ReadAll error(%v)", err)
	}
	if dst, err = StripAll(src); err != nil {
		t.Fatalf("strip all exif error(%v)", err)
	}
	if err = ioutil.WriteFile("strip_all.jpg", dst, 0666); err != nil {
		t.Fatalf("ioutil.WriteFile() error(%v)", err)
	}
}
