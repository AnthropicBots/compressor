# Legacy C++ CLI (original version)

This folder holds the **original command-line implementation** the project
started from, kept for reference/history. It only handled plain text files
via a simple `1/2/3` menu.

The active project now lives in [`/backend`](../backend) — a complete
rewrite in **Go** with a JSON API, a byte-safe codec that losslessly
compresses *any* file type, and a full web dashboard in [`/backend/web`](../backend/web).

Build & run the old version, if you're curious:

```bash
g++ src/*.cpp main.cpp -o build/app
./build/app
```
