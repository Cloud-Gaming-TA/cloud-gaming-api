package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/AdityaP1502/Instant-Messanging/api/database"
	"github.com/AdityaP1502/Instant-Messanging/api/date"
	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/middleware"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
	"github.com/AdityaP1502/Instant-Messanging/api/service/account/config"
	"github.com/AdityaP1502/Instant-Messanging/api/service/account/otp"
	"github.com/AdityaP1502/Instant-Messanging/api/service/account/payload"
	"github.com/AdityaP1502/Instant-Messanging/api/service/account/pwdutil"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
)

var querynator = &database.Querynator{}

type RegisterResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginResponse struct {
	Status string `json:"status"`
	Token  Token  `json:"token"`
}

type GenericResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

var AUTH_ISSUE_TOKEN_ENDPOINT string = "v1/auth/token/issue"
var SEND_MAIL_ENDPOINT string = "mail/send"
var AUTH_REVOKE_TOKEN_ENDPOINT string = "v1/auth/token/revoke"

// func sendMailHTTP(message string, subject string, to string, url string) error {
// 	//TODO: Send http request to node js server

// 	var client = &http.Client{}

// 	var mail struct {
// 		To      string `json:"to"`
// 		Subject string `json:"subject"`
// 		Message string `json:"message"`
// 	}

// 	mail.To = to
// 	mail.Subject = subject
// 	mail.Message = message

// 	json, err := jsonutil.CreateJSONResponse(&mail)

// 	if err != nil {
// 		return err
// 	}

// 	r, err := http.NewRequest("POST", url, bytes.NewReader(json))

// 	if err != nil {
// 		return err
// 	}

// 	r.Header.Set("Content-Type", "application/json")

// 	resp, err := client.Do(r)

// 	if err != nil {
// 		return err
// 	}

// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		return fmt.Errorf("failed to send mail. mail server return with status code %d", resp.StatusCode)
// 	}

// 	return nil
// }

func registerHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	var req *httpx.HTTPRequest

	cf := metadata.Config.(*config.Config)

	body := r.Context().Value(middleware.PayloadKey).(*payload.Account)

	// Check username and email exist or not
	exist, err := querynator.IsExists(&payload.Account{Email: body.Email}, metadata.DB, "account")

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if exist {
		return responseerror.CreateBadRequestError(
			responseerror.EmailExists,
			responseerror.EmailsExistMessage,
			nil,
		)
	}

	exist, err = querynator.IsExists(&payload.Account{Username: body.Username}, metadata.DB, "account")
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if exist {
		return responseerror.CreateBadRequestError(
			responseerror.UsernameExists,
			responseerror.UsernameExistMessage,
			nil,
		)
	}

	newUser, err := payload.NewRegisteredAccountPayload(body.Username, body.Name, body.Email, body.Password, cf.Hash.SecretKeyRaw)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	tx, err := sqlx.NewDb(metadata.DB, cf.Database.Driver).Beginx()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	_, err = querynator.Insert(newUser, tx, "account", "account_id")
	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	otpData, err := payload.NewOTPPayload(body.Email, cf.OTP.OTPDurationMinutes)

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	_, err = querynator.Insert(otpData, tx, "user_otp", "otp_id")

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	// Create an API Call to mail service
	req = &httpx.HTTPRequest{}
	req, err = req.CreateRequest(
		cf.Services.Mail.Scheme,
		cf.Services.Mail.Host,
		cf.Services.Mail.Port,
		SEND_MAIL_ENDPOINT,
		http.MethodPost,
		http.StatusOK,
		struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
			Message string `json:"message"`
		}{
			To:      body.Email,
			Subject: "Email Verification",
			Message: fmt.Sprintf("Dont share this with anyone. This is your OTP %s. Your token will expired in %d minutes",
				otpData.OTP, cf.OTP.OTPDurationMinutes),
		},
		cf.Config,
	)

	if err != nil {
		tx.Rollback()
		return err.(responseerror.HTTPCustomError)
	}

	err = req.Send(nil)
	if err != nil {
		tx.Rollback()
		return err.(responseerror.HTTPCustomError)
	}

	resp := &RegisterResponse{
		Status:  "success",
		Message: "your account is successfully created. OTP has been sent to your email.",
	}

	jsonResponse, err := jsonutil.EncodeToJson(resp)

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	tx.Commit()

	w.WriteHeader(200)
	w.Write(jsonResponse)

	return nil
}

func loginHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	cf := metadata.Config.(*config.Config)

	body := r.Context().Value(middleware.PayloadKey).(*payload.Account)

	// Grab password and salt from db associated with username
	user := &payload.Account{}
	err := querynator.FindOne(&payload.Account{Email: body.Email}, user, metadata.DB, "account", "username", "password", "password_salt", "is_active")

	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return responseerror.CreateUnauthenticatedError(
			responseerror.InvalidCredentials,
			responseerror.InvalidCredentialsMessage,
			nil,
		)
	default:
		return responseerror.CreateInternalServiceError(err)
	}

	if isMatch, err := pwdutil.CheckPassword(body.Password, user.Salt, user.Password, cf.Hash.SecretKeyRaw); err != nil {
		return responseerror.CreateInternalServiceError(err)
	} else if !isMatch {
		return responseerror.CreateUnauthenticatedError(
			responseerror.InvalidCredentials,
			responseerror.InvalidCredentialsMessage,
			nil,
		)
	} else if user.IsActive == strconv.FormatBool(false) {
		return responseerror.CreateUnauthenticatedError(
			responseerror.UserMarkedInActive,
			responseerror.UserMarkedInActiveMessage,
			nil,
		)
	}

	// user is good and dandy
	req := &httpx.HTTPRequest{}
	req, err = req.CreateRequest(
		cf.Services.Auth.Scheme,
		cf.Services.Auth.Host,
		cf.Services.Auth.Port,
		AUTH_ISSUE_TOKEN_ENDPOINT,
		http.MethodPost,
		http.StatusOK,
		struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Roles    string `json:"roles"`
		}{
			Username: user.Username,
			Email:    body.Email,
			Roles:    "user",
		},
		cf.Config,
	)

	if err != nil {
		return err.(responseerror.HTTPCustomError)
	}

	token := Token{}
	err = req.Send(&token)

	if err != nil {
		return err.(responseerror.HTTPCustomError)
	}

	json, err := jsonutil.EncodeToJson(&LoginResponse{Status: "success", Token: token})

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	return nil
}

func resendOTPHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	// TODO: Use Transaction when inserting data or update data into the database
	cf := metadata.Config.(*config.Config)
	body := r.Context().Value(middleware.PayloadKey).(*payload.UserOTP)

	// check if confirmation id exists
	u := &payload.UserOTP{}
	err := querynator.FindOne(&payload.UserOTP{Email: body.Email, MarkedForDeletion: strconv.FormatBool(false)}, u, metadata.DB, "user_otp",
		"otp_id",
		"last_resend",
	)

	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return responseerror.CreateNotFoundError(map[string]string{
			"resourceName": "email",
		})
	default:
		return responseerror.CreateInternalServiceError(err)
	}

	// Check last resend duration
	t, err := date.ParseTimestamp(u.LastResend)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	duration := date.MinutesDifferenceFronNow(t)

	if duration < cf.OTP.ResendDurationMinutes {
		return responseerror.CreateTooManyRequestError(
			responseerror.ResendIntervalNotReachedErr,
			responseerror.ResendIntervalNotReachedMessage,
			nil,
		)
	}

	otp, err := otp.GenerateOTP()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	req := &httpx.HTTPRequest{}
	req, err = req.CreateRequest(
		cf.Services.Mail.Scheme,
		cf.Services.Mail.Host,
		cf.Services.Mail.Port,
		SEND_MAIL_ENDPOINT,
		http.MethodPost,
		http.StatusOK,
		struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
			Message string `json:"message"`
		}{
			To:      body.Email,
			Subject: "Email Verification",
			Message: fmt.Sprintf("Dont share this with anyone. This is your OTP %s. Your token will expired in %d minutes",
				otp, cf.OTP.OTPDurationMinutes),
		},
		cf.Config,
	)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	err = req.Send(nil)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	json, err := jsonutil.EncodeToJson(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{Status: "success", Message: "OTP has been re-send to your email."})

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	// Update the otp
	err = querynator.Update(&payload.UserOTP{
		OTP:        otp,
		LastResend: date.GenerateTimestamp(),
		ExpiredAt:  date.GenerateTimestampWithOffset(cf.OTP.ResendDurationMinutes),
	},
		[]string{"otp_id"}, []any{u.OTPID}, metadata.DB, "user_otp",
	)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	return nil
}

func verifyOTPHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	var validOTP = &payload.UserOTP{}

	body := r.Context().Value(middleware.PayloadKey).(*payload.UserOTP)

	err := querynator.FindOne(&payload.UserOTP{
		Email:             body.Email,
		MarkedForDeletion: strconv.FormatBool(false)},
		validOTP, metadata.DB, "user_otp", "otp", "otp_id", "expired_at",
	)

	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return responseerror.CreateNotFoundError(map[string]string{"resourceName": "email"})
	default:
		return responseerror.CreateInternalServiceError(err)
	}

	if validOTP.OTP != body.OTP {
		// otp is wrong
		return responseerror.CreateBadRequestError(
			responseerror.OTPInvalid,
			responseerror.OTPInvalidMessage,
			nil,
		)
	}

	// Check otp valid date
	otpExpiredAt, err := date.ParseTimestamp(validOTP.ExpiredAt)
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	} else if date.SecondsDifferenceFromNow(otpExpiredAt) > 0 {
		return responseerror.CreateBadRequestError(
			responseerror.OTPExpired,
			responseerror.OTPExpiredMessage,
			nil,
		)
	}

	json, err := jsonutil.EncodeToJson(&GenericResponse{Status: "success", Message: "your account has been activated successfully"})

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	// otp is correct, update user to be an active user, marked otp entry, and add token to revoked list

	// create sqlx connection
	sqlxDb := sqlx.NewDb(metadata.DB, "postgres")
	tx, err := sqlxDb.Beginx()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	err = querynator.Update(&payload.Account{IsActive: strconv.FormatBool(true)}, []string{"email"}, []any{body.Email}, tx, "account")
	if err != nil {
		rollError := tx.Rollback()
		fmt.Println(rollError.Error())
		return responseerror.CreateInternalServiceError(err)
	}

	err = querynator.Update(&payload.UserOTP{MarkedForDeletion: strconv.FormatBool(true)}, []string{"otp_id"}, []any{validOTP.OTPID}, tx, "user_otp")

	if err != nil {
		rollError := tx.Rollback()
		fmt.Println(rollError.Error())
		return responseerror.CreateInternalServiceError(err)
	}

	err = tx.Commit()

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	return nil
}

func linkSteamAccountHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	vars := mux.Vars(r)
	username := vars["username"]
	body := r.Context().Value(middleware.PayloadKey).(*payload.Account)

	// // TODO: Check if username exist and steamid is linkd
	// isExist, err := querynator.IsExists(&payload.Account{Username: username}, db, "account")

	// if err != nil {
	// 	return responseerror.CreateInternalServiceError(err)
	// }

	// if !isExist {
	// 	return responseerror.CreateNotFoundError(map[string]string{
	// 		"resourceName": "username",
	// 	})
	// }

	user := &payload.Account{}

	err := querynator.FindOne(&payload.Account{Username: username}, user, metadata.DB, "account", "steamid")

	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return responseerror.CreateNotFoundError(map[string]string{
			"resourceName": "username",
		})
	default:
		return responseerror.CreateInternalServiceError(err)
	}

	if user.SteamID != "not_linked" {
		return responseerror.CreateBadRequestError(
			responseerror.SteamAlreadyLinked,
			responseerror.SteamAlreadyLinkedMessage,
			nil,
		)
	}

	sqlxDb := sqlx.NewDb(metadata.DB, "postgres")
	tx, err := sqlxDb.Beginx()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	err = querynator.Update(body, []string{"username"}, []any{username}, tx, "account")

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	json, err := jsonutil.EncodeToJson(&GenericResponse{
		Status:  "success",
		Message: "account is linked successfully",
	})

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	tx.Commit()

	return nil
}

func rollbackSteamLinkHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	vars := mux.Vars(r)
	username := vars["username"]

	// check if username exist
	isExist, err := querynator.IsExists(&payload.Account{Username: username}, metadata.DB, "account")

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if !isExist {
		return responseerror.CreateNotFoundError(map[string]string{
			"resourceName": "username",
		})
	}

	sqlxDb := sqlx.NewDb(metadata.DB, "postgres")
	tx, err := sqlxDb.Beginx()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	err = querynator.UpdateUsingColumnNames([]string{"steamid"}, []any{"not_linked"}, []string{"username"}, []any{username}, tx, "account")
	// err = querynator.Update(&payload.Account{SteamID: ""}, []string{"username"}, []any{username}, tx, "account")

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	json, err := jsonutil.EncodeToJson(&GenericResponse{
		Status:  "success",
		Message: "account has been unlinked from steam",
	})

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	tx.Commit()

	return nil
}

func getUserSteamIDHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	vars := mux.Vars(r)
	username := vars["username"]

	user := &payload.Account{}

	err := querynator.FindOne(&payload.Account{Username: username}, user, metadata.DB, "account", "steamid")

	if err != nil {
		if err == sql.ErrNoRows {
			return responseerror.CreateNotFoundError(
				map[string]string{
					"resourceName": "username",
				},
			)
		}

		return responseerror.CreateInternalServiceError(err)
	}

	if user.SteamID == "not_linked" {
		return responseerror.CreateBadRequestError(
			responseerror.SteamNotLinked,
			responseerror.SteamNotLinkedMessage,
			nil,
		)
	}

	json, err := jsonutil.EncodeToJson(
		struct {
			Status  string `json:"status"`
			SteamID string `json:"steamid"`
		}{
			Status:  "success",
			SteamID: user.SteamID,
		},
	)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.WriteHeader(200)
	w.Write(json)

	return nil

}

// Register account subrouter
func SetAccountRoute(r *mux.Router, db *sql.DB, config *config.Config) {
	subrouter := r.PathPrefix("/account").Subrouter()

	subrouter.Use(middleware.RouteGetterMiddleware)

	certMiddleware := middleware.CertMiddleware(config.RootCAs)

	// Create middleware here
	userPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.Account{}, "Username", "Name", "Email", "Password")

	if err != nil {
		log.Fatal(err)
	}

	otpPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.UserOTP{}, "Email", "OTP")

	if err != nil {
		log.Fatal(err)
	}

	otpResendPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.UserOTP{}, "Email")

	if err != nil {
		log.Fatal(err)
	}

	loginPayloadMIddleware, err := middleware.PayloadCheckMiddleware(&payload.Account{}, "Email", "Password")

	if err != nil {
		log.Fatal(err)
	}

	linkSteamPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.Account{}, "SteamID")

	if err != nil {
		log.Fatal(err)
	}

	// REGISTER ROUTE //
	register := httpx.CreateHTTPHandler(db, config, registerHandler)

	subrouter.Handle("/register", middleware.UseMiddleware(db, config, register,
		userPayloadMiddleware)).Methods("POST")

	// VERIFY OTP ROUTE //
	// verifyOTP := &httpx.Handler{
	// 	DB:      db,
	// 	Config:  config,
	// 	Handler: verifyOTPHandler,
	// }

	verifyOTP := httpx.CreateHTTPHandler(db, config, verifyOTPHandler)
	subrouter.Handle("/otp/verify", middleware.UseMiddleware(db, config, verifyOTP,
		otpPayloadMiddleware)).Methods("POST")

	// RESEND OTP ROUTE //
	// resendOTP := &httpx.Handler{
	// 	DB:      db,
	// 	Config:  config,
	// 	Handler: resendOTPHandler,
	// }

	resendOTP := httpx.CreateHTTPHandler(db, config, resendOTPHandler)
	subrouter.Handle("/otp/send", middleware.UseMiddleware(db, config, resendOTP,
		otpResendPayloadMiddleware)).Methods("POST")

	// LOGIN ROUTE //
	// login := &httpx.Handler{
	// 	DB:      db,
	// 	Config:  config,
	// 	Handler: loginHandler,
	// }

	login := httpx.CreateHTTPHandler(db, config, loginHandler)

	subrouter.Handle("/login", middleware.UseMiddleware(db, config, login,
		loginPayloadMIddleware)).Methods("POST")

	linkSteamId := httpx.CreateHTTPHandler(db, config, linkSteamAccountHandler)
	subrouter.Handle("/{username}/steam", middleware.UseMiddleware(db, config, linkSteamId, certMiddleware, linkSteamPayloadMiddleware)).Methods("POST")

	getSteamId := httpx.CreateHTTPHandler(db, config, getUserSteamIDHandler)
	subrouter.Handle("/{username}/steam", getSteamId).Methods("GET")

	rollbackSteamID := httpx.CreateHTTPHandler(db, config, rollbackSteamLinkHandler)
	subrouter.Handle("/{username}/steam", middleware.UseMiddleware(db, config, rollbackSteamID, certMiddleware)).Methods("DELETE")
	// subrouter.Handle("/{username}/steam", rollbackSteamID).Methods("DELETE")

	// subrouter.HandleFunc("/logout", logOutHandler).Methods("POST")
	// subrouter.HandleFunc("/{username}", patchUserInfoHandler).Methods("PATCH")

}
