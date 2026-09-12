package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"huffman-compressor/internal/codec"
)

// runCLI handles the non-server subcommands. It returns true if it
// handled a subcommand (so main can exit), or false to fall through to
// the web server.
func runCLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "compress", "c":
		cliCompress(args[1:])
		return true
	case "decompress", "d", "x":
		cliDecompress(args[1:])
		return true
	case "analyze", "a":
		cliAnalyze(args[1:])
		return true
	case "serve", "server":
		// Handled by main after this returns false.
		return false
	case "help", "-h", "--help":
		printUsage()
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Print(`🗜️  Huffman Compressor

Usage:
  huffman-compressor [serve] [-addr :8080]      Run the web server (default)
  huffman-compressor compress   <in> [out] [-a auto|huffman|rle|store]
  huffman-compressor decompress <in.huff> [out]
  huffman-compressor analyze    <in>
  huffman-compressor help

Examples:
  huffman-compressor compress notes.txt
  huffman-compressor compress photo.bmp photo.huff -a rle
  huffman-compressor decompress notes.huff
  huffman-compressor analyze bigfile.bin
`)
}

func parseAlgoFlag(args []string) (byte, []string) {
	algo := byte(255) // auto
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-a" || args[i] == "--algorithm" {
			if i+1 < len(args) {
				switch strings.ToLower(args[i+1]) {
				case "huffman":
					algo = codec.AlgoHuffman
				case "rle":
					algo = codec.AlgoRLE
				case "store":
					algo = codec.AlgoStore
				}
				i++
			}
			continue
		}
		rest = append(rest, args[i])
	}
	return algo, rest
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func cliCompress(args []string) {
	algo, rest := parseAlgoFlag(args)
	if len(rest) < 1 {
		fatal("compress needs an input file")
	}
	in := rest[0]
	out := ""
	if len(rest) >= 2 {
		out = rest[1]
	} else {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ".huff"
	}

	data, err := os.ReadFile(in)
	if err != nil {
		fatal(err.Error())
	}

	var enc *codec.EncodeResult
	if algo == 255 {
		enc, err = codec.EncodeAuto(data)
	} else {
		enc, err = codec.Encode(data, algo)
	}
	if err != nil {
		fatal(err.Error())
	}
	if err := os.WriteFile(out, enc.Data, 0o644); err != nil {
		fatal(err.Error())
	}

	saved := float64(0)
	if enc.OriginalSize > 0 {
		saved = (1 - float64(enc.CompressedSize)/float64(enc.OriginalSize)) * 100
	}
	fmt.Printf("✅ Compressed %s → %s\n", in, out)
	fmt.Printf("   algorithm : %s\n", enc.AlgorithmName)
	fmt.Printf("   original  : %s\n", humanBytes(enc.OriginalSize))
	fmt.Printf("   compressed: %s\n", humanBytes(enc.CompressedSize))
	fmt.Printf("   saved     : %.1f%%\n", saved)
	fmt.Printf("   checksum  : %08x\n", enc.Checksum)
}

func cliDecompress(args []string) {
	if len(args) < 1 {
		fatal("decompress needs an input .huff file")
	}
	in := args[0]
	out := ""
	if len(args) >= 2 {
		out = args[1]
	} else {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ".restored"
	}

	data, err := os.ReadFile(in)
	if err != nil {
		fatal(err.Error())
	}
	res, err := codec.Decode(data)
	if err != nil && err != codec.ErrChecksumMismatch {
		fatal(err.Error())
	}
	if err := os.WriteFile(out, res.Data, 0o644); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("✅ Decompressed %s → %s\n", in, out)
	fmt.Printf("   algorithm : %s\n", res.AlgorithmName)
	fmt.Printf("   size      : %s\n", humanBytes(res.OriginalSize))
	if err == codec.ErrChecksumMismatch {
		fmt.Printf("   ⚠️  checksum MISMATCH — data may be corrupt\n")
	} else {
		fmt.Printf("   checksum  : %08x (verified ✓)\n", res.Checksum)
	}
}

func cliAnalyze(args []string) {
	if len(args) < 1 {
		fatal("analyze needs an input file")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		fatal(err.Error())
	}
	a := codec.Analyze(data, codec.DefaultAnalyzeOptions())
	fmt.Printf("📊 Analysis of %s\n", args[0])
	fmt.Printf("   size            : %s\n", humanBytes(a.TotalBytes))
	fmt.Printf("   distinct bytes  : %d\n", a.DistinctSymbols)
	fmt.Printf("   entropy         : %.3f bits/byte\n", a.Entropy)
	fmt.Printf("   theoretical min : %s\n", humanBytes(a.TheoreticalMin))
	fmt.Printf("   best algorithm  : %s\n", a.BestAlgorithmName)
	fmt.Println("   algorithm comparison:")
	for _, c := range a.Comparisons {
		marker := "  "
		if c.Best {
			marker = "★ "
		}
		fmt.Printf("     %s%-24s %10s  (%.1f%% saved)\n", marker, c.Name, humanBytes(c.Size), c.SpaceSavedPct)
	}
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
