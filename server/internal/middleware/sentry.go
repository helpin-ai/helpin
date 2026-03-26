package middleware

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

var sentryHTTPHandler = sentryhttp.New(sentryhttp.Options{
	Repanic: true,
})

func SentryHTTP(next http.Handler) http.Handler {
	return sentryHTTPHandler.Handle(next)
}

func SentryRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := sentry.GetHubFromContext(r.Context())
		if hub != nil {
			scope := hub.Scope()
			if scope != nil {
				if requestID := chimiddleware.GetReqID(r.Context()); requestID != "" {
					scope.SetTag("request_id", requestID)
				}
				if workspaceID := GetWorkspaceID(r.Context()); workspaceID != "" {
					scope.SetTag("workspace_id", workspaceID)
				}
				if userID := GetUserID(r.Context()); userID != "" {
					scope.SetUser(sentry.User{ID: userID})
					scope.SetTag("user_id", userID)
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
