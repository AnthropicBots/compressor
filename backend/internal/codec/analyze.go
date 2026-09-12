package codec

import (
	"container/heap"
	"math"
	"sort"
)

// HistogramEntry is one byte value's frequency.
type HistogramEntry struct {
	Symbol    int     `json:"symbol"`
	Display   string  `json:"display"`
	Frequency uint64  `json:"frequency"`
	Percent   float64 `json:"percent"`
	Code      string  `json:"code,omitempty"`
	CodeLen   int     `json:"codeLength,omitempty"`
}

// AlgoComparison reports how one algorithm performed on the input.
type AlgoComparison struct {
	Algorithm     byte    `json:"algorithm"`
	Name          string  `json:"name"`
	Size          uint64  `json:"size"`
	Ratio         float64 `json:"ratio"`
	SpaceSavedPct float64 `json:"spaceSavedPercent"`
	Best          bool    `json:"best"`
}

// TreeNode is a serializable Huffman tree for the frontend visualizer.
type TreeNode struct {
	Symbol  *int      `json:"symbol,omitempty"`
	Display string    `json:"display,omitempty"`
	Freq    uint64    `json:"freq"`
	Left    *TreeNode `json:"left,omitempty"`
	Right   *TreeNode `json:"right,omitempty"`
	IsLeaf  bool      `json:"isLeaf"`
}

// Analysis is the full result of analysing an input buffer.
type Analysis struct {
	TotalBytes        uint64           `json:"totalBytes"`
	DistinctSymbols   int              `json:"distinctSymbols"`
	Entropy           float64          `json:"entropyBitsPerByte"`
	TheoreticalMin    uint64           `json:"theoreticalMinBytes"`
	Histogram         []HistogramEntry `json:"histogram"`
	Comparisons       []AlgoComparison `json:"comparisons"`
	BestAlgorithm     byte             `json:"bestAlgorithm"`
	BestAlgorithmName string           `json:"bestAlgorithmName"`
	Tree              *TreeNode        `json:"tree,omitempty"`
}

// display renders a byte as a short human-readable label.
func display(b byte) string {
	switch b {
	case '\n':
		return "\\n"
	case '\r':
		return "\\r"
	case '\t':
		return "\\t"
	case ' ':
		return "SPACE"
	}
	if b < 32 || b > 126 {
		const hex = "0123456789ABCDEF"
		return "0x" + string(hex[b>>4]) + string(hex[b&0xF])
	}
	return string(rune(b))
}

// ShannonEntropy returns the Shannon entropy of data in bits per byte.
func ShannonEntropy(freq map[byte]uint64, total uint64) float64 {
	if total == 0 {
		return 0
	}
	var h float64
	for _, f := range freq {
		if f == 0 {
			continue
		}
		p := float64(f) / float64(total)
		h -= p * math.Log2(p)
	}
	return h
}

