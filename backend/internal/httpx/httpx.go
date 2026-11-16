// Package httpx holds the HTTP conventions every handler shares: the error
// envelope and JSON responses.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
)

// Error is a handler error with an HTTP status and a client-facing reason.
type Error struct {
	Status int
	Code   string
	Reason string
	Err    error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Reason + ": " + e.Err.Error()
	}
	return e.Reason
}

func (e *Error) Unwrap() error { return e.Err }

// StatusCode lets echo's own helpers read the status.
func (e *Error) StatusCode() int { return e.Status }

// Abort builds an Error. An empty reason becomes the status text.
func Abort(status int, reason string) *Error {
	if reason == "" {
		reason = http.StatusText(status)
	}
	return &Error{Status: status, Reason: reason}
}

// BadRequest is a 400 with reason.
func BadRequest(reason string) *Error { return Abort(http.StatusBadRequest, reason) }

// NotFound is a 404 with reason.
func NotFound(reason string) *Error { return Abort(http.StatusNotFound, reason) }

// Envelope is the JSON body of every error response.
type Envelope struct {
	Error     bool   `json:"error"`
	Code      string `json:"code"`
	Reason    string `json:"reason"`
	RequestID string `json:"requestId,omitempty"`
}

// CodeForStatus is the envelope code for an HTTP status.
func CodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "unavailable"
	}
	if status >= 500 {
		return "internal_server_error"
	}
	return "request_failed"
}

// ErrorHandler renders every error as an Envelope. In production the
// message of an unexpected 5xx is hidden.
func ErrorHandler(logger *slog.Logger, production bool) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
			return
		}

		status := http.StatusInternalServerError
		var reason, code string

		var apiErr *Error
		var httpErr *echo.HTTPError
		switch {
		case errors.As(err, &apiErr):
			status, reason, code = apiErr.Status, apiErr.Reason, apiErr.Code
		case errors.As(err, &httpErr):
			status = httpErr.Code
			reason = httpErr.Message
		default:
			var sc echo.HTTPStatusCoder
			if errors.As(err, &sc) && sc.StatusCode() != 0 {
				status = sc.StatusCode()
			}
			if status < 500 || !production {
				reason = err.Error()
			}
		}
		if reason == "" {
			reason = http.StatusText(status)
		}
		if code == "" {
			code = CodeForStatus(status)
		}
		if status >= 500 {
			logger.Error("request failed", "status", status, "path", c.Request().URL.Path, "error", err)
		}

		if c.Request().Method == http.MethodHead {
			_ = c.NoContent(status)
			return
		}
		_ = JSON(c, status, Envelope{
			Error:     true,
			Code:      code,
			Reason:    reason,
			RequestID: c.Response().Header().Get(echo.HeaderXRequestID),
		})
	}
}

// StatusOf is the status a request ends with, as seen from middleware: the
// error's when the handler failed (the error handler has not written it
// yet), else the response's.
func StatusOf(c *echo.Context, err error) int {
	if err != nil {
		var sc echo.HTTPStatusCoder
		if errors.As(err, &sc) && sc.StatusCode() != 0 {
			return sc.StatusCode()
		}
		return http.StatusInternalServerError
	}
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Status != 0 {
		return r.Status
	}
	return http.StatusOK
}

// JSON writes v as application/json.
func JSON(c *echo.Context, status int, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Blob(status, "application/json; charset=utf-8", body)
}

// OK is JSON with 200.
func OK(c *echo.Context, v any) error { return JSON(c, http.StatusOK, v) }
