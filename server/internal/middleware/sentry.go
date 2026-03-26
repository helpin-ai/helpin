package middleware

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

var sentryHTTPHandler = sentryhttp.New(sentryhttp.Options{
	Repanic: true,
})

func SentryHTTP(next http.Handler) http.Handler {
	return sentryHTTPHandler.Handle(next)
}
