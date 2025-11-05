package middleware

import (
	"net/http"

	"time"

	"sob/pkg/session"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func AccessLog(logger *zap.SugaredLogger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Infow("New request",
				"method", r.Method,
				"remote_addr", r.RemoteAddr,
				"url", r.URL.Path,
				"time", time.Since(start),
			)
		})
	}
}

func Panic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, "Internal server error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Auth middleware - всегда проверяет сессию и добавляет в контекст если есть
func Auth(sm *session.SessionsManager) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Всегда пытаемся получить сессию
			sess, err := sm.Check(r)
			
			// Создаем новый контекст с сессией (даже если она nil)
			ctx := r.Context()
			if err == nil && sess != nil {
				ctx = session.ContextWithSession(ctx, sess)
			}
			
			// Обновляем запрос с новым контекстом
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth middleware - требует авторизацию для определенных маршрутов
func RequireAuth(sm *session.SessionsManager) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := sm.Check(r)
			if err != nil || sess == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			
			ctx := session.ContextWithSession(r.Context(), sess)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}