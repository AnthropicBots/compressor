package codec

import (
	"bytes"
	"math/rand"
	"testing"
)

func roundTripAlgo(t *testing.T, algo byte, data []byte) {
	t.Helper()
	enc, err := Encode(data, algo)
	if err != nil {
		t.Fatalf("Encode(%d) failed: %v", algo, err)
	}
	dec, err := Decode(enc.Data)
	if err != nil {
		t.Fatalf("Decode(%d) failed: %v", algo, err)
	}
	if !bytes.Equal(dec.Data, data) {
		t.Fatalf("algo %d round trip mismatch: got %d bytes want %d", algo, len(dec.Data), len(data))
	}
	if dec.Algorithm != algo {
		t.Fatalf("algo id mismatch: got %d want %d", dec.Algorithm, algo)
	}
}

var testCases = map[string][]byte{
	"empty":          {},
	"single":         {'x'},
	"repeated":       bytes.Repeat([]byte{'a'}, 5000),
	"two-symbols":    []byte("abababababab"),
	"text":           []byte("hello huffman compression project\nthis is my first compressor\n"),
	"runs":           append(bytes.Repeat([]byte{0}, 1000), bytes.Repeat([]byte{0xFF}, 1000)...),
	"mixed-runs-lit": []byte("aaaaabcdefghijkkkkkkkkkklmnop"),
}

func TestAllAlgosRoundTrip(t *testing.T) {
	for _, c := range AllCodecs() {
		for name, data := range testCases {
			t.Run(c.Name()+"/"+name, func(t *testing.T) {
				roundTripAlgo(t, c.ID(), data)
			})
		}
	}
}

func TestAllByteValues(t *testing.T) {
	data := make([]byte, 256*8)
	for i := range data {
		data[i] = byte(i % 256)
	}
	for _, c := range AllCodecs() {
		roundTripAlgo(t, c.ID(), data)
	}
}

func TestRandomBinaryRoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	data := make([]byte, 40000)
	r.Read(data)
	for _, c := range AllCodecs() {
		roundTripAlgo(t, c.ID(), data)
	}
}

func TestEncodeAutoPicksSmallest(t *testing.T) {
	// Highly repetitive data — RLE or Huffman should beat Store.
	data := bytes.Repeat([]byte{7}, 10000)
	auto, err := EncodeAuto(data)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := Encode(data, AlgoStore)
	if auto.CompressedSize >= store.CompressedSize {
		t.Fatalf("auto (%d) should beat store (%d)", auto.CompressedSize, store.CompressedSize)
	}
	dec, err := Decode(auto.Data)
	if err != nil || !bytes.Equal(dec.Data, data) {
		t.Fatalf("auto round trip failed")
	}
}

func TestAutoNeverBloatsMuch(t *testing.T) {
	// Random data is incompressible; auto should fall back to Store and
	// stay within the small header overhead.
	r := rand.New(rand.NewSource(99))
	data := make([]byte, 10000)
	r.Read(data)
	auto, err := EncodeAuto(data)
	if err != nil {
		t.Fatal(err)
	}
	if auto.CompressedSize > uint64(len(data))+headerSize {
		t.Fatalf("auto bloated too much: %d vs %d", auto.CompressedSize, len(data))
	}
	if auto.Algorithm != AlgoStore {
		t.Logf("note: auto picked %s for random data", auto.AlgorithmName)
	}
}

func TestDecodeRejectsBadMagic(t *testing.T) {
	if _, err := Decode([]byte("definitely not a huff file")); err != ErrBadMagic {
		t.Fatalf("expected ErrBadMagic, got %v", err)
	}
}

func TestDecodeDetectsCorruption(t *testing.T) {
	enc, _ := Encode([]byte("some sample text that is reasonably long for corruption"), AlgoHuffman)
	// Flip a bit deep in the payload.
	corrupt := make([]byte, len(enc.Data))
	copy(corrupt, enc.Data)
	corrupt[len(corrupt)-1] ^= 0xFF
	_, err := Decode(corrupt)
	if err == nil {
		t.Fatalf("expected an error decoding corrupted data")
	}
}

func TestAnalyzeEntropy(t *testing.T) {
	// A perfectly uniform 2-symbol stream has entropy 1 bit/byte.
	data := bytes.Repeat([]byte{0, 1}, 5000)
	a := Analyze(data, DefaultAnalyzeOptions())
	if a.DistinctSymbols != 2 {
		t.Fatalf("expected 2 distinct symbols, got %d", a.DistinctSymbols)
	}
	if a.Entropy < 0.99 || a.Entropy > 1.01 {
		t.Fatalf("expected entropy ~1.0, got %f", a.Entropy)
	}
	if a.Tree == nil {
		t.Fatalf("expected a tree for a 2-symbol alphabet")
	}
	if len(a.Comparisons) != 3 {
		t.Fatalf("expected 3 algorithm comparisons, got %d", len(a.Comparisons))
	}
}

func TestCanonicalCodesArePrefixFree(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog, again and again!")
	freq := map[byte]uint64{}
	for _, b := range data {
		freq[b]++
	}
	lengths := codeLengths(freq)
	codes := canonicalCodes(lengths)
	strs := make([]string, 0, len(codes))
	for s, c := range codes {
		strs = append(strs, codeString(c, lengths[s]))
	}
	for i := range strs {
		for j := range strs {
			if i == j {
				continue
			}
			if len(strs[i]) <= len(strs[j]) && strs[j][:len(strs[i])] == strs[i] {
				t.Fatalf("code %q is a prefix of %q — not prefix-free", strs[i], strs[j])
			}
		}
	}
}
