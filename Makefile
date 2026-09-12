.PHONY: run build test fmt vet clean cli

BIN := backend/bin/huffman-compressor

run:
	cd backend && go run . -addr :8080

build:
	mkdir -p backend/bin
	cd backend && go build -o bin/huffman-compressor .

# Example CLI usage after `make build`:
#   ./backend/bin/huffman-compressor compress   file.txt
#   ./backend/bin/huffman-compressor decompress file.huff
#   ./backend/bin/huffman-compressor analyze    file.bin
cli: build
	@echo "Try: ./backend/bin/huffman-compressor help"

test:
	cd backend && go test ./... -v

fmt:
	cd backend && gofmt -w .

vet:
	cd backend && go vet ./...

clean:
	rm -rf backend/bin
