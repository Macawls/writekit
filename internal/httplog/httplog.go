// Package httplog provides structured HTTP request logging and panic recovery
// middleware built on log/slog. A request-scoped logger is injected into the
// request context so downstream handlers can emit logs correlated by request_id.
package httplog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxKey int

const (
	loggerKey ctxKey = iota
	requestIDKey
)

// FromContext returns the request-scoped logger, or slog.Default if none is set.
// Handlers should prefer this over slog.Default so every log line includes
// the request_id and any fields added by middleware upstream.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// RequestID returns the request ID for the current request, or "" if unset.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithLogger returns a copy of ctx carrying the given logger. Useful for tests
// and for workers that want to propagate the request logger into goroutines.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithFields returns a new context whose logger has the given key/value pairs
// permanently attached. Use this to add tenant_id, user_id, etc. once you know them.
func WithFields(ctx context.Context, args ...any) context.Context {
	l := FromContext(ctx).With(args...)
	return WithLogger(ctx, l)
}

// RequestID middleware generates a short random ID for each incoming request,
// exposes it via the X-Request-Id header, and attaches a logger bound to that
// ID into the request context.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		l := slog.Default().With("request_id", id)
		ctx := context.WithValue(r.Context(), loggerKey, l)
		ctx = context.WithValue(ctx, requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Access middleware logs one line per request at Info (2xx/3xx), Warn (4xx),
// or Error (5xx), following the Google HTTP logging conventions: method, path,
// status, duration, bytes, remote, and user-agent. The status/bytes are observed
// via a lightweight ResponseWriter wrapper.
func Access(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		l := FromContext(r.Context())
		dur := time.Since(start)
		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", dur.Milliseconds(),
			"bytes", rw.bytes,
			"remote", clientIP(r),
			"host", r.Host,
		}
		if ua := r.UserAgent(); ua != "" {
			attrs = append(attrs, "user_agent", ua)
		}
		switch {
		case rw.status >= 500:
			l.LogAttrs(r.Context(), slog.LevelError, "http request", slogAttrs(attrs)...)
		case rw.status >= 400:
			l.LogAttrs(r.Context(), slog.LevelWarn, "http request", slogAttrs(attrs)...)
		default:
			l.LogAttrs(r.Context(), slog.LevelInfo, "http request", slogAttrs(attrs)...)
		}
	})
}

// Recoverer middleware catches panics, logs them with a stack trace, and returns
// a 500. Must be installed after RequestIDMiddleware so the stack trace carries
// the request ID.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				FromContext(r.Context()).Error("panic recovered",
					"err", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				if w.Header().Get("Content-Type") == "" {
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush preserves streaming (SSE) when the underlying writer supports it.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "noid"
	}
	return hex.EncodeToString(b)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xr := r.Header.Get("X-Real-Ip"); xr != "" {
		return xr
	}
	return r.RemoteAddr
}

func slogAttrs(kv []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, _ := kv[i].(string)
		out = append(out, slog.Any(key, kv[i+1]))
	}
	return out
}
