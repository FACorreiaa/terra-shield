package dataset_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shield/internal/geo/dataset"
)

func sum(b string) string {
	s := sha256.Sum256([]byte(b))
	return hex.EncodeToString(s[:])
}

func TestFetchVerifiesChecksumAndSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	defer srv.Close()
	pin := dataset.Pin{Name: "t", URL: srv.URL, SHA256: sum("original"), MaxBytes: 1 << 10}

	if _, err := pin.Fetch(t.Context(), srv.Client()); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err %v, want checksum mismatch", err)
	}
	pin.SHA256 = sum("tampered")
	if body, err := pin.Fetch(t.Context(), srv.Client()); err != nil || string(body) != "tampered" {
		t.Fatalf("body %q err %v", body, err)
	}
	pin.MaxBytes = 4
	if _, err := pin.Fetch(t.Context(), srv.Client()); err == nil || !strings.Contains(err.Error(), "over 4 bytes") {
		t.Fatalf("err %v, want size cap", err)
	}
}

func TestReadFromFileVerifies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.json")
	if err := os.WriteFile(path, []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	pin := dataset.Pin{Name: "t", SHA256: sum("local"), MaxBytes: 1 << 10}
	if body, err := pin.Read(t.Context(), nil, path); err != nil || string(body) != "local" {
		t.Fatalf("body %q err %v", body, err)
	}
	pin.SHA256 = sum("other")
	if _, err := pin.Read(t.Context(), nil, path); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err %v, want checksum mismatch", err)
	}
}
