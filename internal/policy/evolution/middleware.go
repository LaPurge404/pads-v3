package evolution

import (
    "log/slog"
    "net/http"
    "time"
)

// LoggingMiddleware enregistre chaque requête avec sa durée et son statut.
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        next(lrw, r)
        slog.Info("requête",
            "method", r.Method,
            "path", r.URL.Path,
            "duration", time.Since(start).String(),
            "status", lrw.statusCode,
        )
    }
}

type loggingResponseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
    lrw.statusCode = code
    lrw.ResponseWriter.WriteHeader(code)
}
