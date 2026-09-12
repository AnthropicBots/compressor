package codec

// storeCodec performs no compression at all. It exists so the "auto"
// strategy always has a worst-case-safe option: incompressible data
// (already-compressed files, random bytes) never grows by more than the
// tiny container header.
type storeCodec struct{}

func (storeCodec) ID() byte     { return AlgoStore }
func (storeCodec) Name() string { return "Store (no compression)" }

func (storeCodec) Compress(data []byte) ([]byte, error) {
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (storeCodec) Decompress(payload []byte, _ uint64) ([]byte, error) {
	out := make([]byte, len(payload))
	copy(out, payload)
	return out, nil
}
