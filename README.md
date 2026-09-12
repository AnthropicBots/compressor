# 🗜️ Huffman Compressor

A fast, lossless, **multi-algorithm** file compressor with a native **Go** backend and a dependency-free web dashboard. Originally a small C++ Huffman CLI — rebuilt as a clean, byte-safe compression engine that works on **any file type**, with automatic best-algorithm selection, entropy analysis, and a live Huffman-tree visualizer.

The original C++ project is preserved under [`legacy-cpp/`](legacy-cpp/).

---

## ✨ Features

- **Three interchangeable algorithms** behind one clean interface:
  - **Canonical Huffman** — optimal prefix coding; stores only 1-byte code *lengths* per symbol (2 bytes/symbol header) instead of full frequency tables, and the decoder deterministically rebuilds the codes.
  - **Run-Length (PackBits)** — excels on data with long byte runs (bitmaps, padded files).
  - **Store** — no-op fallback so incompressible data never bloats beyond a tiny header.
- **✨ Auto mode** — tries every algorithm and keeps the smallest result.
- **Byte-oriented (0–255)** — works on text *and* binary: images, executables, archives, anything.
- **Integrity guaranteed** — every archive embeds a **CRC-32** checksum, verified on decompress.
- **Entropy analysis** — Shannon entropy (bits/byte), theoretical minimum size, and coding efficiency.
- **Algorithm showdown** — see exactly how each algorithm performs on your file.
- **Interactive Huffman tree** — rendered as SVG in the browser, with 0/1 edge labels.
- **Byte-frequency histogram** and full **code table** (copyable as JSON).
- **Dual-purpose binary** — the same executable is both a **web server** and a **CLI tool**.
- **Single self-contained binary** — the frontend is embedded with `go:embed`; zero external dependencies (Go standard library only).
- Dark/light theme, drag-and-drop, multi-file queue, and session history.

---

## 🚀 Quick start

### Run the web app
```bash
cd backend
go run . -addr :8080
# open http://localhost:8080
```

Or with the Makefile from the project root:
```bash
make run       # start the server
make build     # build ./backend/bin/huffman-compressor
make test      # run the test suite
make fmt vet   # format & static-check
```

### Use it from the command line
```bash
make build

./backend/bin/huffman-compressor compress   notes.txt              # -> notes.huff (auto)
./backend/bin/huffman-compressor compress   photo.bmp out.huff -a rle
./backend/bin/huffman-compressor decompress notes.huff             # -> notes.restored
./backend/bin/huffman-compressor analyze    bigfile.bin
./backend/bin/huffman-compressor help
```

Example `analyze` output:
```
📊 Analysis of notes.txt
   size            : 18.16 KB
   distinct bytes  : 19
   entropy         : 4.016 bits/byte
   theoretical min : 9.12 KB
   best algorithm  : Canonical Huffman
   algorithm comparison:
       Store (no compression)     18.18 KB  (-0.1% saved)
     ★ Canonical Huffman           9.25 KB  (49.1% saved)
       Run-Length (PackBits)      19.35 KB  (-6.6% saved)
```

---

## 🌐 HTTP API

All endpoints are CORS-enabled. Uploads use `multipart/form-data` with a `file` field.

| Method | Path              | Purpose |
|--------|-------------------|---------|
| `POST` | `/api/compress`   | Compress a file. Optional `algorithm` field: `auto` (default), `huffman`, `rle`, `store`. Returns stats, analysis, and the base64 archive. |
| `POST` | `/api/decompress` | Restore a `.huff` archive; verifies the CRC-32 checksum. |
| `POST` | `/api/analyze`    | Entropy, histogram, per-algorithm comparison, and Huffman tree — **without** returning compressed bytes (fast preview). |
| `GET`  | `/api/health`     | Service status and the list of available algorithms. |

```bash
# Compress (auto-select the best algorithm)
curl -F "file=@notes.txt" http://localhost:8080/api/compress

# Force a specific algorithm
curl -F "file=@photo.bmp" -F "algorithm=rle" http://localhost:8080/api/compress

# Analyze only
curl -F "file=@bigfile.bin" http://localhost:8080/api/analyze
```

---

## 📦 The `.huff` container format (HFC3)

Every archive begins with an 18-byte header, all integers little-endian:

| Offset | Size | Field |
|-------:|-----:|-------|
| 0  | 4 | Magic `HFC3` |
| 4  | 1 | Format version |
| 5  | 1 | Algorithm ID (`0` store, `1` huffman, `2` rle) |
| 6  | 4 | CRC-32 of the original data |
| 10 | 8 | Original size in bytes |
| 18 | … | Algorithm payload |

Because the algorithm ID and original size travel with the file, **decompression needs no extra input** — it dispatches to the right codec and checks integrity automatically.

---

## 🧱 Project structure

```
huffman-compressor/
├── backend/
│   ├── main.go              # server + CLI dispatch, go:embed frontend
│   ├── cli.go               # compress / decompress / analyze subcommands
│   ├── go.mod
│   ├── internal/
│   │   ├── codec/           # the compression engine
│   │   │   ├── codec.go       # Codec interface + algorithm registry
│   │   │   ├── bitio.go       # MSB-first bit reader/writer
│   │   │   ├── huffman.go     # canonical Huffman
│   │   │   ├── rle.go         # PackBits run-length
│   │   │   ├── store.go       # no-op fallback
│   │   │   ├── container.go   # HFC3 container + auto-best strategy
│   │   │   ├── analyze.go     # entropy, histogram, comparison, tree
│   │   │   └── codec_test.go  # round-trips, corruption, entropy, prefix-free
│   │   └── api/handlers.go   # JSON/HTTP handlers
│   └── web/                 # embedded frontend (index.html, style.css, app.js)
├── legacy-cpp/              # the original C++ project, preserved
├── Makefile
├── LICENSE                  # MIT
└── README.md
```

---

## 🔬 How canonical Huffman works here

1. Count byte frequencies and build a Huffman tree to get each symbol's **code length**.
2. Assign **canonical codes**: sort symbols by `(length, value)` and hand out consecutive binary codes. This makes codes reproducible from lengths alone.
3. Store just `symbol → length` pairs in the header (2 bytes each).
4. On decode, rebuild the identical canonical codes from those lengths and read the bit stream using a fast first-code-per-length table.

This keeps the header small while staying a textbook-correct, provably prefix-free optimal code.

---

## ✅ Testing

```bash
cd backend && go test ./... -v
```

The suite covers round-trips for every algorithm across empty / single-byte / repeated / two-symbol / text / run-heavy / all-256-values / 40 KB random inputs, plus auto-best selection, incompressible-data safety, corruption/bad-magic detection, entropy correctness, and a prefix-free property check on the canonical codes.

---

## 📜 License

MIT — see [LICENSE](LICENSE). Original C++ implementation by Mohit Yadav; Go rewrite and dashboard build on that work.
