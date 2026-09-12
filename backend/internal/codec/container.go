package codec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// Container header layout (all integers little-endian):
//
//	offset size field
//	0      4    magic "HFC3"
//	4      1    format version
//	5      1    algorithm id
//	6      4    CRC-32 of the ORIGINAL (uncompressed) data
//	7..    8    original size in bytes
//	18     ...  algorithm payload
const (
	headerSize    = 18
	formatVersion = 1
)

var (
	magic = [4]byte{'H', 'F', 'C', '3'}

	ErrBadMagic           = errors.New("codec: not a valid .huff file (bad magic header)")
	ErrUnsupportedVersion = errors.New("codec: unsupported file format version")
	ErrUnknownAlgorithm   = errors.New("codec: unknown compression algorithm in header")
	ErrCorrupt            = errors.New("codec: file is truncated or corrupt")
	ErrChecksumMismatch   = errors.New("codec: checksum mismatch, decompressed data may be corrupt")
)

// EncodeResult is returned by Encode/EncodeAuto with rich metadata.
type EncodeResult struct {
	Data           []byte
	Algorithm      byte
	AlgorithmName  string
	OriginalSize   uint64
	CompressedSize uint64
	Checksum       uint32
}

// Encode compresses data with a specific algorithm and wraps it in the
// container format.
func Encode(data []byte, algo byte) (*EncodeResult, error) {
	c, ok := codecFor(algo)
	if !ok {
		return nil, ErrUnknownAlgorithm
	}
	payload, err := c.Compress(data)
	if err != nil {
		return nil, err
	}
	checksum := crc32.ChecksumIEEE(data)

	buf := make([]byte, 0, headerSize+len(payload))
	buf = append(buf, magic[:]...)
	buf = append(buf, formatVersion, c.ID())
	var tmp [8]byte
	binary.LittleEndian.PutUint32(tmp[:4], checksum)
	buf = append(buf, tmp[:4]...)
	binary.LittleEndian.PutUint64(tmp[:8], uint64(len(data)))
	buf = append(buf, tmp[:8]...)
	buf = append(buf, payload...)

	return &EncodeResult{
		Data:           buf,
		Algorithm:      c.ID(),
		AlgorithmName:  c.Name(),
		OriginalSize:   uint64(len(data)),
		CompressedSize: uint64(len(buf)),
		Checksum:       checksum,
	}, nil
}

// EncodeAuto tries every registered algorithm and returns the smallest
// container. Ties break toward the earlier codec in AllCodecs order.
func EncodeAuto(data []byte) (*EncodeResult, error) {
	var best *EncodeResult
	for _, c := range AllCodecs() {
		res, err := Encode(data, c.ID())
		if err != nil {
			continue
		}
		if best == nil || res.CompressedSize < best.CompressedSize {
			best = res
		}
	}
	if best == nil {
		return nil, errors.New("codec: no algorithm succeeded")
	}
	return best, nil
}

// DecodeResult is returned by Decode.
type DecodeResult struct {
	Data          []byte
	Algorithm     byte
	AlgorithmName string
	OriginalSize  uint64
	Checksum      uint32
}

// Decode validates the container header, dispatches to the right codec,
// and verifies the CRC-32 of the recovered data. On a checksum mismatch
// it still returns the (suspect) bytes alongside ErrChecksumMismatch so
// callers can choose how to handle it.
func Decode(input []byte) (*DecodeResult, error) {
	if len(input) < headerSize {
		return nil, ErrCorrupt
	}
	if !bytes.Equal(input[0:4], magic[:]) {
		return nil, ErrBadMagic
	}
	if input[4] != formatVersion {
		return nil, ErrUnsupportedVersion
	}
	algo := input[5]
	c, ok := codecFor(algo)
	if !ok {
		return nil, ErrUnknownAlgorithm
	}
	checksum := binary.LittleEndian.Uint32(input[6:10])
	originalSize := binary.LittleEndian.Uint64(input[10:18])
	payload := input[headerSize:]

	data, err := c.Decompress(payload, originalSize)
	if err != nil {
		return nil, ErrCorrupt
	}
	if uint64(len(data)) != originalSize {
		return nil, ErrCorrupt
	}

	actual := crc32.ChecksumIEEE(data)
	result := &DecodeResult{
		Data:          data,
		Algorithm:     algo,
		AlgorithmName: c.Name(),
		OriginalSize:  originalSize,
		Checksum:      actual,
	}
	if actual != checksum {
		return result, ErrChecksumMismatch
	}
	return result, nil
}
