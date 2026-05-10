package logger

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware injects a request-scoped logger into ctx and logs req/resp.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := uuid.New().String()
			l := global.With().
				Str("req_id", reqID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Logger()

			ctx := WithContext(r.Context(), &l)
			l.Debug().Msg("request started")

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r.WithContext(ctx))

			level := zerolog.InfoLevel
			if rec.status >= 500 {
				level = zerolog.ErrorLevel
			}
			l.WithLevel(level).
				Int("status", rec.status).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("request done")
		})
	}
}
