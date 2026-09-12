// Package codec implements a small, dependency-free compression engine
// with several interchangeable algorithms behind a common interface,
// plus a self-describing container format and an "auto" strategy that
// picks whichever algorithm compresses a given input best.
package codec

// Algorithm identifiers, stored as a single byte in the container
// header so decompression knows which codec to use.
const (
	AlgoStore   byte = 0 // no compression (safe fallback for incompressible data)
	AlgoHuffman byte = 1 // canonical Huffman coding
	AlgoRLE     byte = 2 // PackBits-style run-length encoding
)

// Codec is one compression algorithm. Implementations operate purely on
// their own payload bytes; the container layer handles the outer header,
// checksum and original-size bookkeeping.
type Codec interface {
	ID() byte
	Name() string
	// Compress turns raw input into an algorithm-specific payload.
	Compress(data []byte) ([]byte, error)
	// Decompress reverses Compress. originalSize is supplied from the
	// container header so codecs that need it can stop at exactly the
	// right byte.
	Decompress(payload []byte, originalSize uint64) ([]byte, error)
}

// registry maps an algorithm ID to its codec implementation.
var registry = map[byte]Codec{}

func register(c Codec) {
	registry[c.ID()] = c
}

func init() {
	register(storeCodec{})
	register(huffmanCodec{})
	register(rleCodec{})
}

// codecFor returns the codec registered for id, or false.
func codecFor(id byte) (Codec, bool) {
	c, ok := registry[id]
	return c, ok
}

// AlgorithmName returns a human-readable name for an algorithm ID.
func AlgorithmName(id byte) string {
	if c, ok := codecFor(id); ok {
		return c.Name()
	}
	return "unknown"
}

// AllCodecs returns every registered codec in a stable order
// (store, huffman, rle) — handy for "try them all" strategies.
func AllCodecs() []Codec {
	return []Codec{registry[AlgoStore], registry[AlgoHuffman], registry[AlgoRLE]}
}
