package codec

import (
	"container/heap"
	"encoding/binary"
	"errors"
	"sort"
)

// huffmanCodec implements canonical Huffman coding.
//
// Unlike the textbook version that stores an 8-byte frequency for every
// symbol, this stores only a 1-byte canonical code length per present
// symbol. The decoder deterministically reconstructs the exact same
// codes from those lengths alone, which shrinks the header dramatically
// (9 bytes/symbol → 2 bytes/symbol) while remaining perfectly lossless.
//
// Payload layout:
//
//	2 bytes   symbol count N (little-endian, 1..256)
//	N*2 bytes N entries of {symbol:1, codeLength:1}
//	1 byte    padding bits in the final data byte (0..7)
//	...       packed bit stream (MSB-first)
type huffmanCodec struct{}

func (huffmanCodec) ID() byte     { return AlgoHuffman }
func (huffmanCodec) Name() string { return "Canonical Huffman" }

// hnode is a node in the length-computing Huffman tree.
type hnode struct {
	symbol      int
	freq        uint64
	order       int
	left, right *hnode
}

type hheap []*hnode

func (h hheap) Len() int { return len(h) }
func (h hheap) Less(i, j int) bool {
	if h[i].freq != h[j].freq {
		return h[i].freq < h[j].freq
	}
	return h[i].order < h[j].order
}
func (h hheap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *hheap) Push(x any)   { *h = append(*h, x.(*hnode)) }
func (h *hheap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// codeLengths returns, for each present symbol, the depth it would have
// in a Huffman tree built from freq. A lone symbol gets length 1.
func codeLengths(freq map[byte]uint64) map[byte]uint8 {
	present := make([]int, 0, len(freq))
	for s := 0; s < 256; s++ {
		if freq[byte(s)] > 0 {
			present = append(present, s)
		}
	}
	lengths := make(map[byte]uint8, len(present))
	if len(present) == 0 {
		return lengths
	}
	if len(present) == 1 {
		lengths[byte(present[0])] = 1
		return lengths
	}

	h := &hheap{}
	order := 0
	for _, s := range present {
		*h = append(*h, &hnode{symbol: s, freq: freq[byte(s)], order: order})
		order++
	}
	heap.Init(h)
	for h.Len() > 1 {
		a := heap.Pop(h).(*hnode)
		b := heap.Pop(h).(*hnode)
		heap.Push(h, &hnode{freq: a.freq + b.freq, order: order, left: a, right: b})
		order++
	}
	root := heap.Pop(h).(*hnode)

	var walk func(n *hnode, depth uint8)
	walk = func(n *hnode, depth uint8) {
		if n.left == nil && n.right == nil {
			lengths[byte(n.symbol)] = depth
			return
		}
		walk(n.left, depth+1)
		walk(n.right, depth+1)
	}
	walk(root, 0)
	return lengths
}

// canonicalCodes assigns canonical Huffman codes given code lengths.
// Symbols are ordered by (length, symbol) and codes are assigned in
// increasing numeric order — the standard, fully deterministic scheme.
func canonicalCodes(lengths map[byte]uint8) map[byte]uint64 {
	type sl struct {
		symbol byte
		length uint8
	}
	items := make([]sl, 0, len(lengths))
	for s, l := range lengths {
		items = append(items, sl{s, l})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].length != items[j].length {
			return items[i].length < items[j].length
		}
		return items[i].symbol < items[j].symbol
	})

	codes := make(map[byte]uint64, len(items))
	var code uint64
	var prevLen uint8
	for _, it := range items {
		if prevLen != 0 {
			code <<= uint(it.length - prevLen)
		}
		codes[it.symbol] = code
		code++
		prevLen = it.length
	}
	return codes
}

func (c huffmanCodec) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		// Header with zero symbols and zero padding.
		return []byte{0, 0, 0}, nil
	}

	freq := make(map[byte]uint64)
	for _, b := range data {
		freq[b]++
	}
	lengths := codeLengths(freq)
	codes := canonicalCodes(lengths)

	// --- header ---
	out := make([]byte, 0, len(data)/2+len(lengths)*2+8)
	var tmp2 [2]byte
	binary.LittleEndian.PutUint16(tmp2[:], uint16(len(lengths)))
	out = append(out, tmp2[:]...)
	for s := 0; s < 256; s++ {
		l, ok := lengths[byte(s)]
		if !ok {
			continue
		}
		out = append(out, byte(s), byte(l))
	}

	// --- bit stream ---
	bw := newBitWriter(len(data) / 2)
	for _, b := range data {
		bw.writeBits(codes[b], lengths[b])
	}
	bits, padding := bw.finish()

	out = append(out, padding)
	out = append(out, bits...)
	return out, nil
}

func (c huffmanCodec) Decompress(payload []byte, originalSize uint64) ([]byte, error) {
	if len(payload) < 3 {
		return nil, errors.New("codec: huffman payload too short")
	}
	symbolCount := int(binary.LittleEndian.Uint16(payload[0:2]))
	offset := 2

	lengths := make(map[byte]uint8, symbolCount)
	for i := 0; i < symbolCount; i++ {
		if offset+2 > len(payload) {
			return nil, errors.New("codec: truncated huffman header")
		}
		sym := payload[offset]
		l := payload[offset+1]
		lengths[sym] = l
		offset += 2
	}

	if offset >= len(payload) {
		return nil, errors.New("codec: missing huffman padding byte")
	}
	padding := payload[offset]
	offset++
	bitData := payload[offset:]
	if padding > 7 {
		return nil, errors.New("codec: invalid huffman padding")
	}

	if originalSize == 0 {
		return []byte{}, nil
	}

	// Rebuild canonical codes, then a fast length-indexed decode table.
	codes := canonicalCodes(lengths)

	// symbolAt[length][code] -> symbol, expressed via first-code offsets.
	type entry struct {
		code   uint64
		symbol byte
	}
	byLen := map[uint8][]entry{}
	var maxLen uint8
	for sym, l := range lengths {
		byLen[l] = append(byLen[l], entry{codes[sym], sym})
		if l > maxLen {
			maxLen = l
		}
	}
	firstCode := make(map[uint8]uint64)
	countAt := make(map[uint8]uint64)
	symArr := make(map[uint8][]byte)
	for l := uint8(1); l <= maxLen; l++ {
		es := byLen[l]
		if len(es) == 0 {
			continue
		}
		sort.Slice(es, func(i, j int) bool { return es[i].code < es[j].code })
		firstCode[l] = es[0].code
		countAt[l] = uint64(len(es))
		arr := make([]byte, len(es))
		for i, e := range es {
			arr[i] = e.symbol
		}
		symArr[l] = arr
	}

	// Single-symbol special case: every "0" bit is that symbol.
	if len(lengths) == 1 {
		var only byte
		for s := range lengths {
			only = s
		}
		out := make([]byte, originalSize)
		for i := range out {
			out[i] = only
		}
		return out, nil
	}

	out := make([]byte, 0, originalSize)
	r := newBitReader(bitData)
	var code uint64
	var length uint8
	for uint64(len(out)) < originalSize {
		bit, ok := r.readBit()
		if !ok {
			return nil, errors.New("codec: unexpected end of huffman stream")
		}
		code = (code << 1) | uint64(bit)
		length++
		if length > maxLen {
			return nil, errors.New("codec: corrupt huffman stream")
		}
		if cnt, ok := countAt[length]; ok {
			idx := code - firstCode[length]
			if idx < cnt {
				out = append(out, symArr[length][idx])
				code = 0
				length = 0
			}
		}
	}
	return out, nil
}
