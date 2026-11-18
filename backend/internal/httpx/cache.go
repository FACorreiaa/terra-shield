package httpx

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

// ETag returns a strong validator for content: the first 128 bits of its
// SHA-256, quoted.
func ETag(content []byte) string {
	sum := sha256.Sum256(content)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// Cached writes body with an ETag and a public Cache-Control, or a bare
// 304 when the request's If-None-Match already holds etag. etag should
// cover only what identifies the content, so per-request fields such as
// generatedAt do not defeat revalidation.
func Cached(c *echo.Context, contentType string, body []byte, etag string, maxAge time.Duration) error {
	h := c.Response().Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int(maxAge/time.Second)))
	if matches(c.Request().Header.Get("If-None-Match"), etag) {
		return c.NoContent(http.StatusNotModified)
	}
	return c.Blob(http.StatusOK, contentType, body)
}

// matches applies If-None-Match's weak comparison (RFC 9110 §13.1.2).
func matches(header, etag string) bool {
	if header == "" {
		return false
	}
	want := strings.TrimPrefix(etag, "W/")
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == want {
			return true
		}
	}
	return false
}
