package routes

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AdityaP1502/Instant-Messanging/api/database"
	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/middleware"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/http/router"
	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/config"
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/jwtutil"
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/payload"
)

type RevokedToken struct {
	TokenID   string `db:"token_id"`
	Username  string `db:"username"`
	Token     string `db:"token"`
	TokenType string `db:"type"`
	ExpiredAt string `db:"expired_at"`
}

var querynator = &database.Querynator{}

func IssueTokenHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	var roles jwtutil.Roles
	var ok bool

	cf := metadata.Config.(*config.Config)

	body := r.Context().Value(middleware.PayloadKey).(*payload.Credentials)

	if roles, ok = jwtutil.ParseRoles(body.Roles); !ok {
		return responseerror.CreateBadRequestError(
			responseerror.PayloadInvalid,
			"cannot parse roles",
			nil,
		)
	}

	// Create a new token
	token := &payload.Token{}
	err := token.GenerateTokenPair(cf, body.Username, body.Email, roles)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	resp := &struct {
		Status string `json:"status"`
		*payload.Token
	}{
		Status: "success",
		Token:  token,
	}

	json, err := jsonutil.EncodeToJson(resp)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	return nil
}

func RefreshTokenHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	cf := metadata.Config.(*config.Config)

	body := r.Context().Value(middleware.PayloadKey).(*payload.Token)

	// Create a new token
	claims, err := body.CheckRefreshEligibility(cf)

	if err != nil {
		return err.(responseerror.HTTPCustomError)
	}

	// check if the token is revoked
	isExist, err := querynator.IsExists(&RevokedToken{Token: body.AccessToken}, metadata.DB, "revoked_token")

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if isExist {
		return responseerror.CreateUnauthorizedError(
			responseerror.InvalidToken,
			responseerror.InvalidTokenMessage,
			map[string]string{
				"description": "trying to refresh a revoked token",
			},
		)
	}

	newToken := &payload.Token{}

	err = newToken.GenerateTokenPair(cf, claims.Username, claims.Email, jwtutil.Roles(claims.Roles))
	if err != nil {
		return err.(responseerror.HTTPCustomError)
	}

	resp := &struct {
		Status string `json:"status"`
		*payload.Token
	}{
		Status: "success",
		Token:  newToken,
	}

	json, err := jsonutil.EncodeToJson(resp)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	return nil
}

func VerifyTokenHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	var err error
	cf := metadata.Config.(*config.Config)
	body := r.Context().Value(middleware.PayloadKey).(*payload.Access)

	// verify token
	claims, err := jwtutil.VerifyToken(body.AccessToken, cf.Session.SecretKeyRaw)

	if err != nil {
		return err.(responseerror.HTTPCustomError)
	}

	// TODO: check if current roles can access the endpoint
	json, err := jsonutil.EncodeToJson(&jwtutil.Claims{
		Email:    claims.Email,
		Username: claims.Username,
		Roles:    claims.Username,
	})

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.Write(json)
	w.WriteHeader(200)
	return nil
}

func RevokeTokenHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	var err error

	cf := metadata.Config.(*config.Config)
	body := r.Context().Value(middleware.PayloadKey).(*payload.Token)

	claims, err := jwtutil.VerifyToken(body.AccessToken, cf.Session.SecretKeyRaw)

	if err != nil {
		return err.(responseerror.HTTPCustomError)
	}

	fmt.Println(claims)

	_, err = querynator.Insert(&RevokedToken{
		Token:     body.AccessToken,
		TokenType: string(jwtutil.Auth),
		ExpiredAt: claims.RegisteredClaims.ExpiresAt.Format(time.RFC3339),
		Username:  claims.Username,
	}, metadata.DB, "revoked_token", "")

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)

	return nil
}

func SetAuthRoute(r *router.Routerx) {
	subrouter := r.PathPrefix("/auth").Subrouter()

	conf := subrouter.Metadata.Config.(*config.Config)
	credentialsPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.Credentials{}, r.Metadata,
		"Username", "Roles", "Email",
	)

	if err != nil {
		log.Fatal(err)
	}

	refreshpayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.Token{}, r.Metadata,
		"RefreshToken", "AccessToken",
	)

	if err != nil {
		log.Fatal(err)
	}

	revokepayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.Token{}, r.Metadata,
		"AccessToken",
	)

	if err != nil {
		log.Fatal(err)
	}

	certMiddleware := middleware.CertMiddleware(conf.RootCAs, r.Metadata)

	accesspayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.Access{}, r.Metadata,
		"Endpoint", "AccessToken",
	)

	if err != nil {
		log.Fatal(err)
	}

	// issueToken := httpx.CreateHTTPHandler(db, conf, IssueTokenHandler)

	// refreshToken := &httpx.Handler{
	// 	DB:      db,
	// 	Config:  conf,
	// 	Handler: httpx.HandlerLogic(RefreshTokenHandler),
	// }

	// refreshToken := httpx.CreateHTTPHandler(db, conf, RefreshTokenHandler)

	// verifyToken := httpx.CreateHTTPHandler(db, conf, VerifyTokenHandler)

	// revokeToken := httpx.CreateHTTPHandler(db, conf, RevokeTokenHandler)

	// subrouter.Handle("/token/issue", middleware.UseMiddleware(db, conf, issueToken, certMiddleware, credentialsPayloadMiddleware))
	// subrouter.Handle("/token/refresh", middleware.UseMiddleware(db, conf, refreshToken,
	// refreshpayloadMiddleware)).Methods("POST")

	// subrouter.Handle("/token/verify", middleware.UseMiddleware(db, conf, verifyToken, certMiddleware, accesspayloadMiddleware))
	// subrouter.Handle("/token/revoke", middleware.UseMiddleware(db, conf, revokeToken, certMiddleware, revokepayloadMiddleware))

	subrouter.Handle("/token/issue", IssueTokenHandler).UseMiddleware(certMiddleware, credentialsPayloadMiddleware).Methods("POST")
	subrouter.Handle("/token/refresh", RefreshTokenHandler).UseMiddleware(certMiddleware, refreshpayloadMiddleware).Methods("POST")
	subrouter.Handle("/token/revoke", RevokeTokenHandler).UseMiddleware(certMiddleware, revokepayloadMiddleware).Methods("POST")
	subrouter.Handle("/token/verify", VerifyTokenHandler).UseMiddleware(certMiddleware, accesspayloadMiddleware).Methods("POST")

}
