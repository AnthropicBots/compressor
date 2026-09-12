// Package api exposes the compression engine over a small, dependency-free
// JSON/HTTP API built entirely on the Go standard library.
package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"huffman-compressor/internal/codec"
)

// MaxUploadSize caps how large a single uploaded file may be.
const MaxUploadSize = 64 << 20 // 64 MiB

// ---- response types --------------------------------------------------

type CompressResponse struct {
	Filename        string          `json:"filename"`
	Algorithm       string          `json:"algorithm"`
	OriginalSize    uint64          `json:"originalSize"`
	CompressedSize  uint64          `json:"compressedSize"`
	SpaceSavedBytes int64           `json:"spaceSavedBytes"`
	SpaceSavedPct   float64         `json:"spaceSavedPercent"`
	Ratio           float64         `json:"ratio"`
	ProcessingMs    float64         `json:"processingTimeMs"`
	Checksum        string          `json:"checksum"`
	Analysis        *codec.Analysis `json:"analysis"`
	FileBase64      string          `json:"fileBase64"`
	DownloadName    string          `json:"downloadName"`
}

type DecompressResponse struct {
	Filename         string  `json:"filename"`
	Algorithm        string  `json:"algorithm"`
	DecompressedSize uint64  `json:"decompressedSize"`
	ProcessingMs     float64 `json:"processingTimeMs"`
	Checksum         string  `json:"checksum"`
	ChecksumVerified bool    `json:"checksumVerified"`
	FileBase64       string  `json:"fileBase64"`
	DownloadName     string  `json:"downloadName"`
}

type AnalyzeResponse struct {
	Filename     string          `json:"filename"`
	ProcessingMs float64         `json:"processingTimeMs"`
	Analysis     *codec.Analysis `json:"analysis"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ---- helpers ---------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("api: failed writing JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func readUploadedFile(w http.ResponseWriter, r *http.Request) (filename string, data []byte, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize+1<<20)
	if err = r.ParseMultipartForm(MaxUploadSize); err != nil {
		return "", nil, errors.New("could not parse upload (file may exceed the 64 MiB limit)")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", nil, errors.New("no file provided under the 'file' form field")
	}
	defer file.Close()

	buf := make([]byte, 0, header.Size)
	tmp := make([]byte, 32*1024)
	for {
		n, rerr := file.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > MaxUploadSize {
				return "", nil, errors.New("file exceeds the 64 MiB limit")
			}
		}
		if rerr != nil {
			break
		}
	}
	return header.Filename, buf, nil
}

func algorithmFromForm(r *http.Request) byte {
	switch strings.ToLower(strings.TrimSpace(r.FormValue("algorithm"))) {
	case "huffman":
		return codec.AlgoHuffman
	case "rle":
		return codec.AlgoRLE
	case "store":
		return codec.AlgoStore
	default:
		return 255 // sentinel meaning "auto"
	}
}

// ---- handlers --------------------------------------------------------

// HandleCompress implements POST /api/compress. Accepts an optional
// "algorithm" form field (auto|huffman|rle|store); defaults to auto.
func HandleCompress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	filename, data, err := readUploadedFile(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	algo := algorithmFromForm(r)

	start := time.Now()
	var enc *codec.EncodeResult
	if algo == 255 {
		enc, err = codec.EncodeAuto(data)
	} else {
		enc, err = codec.Encode(data, algo)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compression failed: "+err.Error())
		return
	}
	analysis := codec.Analyze(data, codec.DefaultAnalyzeOptions())
	elapsed := time.Since(start)

	var ratio, savedPct float64
	var savedBytes int64
	if enc.OriginalSize > 0 {
		ratio = float64(enc.CompressedSize) / float64(enc.OriginalSize)
		savedBytes = int64(enc.OriginalSize) - int64(enc.CompressedSize)
		savedPct = (1 - ratio) * 100
	}

	writeJSON(w, http.StatusOK, CompressResponse{
		Filename:        filename,
		Algorithm:       enc.AlgorithmName,
		OriginalSize:    enc.OriginalSize,
		CompressedSize:  enc.CompressedSize,
		SpaceSavedBytes: savedBytes,
		SpaceSavedPct:   savedPct,
		Ratio:           ratio,
		ProcessingMs:    msSince(elapsed),
		Checksum:        toHex(enc.Checksum),
		Analysis:        analysis,
		FileBase64:      base64.StdEncoding.EncodeToString(enc.Data),
		DownloadName:    safeName(filename) + ".huff",
	})
}

// HandleDecompress implements POST /api/decompress.
func HandleDecompress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	filename, data, err := readUploadedFile(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	start := time.Now()
	res, err := codec.Decode(data)
	elapsed := time.Since(start)

	verified := true
	if err != nil {
		if errors.Is(err, codec.ErrChecksumMismatch) {
			verified = false
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, DecompressResponse{
		Filename:         filename,
		Algorithm:        res.AlgorithmName,
		DecompressedSize: res.OriginalSize,
		ProcessingMs:     msSince(elapsed),
		Checksum:         toHex(res.Checksum),
		ChecksumVerified: verified,
		FileBase64:       base64.StdEncoding.EncodeToString(res.Data),
		DownloadName:     safeName(strings.TrimSuffix(filename, ".huff")),
	})
}

// HandleAnalyze implements POST /api/analyze. It reports entropy, a byte
// histogram, an accurate algorithm comparison and (for small alphabets)
// the Huffman tree — WITHOUT returning the compressed bytes, so it's a
// fast, cheap preview.
func HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	filename, data, err := readUploadedFile(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	start := time.Now()
	analysis := codec.Analyze(data, codec.DefaultAnalyzeOptions())
	writeJSON(w, http.StatusOK, AnalyzeResponse{
		Filename:     filename,
		ProcessingMs: msSince(time.Since(start)),
		Analysis:     analysis,
	})
}

// HandleHealth implements GET /api/health.
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	algos := []string{}
	for _, c := range codec.AllCodecs() {
		algos = append(algos, c.Name())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"service":    "huffman-compressor",
		"algorithms": algos,
		"time":       time.Now().UTC().Format(time.RFC3339),
	})
}

// Routes wires the API's handlers (CORS-wrapped) onto mux.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/compress", withCORS(HandleCompress))
	mux.HandleFunc("/api/decompress", withCORS(HandleDecompress))
	mux.HandleFunc("/api/analyze", withCORS(HandleAnalyze))
	mux.HandleFunc("/api/health", withCORS(HandleHealth))
}

// ---- small utilities -------------------------------------------------

func msSince(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func toHex(v uint32) string {
	const hexdigits = "0123456789abcdef"
	b := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		b[i] = hexdigits[v&0xF]
		v >>= 4
	}
	return string(b)
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if idx := strings.LastIndex(name, "."); idx > 0 {
		name = name[:idx]
	}
	return name
}
