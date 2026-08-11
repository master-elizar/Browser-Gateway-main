package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"path/filepath"
	"time"
)

func zipFilePlain(name string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{
		Name:     filepath.Base(name),
		Method:   zip.Deflate,
		Modified: time.Now(),
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func zipFilePassword(name string, data []byte, password string) ([]byte, error) {
	if password == "" {
		return zipFilePlain(name, data)
	}
	return zipCryptoManual(filepath.Base(name), data, password)
}

// zipCryptoManual builds a single-file ZipCrypto (PKWARE) encrypted store archive.
func zipCryptoManual(name string, data []byte, password string) ([]byte, error) {
	crc := crc32.ChecksumIEEE(data)
	keys := zipCryptoKeys{}
	keys.init(password)

	// 12-byte encryption header: 11 random-ish bytes + high CRC byte.
	header := make([]byte, 12)
	seed := crc ^ uint32(time.Now().UnixNano())
	for i := 0; i < 11; i++ {
		seed = seed*1103515245 + 12345
		header[i] = keys.encodeByte(byte(seed >> 16))
	}
	header[11] = keys.encodeByte(byte(crc >> 24))

	enc := make([]byte, len(data))
	for i, b := range data {
		enc[i] = keys.encodeByte(b)
	}

	var buf bytes.Buffer
	// local file header
	mod := time.Now()
	dostime, dosdate := timeToMsDos(mod)
	nameBytes := []byte(name)
	compSize := uint32(12 + len(enc))
	uncompSize := uint32(len(data))

	writeU16 := func(v uint16) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	writeU32 := func(v uint32) { _ = binary.Write(&buf, binary.LittleEndian, v) }

	offsetLocal := buf.Len()
	writeU32(0x04034b50) // local sig
	writeU16(20)         // version needed
	writeU16(1)          // flags: encrypted
	writeU16(0)          // store
	writeU16(dostime)
	writeU16(dosdate)
	writeU32(crc)
	writeU32(compSize)
	writeU32(uncompSize)
	writeU16(uint16(len(nameBytes)))
	writeU16(0) // extra
	buf.Write(nameBytes)
	buf.Write(header)
	buf.Write(enc)

	offsetCD := buf.Len()
	writeU32(0x02014b50) // central dir
	writeU16(20)
	writeU16(20)
	writeU16(1)
	writeU16(0)
	writeU16(dostime)
	writeU16(dosdate)
	writeU32(crc)
	writeU32(compSize)
	writeU32(uncompSize)
	writeU16(uint16(len(nameBytes)))
	writeU16(0)
	writeU16(0)
	writeU16(0)
	writeU16(0)
	writeU32(0)
	writeU32(uint32(offsetLocal))
	buf.Write(nameBytes)

	writeU32(0x06054b50) // end of central
	writeU16(0)
	writeU16(0)
	writeU16(1)
	writeU16(1)
	writeU32(uint32(buf.Len() - offsetCD))
	writeU32(uint32(offsetCD))
	writeU16(0)
	_ = io.EOF
	return buf.Bytes(), nil
}

func timeToMsDos(t time.Time) (uint16, uint16) {
	t = t.Local()
	timeVal := uint16(t.Second()/2) | uint16(t.Minute())<<5 | uint16(t.Hour())<<11
	dateVal := uint16(t.Day()) | uint16(t.Month())<<5 | uint16(t.Year()-1980)<<9
	return timeVal, dateVal
}

type zipCryptoKeys struct {
	k [3]uint32
}

func (z *zipCryptoKeys) init(password string) {
	z.k[0], z.k[1], z.k[2] = 305419896, 591751049, 878082192
	for i := 0; i < len(password); i++ {
		z.update(password[i])
	}
}

func (z *zipCryptoKeys) update(b byte) {
	z.k[0] = crc32Update(z.k[0], b)
	z.k[1] = z.k[1] + (z.k[0] & 0xff)
	z.k[1] = z.k[1]*134775813 + 1
	z.k[2] = crc32Update(z.k[2], byte(z.k[1]>>24))
}

func (z *zipCryptoKeys) magic() byte {
	t := z.k[2]|2
	return byte((t * (t ^ 1)) >> 8)
}

func (z *zipCryptoKeys) encodeByte(b byte) byte {
	c := b ^ z.magic()
	z.update(b)
	return c
}

func crc32Update(crc uint32, b byte) uint32 {
	return crc32.IEEETable[(crc^uint32(b))&0xff] ^ (crc >> 8)
}
