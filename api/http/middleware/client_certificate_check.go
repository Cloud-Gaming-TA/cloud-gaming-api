package middleware

import (
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
)

func CertMiddleware(rootCACerts *x509.CertPool) Middleware {
	return func(next http.Handler, db *sql.DB, conf interface{}) http.Handler {
		fn := func(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
			if r.TLS == nil {
				fmt.Println("restricted route called without TLS")
			}

			if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
				next.ServeHTTP(w, r)
				return nil
			}

			// client request is being redirected by the proxy
			if x := r.Header.Get("x-client-cert"); r.TLS != nil && x != "" {
				pemBytes, err := base64.StdEncoding.DecodeString(x)
				if err != nil {
					return responseerror.CreateUnauthorizedError(
						responseerror.AccessDenied,
						responseerror.MTLSFailureMessage,
						nil,
					)
				}

				pemBlock, _ := pem.Decode(pemBytes)
				if pemBlock == nil {
					return responseerror.CreateUnauthorizedError(
						responseerror.AccessDenied,
						responseerror.MTLSFailureMessage,
						nil,
					)
				}

				cert, err := x509.ParseCertificate(pemBlock.Bytes)

				if err != nil {
					return responseerror.CreateUnauthorizedError(
						responseerror.AccessDenied,
						responseerror.MTLSFailureMessage,
						nil,
					)
				}

				opts := x509.VerifyOptions{
					Roots:       rootCACerts,
					CurrentTime: time.Now(),
				}

				chains, err := cert.Verify(opts)

				if err != nil {
					return responseerror.CreateUnauthorizedError(
						responseerror.AccessDenied,
						responseerror.MTLSFailureMessage,
						nil,
					)
				}

				if len(chains) > 0 {
					next.ServeHTTP(w, r)
					return nil
				}

			}

			return responseerror.CreateUnauthorizedError(
				responseerror.AccessDenied,
				responseerror.MTLSFailureMessage,
				nil,
			)
		}

		return httpx.CreateHTTPHandler(db, conf, fn)
	}
}
