package codec

// bitWriter packs individual bits (MSB-first within each byte) into a
// growing []byte buffer.
type bitWriter struct {
	buf   []byte
	cur   byte
	nbits uint8
}

func newBitWriter(sizeHint int) *bitWriter {
	if sizeHint < 8 {
		sizeHint = 8
	}
	return &bitWriter{buf: make([]byte, 0, sizeHint)}
}

func (w *bitWriter) writeBit(bit byte) {
	w.cur = (w.cur << 1) | (bit & 1)
	w.nbits++
	if w.nbits == 8 {
		w.buf = append(w.buf, w.cur)
		w.cur = 0
		w.nbits = 0
	}
}

// writeBits writes the low n bits of v, most-significant of those first.
func (w *bitWriter) writeBits(v uint64, n uint8) {
	for i := int(n) - 1; i >= 0; i-- {
		w.writeBit(byte((v >> uint(i)) & 1))
	}
}

// finish flushes a partial final byte (zero-padded) and reports how many
// padding bits were added (0 when already byte-aligned).
func (w *bitWriter) finish() (data []byte, padding uint8) {
	if w.nbits == 0 {
		return w.buf, 0
	}
	padding = 8 - w.nbits
	w.cur <<= padding
	w.buf = append(w.buf, w.cur)
	w.cur, w.nbits = 0, 0
	return w.buf, padding
}

// bitReader walks a []byte MSB-first, bit by bit.
type bitReader struct {
	data    []byte
	bytePos int
	bitPos  uint8
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (r *bitReader) readBit() (byte, bool) {
	if r.bytePos >= len(r.data) {
		return 0, false
	}
	b := (r.data[r.bytePos] >> (7 - r.bitPos)) & 1
	r.bitPos++
	if r.bitPos == 8 {
		r.bitPos = 0
		r.bytePos++
	}
	return b, true
}
