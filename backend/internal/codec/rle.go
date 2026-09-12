package codec

import "errors"

// rleCodec implements a PackBits-style run-length encoder. It excels on
// data with long runs of identical bytes (bitmaps, simple images, padded
// files) and, thanks to the "auto" strategy pairing it with Store, never
// meaningfully bloats incompressible input.
//
// Encoding uses a control byte before each block:
//
//	0x00..0x7F  n   -> copy the next (n+1) bytes literally     (1..128 literals)
//	0x81..0xFF  n   -> repeat the next single byte (257-n) times (2..128 repeats)
//	0x80            -> unused / no-op
type rleCodec struct{}

func (rleCodec) ID() byte     { return AlgoRLE }
func (rleCodec) Name() string { return "Run-Length (PackBits)" }

func (rleCodec) Compress(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data)/2+8)
	n := len(data)
	i := 0
	for i < n {
		// Measure the run length of the current byte (cap at 128).
		runLen := 1
		for i+runLen < n && data[i+runLen] == data[i] && runLen < 128 {
			runLen++
		}

		if runLen >= 2 {
			// Emit a repeat block.
			out = append(out, byte(257-runLen), data[i])
			i += runLen
			continue
		}

		// Otherwise accumulate a literal run until we hit a 2+ run or 128.
		litStart := i
		litLen := 0
		for i < n && litLen < 128 {
			// Peek: does a run of >=2 start here? If so, stop the literal.
			if i+1 < n && data[i+1] == data[i] {
				break
			}
			i++
			litLen++
		}
		out = append(out, byte(litLen-1))
		out = append(out, data[litStart:litStart+litLen]...)
	}
	return out, nil
}

func (rleCodec) Decompress(payload []byte, originalSize uint64) ([]byte, error) {
	out := make([]byte, 0, originalSize)
	i := 0
	n := len(payload)
	for i < n {
		ctrl := payload[i]
		i++
		switch {
		case ctrl == 0x80:
			// no-op
		case ctrl < 0x80:
			count := int(ctrl) + 1
			if i+count > n {
				return nil, errors.New("codec: truncated rle literal block")
			}
			out = append(out, payload[i:i+count]...)
			i += count
		default: // 0x81..0xFF
			count := 257 - int(ctrl)
			if i >= n {
				return nil, errors.New("codec: truncated rle repeat block")
			}
			b := payload[i]
			i++
			for k := 0; k < count; k++ {
				out = append(out, b)
			}
		}
	}
	if uint64(len(out)) != originalSize {
		return nil, errors.New("codec: rle output size mismatch")
	}
	return out, nil
}
