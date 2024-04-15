package middleware

import (
	"net/http"
)

type ContextKey string

type Middleware func(next http.Handler) http.Handler

func UseMiddleware(handler http.Handler, middlewares ...Middleware) http.Handler {
	chained := handler

	for i := len(middlewares) - 1; i > -1; i-- {
		chained = middlewares[i](chained)
	}

	// append cors middleware into the chain
	// if corsOptions != nil {
	// 	fmt.Println(*corsOptions)
	// 	chained = cors.New(*corsOptions).Handler(chained)
	// }

	return chained
}
