package main

import (
	_ "github.com/itchio/wharf/compressors/cbrotli"
	_ "github.com/itchio/wharf/compressors/zstd"
	_ "github.com/itchio/wharf/decompressors/cbrotli"
	_ "github.com/itchio/wharf/decompressors/zstd"

	_ "github.com/itchio/butler/archive/lzmasupport"
)
