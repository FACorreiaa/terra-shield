// Package dataset downloads pinned reference files (country borders,
// MeteoAlarm warning areas) and verifies them, so a moved or altered file
// fails loudly instead of loading wrong shapes.
package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Pin is one reference file at a fixed version. Changing the file means
// changing URL and SHA256 together.
type Pin struct {
	Name     string // key in geo_datasets
	URL      string // immutable: a tag or commit, never a branch
	SHA256   string // hex
	MaxBytes int64  // download cap, with headroom over today's size
}

// Read returns the pinned file from path when given (offline), else
// downloads it. Both paths verify the checksum.
func (p Pin) Read(ctx context.Context, client *http.Client, path string) ([]byte, error) {
	if path == "" {
		return p.Fetch(ctx, client)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.Name, err)
	}
	if err := Verify(body, p.SHA256); err != nil {
		return nil, fmt.Errorf("%s: %s: %w", p.Name, path, err)
	}
	return body, nil
}

// Fetch downloads the pinned file and verifies it.
func (p Pin) Fetch(ctx context.Context, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "shield-seed/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", p.URL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, p.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > p.MaxBytes {
		return nil, fmt.Errorf("GET %s: body over %d bytes", p.URL, p.MaxBytes)
	}
	if err := Verify(body, p.SHA256); err != nil {
		return nil, fmt.Errorf("GET %s: %w", p.URL, err)
	}
	return body, nil
}

// Verify checks body against a hex SHA-256.
func Verify(body []byte, wantSHA256 string) error {
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != wantSHA256 {
		return fmt.Errorf("sha256 %s, want %s", got, wantSHA256)
	}
	return nil
}
