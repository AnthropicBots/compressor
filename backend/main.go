// Command huffman-compressor is a dual-purpose tool. With no arguments
// (or "serve") it runs a self-contained web server exposing a
// multi-algorithm compression engine: a JSON API under /api/* plus the
// bundled single-page frontend under /. It also works as a command-line
// compressor — see `huffman-compressor help`.
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"huffman-compressor/internal/api"
)

//go:embed web
var webFS embed.FS

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	// CLI subcommands take priority; if one is handled we're done.
	if len(os.Args) > 1 && runCLI(os.Args[1:]) {
		return
	}

	// Allow "serve" as an explicit prefix before flags.
	serverArgs := os.Args[1:]
	if len(serverArgs) > 0 && (serverArgs[0] == "serve" || serverArgs[0] == "server") {
		serverArgs = serverArgs[1:]
	}

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", envOr("PORT_ADDR", ":8080"), "address to listen on, e.g. :8080")
	_ = fs.Parse(serverArgs)

	runServer(*addr)
}

func runServer(addr string) {
	staticFS, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	mux := http.NewServeMux()
	api.Routes(mux)
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	handler := loggingMiddleware(mux)

	log.Printf("🗜️  Huffman Compressor server starting on %s", addr)
	log.Printf("   → open http://localhost%s in your browser", normalizeAddr(addr))
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func normalizeAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return addr
	}
	return ":" + addr
}
