package middleware

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"

	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/jwtutil"
)

var ClaimsKey ContextKey = "claims"
var TokenKey ContextKey = "token"

var AUTH_VERIFY_TOKEN_ENDPOINT = "v1/auth/token/verify"

type ServiceAPI struct {
	Host   string `json:"host"`
	Port   int    `json:"port,string"`
	Scheme string `json:"scheme"`
}

func AuthMiddleware(authAPI ServiceAPI, tlsConfig *tls.Config, metadata *httpx.Metadata) Middleware {
	return func(next http.Handler) http.Handler {
		fn := func(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
			var token string

			endpoint := r.Context().Value(EndpointKey).(string)

			auth := r.Header.Get("Authorization")

			if auth == "" {
				return responseerror.CreateBadRequestError(
					responseerror.EmptyAuthHeader,
					responseerror.EmptyAuthHeaderMessage,
					nil,
				)
			}

			if authType, authValue, _ := strings.Cut(auth, " "); authType != "Bearer" {
				return responseerror.CreateUnauthorizedError(
					responseerror.InvalidAuthHeader,
					responseerror.InvalidAuthHeaderMessage,
					map[string]string{
						"authType": authType,
					},
				)

			} else {
				token = authValue
			}

			req := &httpx.HTTPRequest{}
			req, err := req.CreateRequest(
				authAPI.Scheme,
				authAPI.Host,
				authAPI.Port,
				AUTH_VERIFY_TOKEN_ENDPOINT,
				http.MethodPost,
				http.StatusOK,
				struct {
					Token    string `json:"token"`
					Endpoint string `json:"endpoint"`
				}{
					Token:    token,
					Endpoint: endpoint,
				},
				tlsConfig,
			)

			if err != nil {
				return err
			}

			claims := &jwtutil.Claims{}
			err = req.Send(claims)

			if err != nil {
				return err
			}

			// send token and claims into the next middleware chain
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			ctx = context.WithValue(ctx, TokenKey, token)

			next.ServeHTTP(w, r.WithContext(ctx))
			return nil
		}

		return &httpx.Handler{
			Metadata: metadata,
			Handler:  fn,
		}
	}
}
