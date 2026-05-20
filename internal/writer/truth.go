package writer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/humzakhan/fauxbooks/internal/gen"
)

// truthJSON is the typed struct written to truth.json.
// The Records field holds the sorted TruthLog. Using a wrapper struct (rather
// than marshalling the slice directly) gives the file a named root object.
type truthJSON struct {
	Records gen.TruthLog `json:"records"`
}

// WriteTruth writes ds.Truth to outDir/truth.json and returns the row count
// and SHA-256 hex digest. The TruthLog must already be sorted by the caller
// (gen.InjectStress sorts it before returning). Returns 0, "", nil when
// ds.Truth is nil (non-stress bundles).
func WriteTruth(outDir string, ds gen.Dataset) (rowCount int, hexHash string, err error) {
	if ds.Truth == nil {
		return 0, "", nil
	}

	path := filepath.Join(outDir, "truth.json")
	f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if ferr != nil {
		return 0, "", fmt.Errorf("open truth.json: %w", ferr)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close truth.json: %w", cerr)
		}
	}()

	bw := bufio.NewWriterSize(f, csvBufSize)
	tee, sum := newHashingWriter(bw)
	defer func() {
		if ferr := bw.Flush(); ferr != nil && err == nil {
			err = fmt.Errorf("flush truth.json: %w", ferr)
		}
	}()

	count, werr := writeTruthTo(tee, ds.Truth)
	if werr != nil {
		return 0, "", fmt.Errorf("write truth.json: %w", werr)
	}

	// Flush must happen before sum() to ensure all buffered bytes are hashed.
	if ferr := bw.Flush(); ferr != nil {
		return 0, "", fmt.Errorf("flush truth.json for hash: %w", ferr)
	}
	hexHash = sum()
	return count, hexHash, nil
}

// writeTruthTo encodes the TruthLog into w as indented JSON and returns the
// number of records written. It is separated from WriteTruth to allow testing
// against an in-memory io.Writer without touching the filesystem.
func writeTruthTo(w io.Writer, log gen.TruthLog) (int, error) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(truthJSON{Records: log}); err != nil {
		return 0, fmt.Errorf("encode truth.json: %w", err)
	}
	return len(log), nil
}
