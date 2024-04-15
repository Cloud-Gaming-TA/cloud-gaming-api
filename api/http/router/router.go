package router

import (
	"net/http"

	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/middleware"
	"github.com/gorilla/mux"
)

type Routerx struct {
	Router   *mux.Router
	Metadata *httpx.Metadata
}

func (r *Routerx) Handle(path string, logic httpx.HandlerLogic) *Routex {
	handler := &httpx.Handler{
		Metadata: r.Metadata,
		Handler:  logic,
	}

	return &Routex{
		Route:    r.Router.Handle(path, handler),
		Metadata: r.Metadata,
	}
}

func (r *Routerx) PathPrefix(route string) *Routex {
	return &Routex{
		Route:       r.Router.PathPrefix(route),
		Metadata:    r.Metadata,
		middlewares: []middleware.Middleware{},
	}
}

func (r *Routerx) Use(mwf ...mux.MiddlewareFunc) {
	r.Router.Use(mwf...)
}

func CreateRouterx(metadata *httpx.Metadata) *Routerx {
	return &Routerx{
		Router:   mux.NewRouter(),
		Metadata: metadata,
	}
}

type Routex struct {
	Route       *mux.Route
	Metadata    *httpx.Metadata
	middlewares []middleware.Middleware
}

func (r *Routex) Subrouter() *Routerx {
	return &Routerx{
		Router:   r.Route.Subrouter(),
		Metadata: r.Metadata,
	}
}

func (r *Routex) Handler(handler http.Handler) *Routex {
	r.Route = r.Route.Handler(handler)
	return r
}

func (r *Routex) Methods(methods ...string) *Routex {
	r.Route = r.Route.Methods(methods...)

	return r
}

func (r *Routex) UseMiddleware(midddlewares ...middleware.Middleware) *Routex {
	r.middlewares = append(r.middlewares, midddlewares...)
	// attach the middlewares into the handler
	handler := middleware.UseMiddleware(r.Route.GetHandler(), midddlewares...)
	return r.Handler(handler)
}
