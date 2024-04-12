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
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/jwtutil"
	"github.com/AdityaP1502/Instant-Messanging/api/service/session/config"
	"github.com/AdityaP1502/Instant-Messanging/api/service/session/payload"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	SESSION_MANAGER_HOST            string = ""
	SESSION_MANAGER_PORT            int    = 5000
	SESSION_MANAGER_CREATE_ENDPOINT        = "/create"
)

const (
	PROVISIONING           string = "Provisioning"
	WAITING_FOR_CONNECTION string = "WaitingForConnection"
	PAIRING                string = "Pairing"
	RUNNING                string = "Running"
	FAILED                 string = "Failed"
	TERMINATED             string = "Terminated"
)

var querynator = database.Querynator{
	DriverName: "postgres",
}

func createNewSessionHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	// cf := conf.(*config.Config)
	body := r.Context().Value(middleware.PayloadKey).(*payload.UserSession)

	uuidv7, err := uuid.NewV7()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	sessionId, err := uuidv7.MarshalBinary()
	sessionIdString := uuidv7.String()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	// Set additional information
	body.SessionID = sessionId
	body.LastUpdatedAt = date.GenerateTimestamp()
	body.MarkedForDeletion = strconv.FormatBool(false)
	body.RequestStatus = PROVISIONING

	body.SessionMetadata.SessionID = sessionId
	body.SessionMetadata.GameLocation.Server.Host = fmt.Sprintf("%s:%d",
		body.SessionMetadata.GameLocation.Server.IP,
		body.SessionMetadata.GameLocation.Server.Port,
	)
	body.SessionMetadata.CreatedAt = date.GenerateTimestamp()

	// Insert data isnto database
	tx, err := metadata.DB.Begin()
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	// only insert the root file on the body
	_, err = querynator.Insert(body, tx, "user_session", "")

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	// Insert the metadata into the database
	_, err = querynator.Insert(body.SessionMetadata, tx, "session_metadata", "")
	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	// req := &httpx.HTTPRequest{}
	// req, err_ := req.CreateRequest(
	// 	"http",
	// 	cf.Service.SessionManager.Host,
	// 	cf.Service.SessionManager.Port,
	// 	SESSION_MANAGER_CREATE_ENDPOINT,
	// 	http.MethodPost,
	// 	200,
	// 	body,
	// 	cf.Config,
	// )

	// if err_ != nil {
	// 	tx.Rollback()
	// 	return responseerror.CreateInternalServiceError(err_)
	// }

	// err_ = req.Send(nil)

	// // Propagate the error to the user
	// if err_ != nil {
	// 	tx.Rollback()
	// 	w.WriteHeader(req.ReturnedStatusCode)
	// 	w.Write(req.Payload)
	// 	return nil
	// }

	tmp := struct {
		Status    string `json:"status"`
		SessionID string `json:"session_id"`
	}{
		Status:    "success",
		SessionID: sessionIdString,
	}

	json, err := jsonutil.EncodeToJson(tmp)

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	w.Write(json)
	tx.Commit()

	return nil

}

func getRequestStatus(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	vars := mux.Vars(r)

	claims := r.Context().Value(middleware.ClaimsKey).(*jwtutil.Claims)

	sessionIDstring := vars["session_id"]
	uuidv7, err := uuid.Parse(sessionIDstring)

	if err != nil {
		return responseerror.CreateBadRequestError(responseerror.MalformedSessionID, responseerror.MalformedSessionIDMessage,
			map[string]string{
				"id": "session_id",
			},
		)
	}

	sessionID, err := uuidv7.MarshalBinary()
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	// err = querynator.FindOne(&payload.UserSession{
	// 	SessionID:         sessionID,
	// 	MarkedForDeletion: strconv.FormatBool(false),
	// }, userSession, metadata.DB, "user_session", "username", "request_status", "last_updated")

	joinTables := []struct {
		payload.UserSession
		payload.SessionHost
	}{}

	joinExecutor := querynator.PrepareJoinOperation()
	joinExecutor.AddJoinTable("session_host", "session_id", "user_session", "session_id")

	err = joinExecutor.Find(metadata.DB, []database.QueryCondition{
		{TableName: "user_session", ColumnName: "session_id", MatchValue: sessionID, Operand: database.EQ},
	}, &joinTables, "user_session", database.LEFT_JOIN, map[string][]string{
		"user_session": {"username", "request_status", "last_update"},
		"session_host": {"network_id,string"},
	})

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if len(joinTables) < 1 {
		return responseerror.CreateNotFoundError(map[string]string{
			"resourceName": "session_id",
		})
	}

	if claims.Username != joinTables[0].Username {
		return responseerror.CreateUnauthorizedError(responseerror.AccessDenied, responseerror.AccessDeniedMesage, nil)
	}

	tmp := struct {
		Status        string `json:"status"`
		Username      string `json:"username"`
		LastUpdatedAt string `json:"last_updated"`
		RequestStatus string `json:"request_status"`
		NetworkID     string `json:"network_id"`
	}{
		Status:        "success",
		Username:      joinTables[0].Username,
		LastUpdatedAt: joinTables[0].LastUpdatedAt,
		RequestStatus: joinTables[0].RequestStatus,
		NetworkID:     joinTables[0].SessionHost.NetworkID,
	}

	json, err := jsonutil.EncodeToJson(tmp)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.Write(json)

	return nil
}

func startConnectionEstablishmentHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	vars := mux.Vars(r)
	sessionIDString := vars["session_id"]

	uuidv7, err := uuid.Parse(sessionIDString)

	if err != nil {
		return responseerror.CreateBadRequestError(responseerror.MalformedSessionID, responseerror.MalformedSessionIDMessage, map[string]string{
			"id": "session_id",
		})
	}

	sessionID, err := uuidv7.MarshalBinary()
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	body := r.Context().Value(middleware.PayloadKey).(*payload.SessionHost)
	body.SessionID_ = sessionID

	tx, err := metadata.DB.Begin()

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	_, err = querynator.Insert(body, tx, "session_host", "")
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	err = querynator.Update(&payload.UserSession{RequestStatus: WAITING_FOR_CONNECTION}, []string{"session_id"}, []any{sessionID}, tx, "user_session")
	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	tmp := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "success",
		Message: "instance has been attached successfully",
	}

	json, err := jsonutil.EncodeToJson(tmp)

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	w.Write(json)
	tx.Commit()
	return nil
}

func pairHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	cf := metadata.Config.(*config.Config)
	vars := mux.Vars(r)
	sessionIDString := vars["session_id"]

	uuidv7, err := uuid.Parse(sessionIDString)
	if err != nil {
		return responseerror.CreateBadRequestError(
			responseerror.MalformedSessionID,
			responseerror.MalformedSessionIDMessage,
			map[string]string{
				"id": "session_id",
			},
		)
	}

	sessionID, err := uuidv7.MarshalBinary()
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	body := r.Context().Value(middleware.PayloadKey).(*payload.SessionPIN)

	// get the host id
	host := &payload.SessionHost{}
	err = querynator.FindOne(&payload.SessionHost{SessionID_: sessionID}, host, metadata.DB, "session_host", "webhook_host")
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return responseerror.CreateNotFoundError(map[string]string{
			"resourceName": "session_id",
		})
	default:
		return responseerror.CreateInternalServiceError(err)
	}

	// update the status to Running
	tx, err := metadata.DB.Begin()
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	err = querynator.Update(&payload.UserSession{RequestStatus: RUNNING}, []string{"session_id"}, []any{sessionID}, tx, "user_session")
	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	port, err := strconv.Atoi(host.Webhook.Port)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	req := &httpx.HTTPRequest{}
	req, err = req.CreateRequest(
		"http",
		host.Webhook.Host,
		port,
		"/pin",
		http.MethodPost,
		200,
		body,
		cf.Config,
	)

	err = req.Send(nil)

	if err != nil {
		tx.Rollback()
		return responseerror.CreateBadRequestError(responseerror.InvalidPIN, responseerror.InvalidPINMessage, map[string]string{
			"pin": body.PIN,
		})

	}

	tmp := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "success",
		Message: "instance has been attached successfully",
	}

	json, err := jsonutil.EncodeToJson(tmp)

	if err != nil {
		tx.Rollback()
		return responseerror.CreateInternalServiceError(err)
	}

	w.Write(json)
	tx.Commit()
	return nil

}

func SetSessionRoute(r *mux.Router, db *sql.DB, conf *config.Config) {
	subrouter := r.PathPrefix("/session").Subrouter()

	subrouter.Use(middleware.RouteGetterMiddleware)

	authMiddleware := middleware.AuthMiddleware(conf.Service.Auth, conf.Config)
	certMiddleware := middleware.CertMiddleware(conf.RootCAs)

	createNewSessionPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.UserSession{},
		"Username",
		"SessionMetadata:GameID",
		"SessionMetadata:GameLocation:Protocol",
		"SessionMetadata:GameLocation:Path",
		"SessionMetadata:GameLocation:Server:IP",
	)

	if err != nil {
		log.Fatal(err)
	}

	startConnectionPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.SessionHost{},
		"Webhook:Port",
		"Webhook:Host",
		"NetworkID",
	)

	if err != nil {
		log.Fatal(err)
	}

	pinPairPayloadMiddleware, err := middleware.PayloadCheckMiddleware(&payload.SessionPIN{}, "PIN")

	if err != nil {
		log.Fatal(err)
	}

	createSession := httpx.CreateHTTPHandler(db, conf, createNewSessionHandler)
	subrouter.Handle("/create", middleware.UseMiddleware(db, conf, createSession, certMiddleware, createNewSessionPayloadMiddleware))

	getStatus := httpx.CreateHTTPHandler(db, conf, getRequestStatus)
	subrouter.Handle("/{session_id}/status", middleware.UseMiddleware(db, conf, getStatus, authMiddleware))

	startConnection := httpx.CreateHTTPHandler(db, conf, startConnectionEstablishmentHandler)
	subrouter.Handle("/{session_id}/connection/start", middleware.UseMiddleware(db, conf, startConnection, certMiddleware, startConnectionPayloadMiddleware))

	pair := httpx.CreateHTTPHandler(db, conf, pairHandler)
	subrouter.Handle("/{session_id}/pair", middleware.UseMiddleware(db, conf, pair, authMiddleware, pinPairPayloadMiddleware))
}