// Analyze inspects data and returns entropy, a histogram (with Huffman
// codes), an accurate per-algorithm size comparison, and a tree for
// small alphabets.
func Analyze(data []byte, opts AnalyzeOptions) *Analysis {
	freq := make(map[byte]uint64)
	for _, b := range data {
		freq[b]++
	}
	total := uint64(len(data))
	entropy := ShannonEntropy(freq, total)

	a := &Analysis{
		TotalBytes:      total,
		DistinctSymbols: len(freq),
		Entropy:         entropy,
		TheoreticalMin:  uint64(math.Ceil(entropy * float64(total) / 8.0)),
	}

	// Histogram with canonical Huffman codes attached.
	lengths := codeLengths(freq)
	codes := canonicalCodes(lengths)
	for s := 0; s < 256; s++ {
		f := freq[byte(s)]
		if f == 0 {
			continue
		}
		e := HistogramEntry{
			Symbol:    s,
			Display:   display(byte(s)),
			Frequency: f,
			Percent:   float64(f) / float64(total) * 100,
		}
		if l, ok := lengths[byte(s)]; ok {
			e.CodeLen = int(l)
			e.Code = codeString(codes[byte(s)], l)
		}
		a.Histogram = append(a.Histogram, e)
	}
	sort.Slice(a.Histogram, func(i, j int) bool {
		return a.Histogram[i].Frequency > a.Histogram[j].Frequency
	})
	if opts.HistogramLimit > 0 && len(a.Histogram) > opts.HistogramLimit {
		a.Histogram = a.Histogram[:opts.HistogramLimit]
	}

	// Accurate algorithm comparison — actually run each codec.
	var bestSize uint64
	var bestAlgo byte
	first := true
	for _, c := range AllCodecs() {
		res, err := Encode(data, c.ID())
		if err != nil {
			continue
		}
		cmp := AlgoComparison{
			Algorithm: c.ID(),
			Name:      c.Name(),
			Size:      res.CompressedSize,
		}
		if total > 0 {
			cmp.Ratio = float64(res.CompressedSize) / float64(total)
			cmp.SpaceSavedPct = (1 - cmp.Ratio) * 100
		}
		a.Comparisons = append(a.Comparisons, cmp)
		if first || res.CompressedSize < bestSize {
			bestSize = res.CompressedSize
			bestAlgo = c.ID()
			first = false
		}
	}
	a.BestAlgorithm = bestAlgo
	a.BestAlgorithmName = AlgorithmName(bestAlgo)
	for i := range a.Comparisons {
		if a.Comparisons[i].Algorithm == bestAlgo {
			a.Comparisons[i].Best = true
		}
	}

	// Build a serializable tree when the alphabet is small enough to
	// render cleanly.
	if opts.BuildTree && len(freq) > 0 && len(freq) <= opts.MaxTreeSymbols {
		a.Tree = buildDisplayTree(freq)
	}

	return a
}

// AnalyzeOptions controls how much detail Analyze produces.
type AnalyzeOptions struct {
	HistogramLimit int  // 0 = no limit
	BuildTree      bool // include tree structure
	MaxTreeSymbols int  // only build tree at/under this many symbols
}

// DefaultAnalyzeOptions is a sensible default for API use.
func DefaultAnalyzeOptions() AnalyzeOptions {
	return AnalyzeOptions{HistogramLimit: 64, BuildTree: true, MaxTreeSymbols: 32}
}

func codeString(code uint64, length uint8) string {
	if length == 0 {
		return ""
	}
	b := make([]byte, length)
	for i := int(length) - 1; i >= 0; i-- {
		if code&1 == 1 {
			b[i] = '1'
		} else {
			b[i] = '0'
		}
		code >>= 1
	}
	return string(b)
}

// buildDisplayTree constructs the actual Huffman tree (same shape as the
// one used for coding) as a serializable structure.
func buildDisplayTree(freq map[byte]uint64) *TreeNode {
	h := &hheap{}
	order := 0
	nodeMap := map[*hnode]*TreeNode{}
	for s := 0; s < 256; s++ {
		f := freq[byte(s)]
		if f == 0 {
			continue
		}
		n := &hnode{symbol: s, freq: f, order: order}
		sym := s
		nodeMap[n] = &TreeNode{Symbol: &sym, Display: display(byte(s)), Freq: f, IsLeaf: true}
		*h = append(*h, n)
		order++
	}
	if h.Len() == 0 {
		return nil
	}
	heap.Init(h)
	if h.Len() == 1 {
		only := heap.Pop(h).(*hnode)
		return nodeMap[only]
	}
	for h.Len() > 1 {
		a := heap.Pop(h).(*hnode)
		b := heap.Pop(h).(*hnode)
		parent := &hnode{freq: a.freq + b.freq, order: order, left: a, right: b}
		nodeMap[parent] = &TreeNode{
			Freq:  parent.freq,
			Left:  nodeMap[a],
			Right: nodeMap[b],
		}
		heap.Push(h, parent)
		order++
	}
	root := heap.Pop(h).(*hnode)
	return nodeMap[root]
}
