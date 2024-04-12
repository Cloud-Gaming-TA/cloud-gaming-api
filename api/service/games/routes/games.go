package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/AdityaP1502/Instant-Messanging/api/cache"
	"github.com/AdityaP1502/Instant-Messanging/api/database"
	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/middleware"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
	"github.com/AdityaP1502/Instant-Messanging/api/service/games/config"
	"github.com/AdityaP1502/Instant-Messanging/api/service/games/payload"
	"github.com/gorilla/mux"
)

const (
	CHECK_ACCOUNT_STEAM_LINK_ENDPOINT = "v1/account/%s/steam"
	CREATE_NEW_SESSION_ENDPOINT       = "v1/session/create"
)

var querynator = database.Querynator{
	DriverName: "postgres",
}

type GenericResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
type Server struct {
	IP   string `json:"ip"`
	Port int    `json:"port,omitempty"`
}
type GameLocation struct {
	Protocol string `json:"protocol"`
	Server   Server `json:"server"`
	Location string `json:"path"`
}
type SessionMetadata struct {
	GameID       int          `json:"game_id"`
	GameLocation GameLocation `json:"game_location"`
}

type SessionRequest struct {
	Username string          `json:"username"`
	Metadata SessionMetadata `json:"session_metadata"`
}

func syncUserGamesHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	body := r.Context().Value(middleware.PayloadKey).(*payload.Collections)

	// read username from path parameter
	vars := mux.Vars(r)
	username := vars["username"]

	filterExecutor := querynator.PrepareFilterOperation()
	err := filterExecutor.UseTransaction(metadata.DB)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	userGames := []*payload.UserGames{}

	for _, game := range body.Games {
		userGames = append(userGames, &payload.UserGames{
			Username: username,
			GameID:   game.GameID,
		})
	}

	filterExecutor.AddTableSource("games", "game_id", "game_id")
	filterExecutor.UseExplicitCast()
	err = filterExecutor.BatchInsert(userGames, metadata.DB, "user_games")

	if err != nil {
		if filterExecutor.Tx != nil {
			filterExecutor.Rollback()
		}

		return responseerror.CreateInternalServiceError(err)
	}

	json, err := jsonutil.EncodeToJson(&GenericResponse{
		Status:  "success",
		Message: "user game collections has been successfully synced",
	})

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	w.Write(json)
	w.WriteHeader(200)

	filterExecutor.Commit()

	return nil
}

func listGamesHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	cf := metadata.Config.(*config.Config)
	vars := mux.Vars(r)
	username := vars["username"]

	// check steamid
	req := &httpx.HTTPRequest{}
	req, err := req.CreateRequest(
		cf.Service.Account.Scheme,
		cf.Service.Account.Host,
		cf.Service.Account.Port,
		fmt.Sprintf(CHECK_ACCOUNT_STEAM_LINK_ENDPOINT, username),
		http.MethodGet,
		http.StatusOK,
		nil,
		cf.Config,
	)

	if err != nil {
		return err
	}

	tmp1 := struct {
		SteamID string `json:"steamid"`
	}{}

	err = req.Send(&tmp1)

	if err != nil {
		return err
	}

	cursorQ := r.URL.Query().Get("cursor")
	cursor, err_ := strconv.Atoi(cursorQ)

	if err_ != nil {
		cursor = 0
	}

	limitQ := r.URL.Query().Get("limit")
	limit, err_ := strconv.Atoi(limitQ)

	if err_ != nil {
		limit = cf.Pagination.DefaultLimit
	}

	limit = max(limit, cf.Pagination.DefaultLimit)

	joinResults := []payload.UserCollections{}

	// Query the database
	joinExecutor := querynator.PrepareJoinOperation()
	joinExecutor.AddJoinTable("games", "game_id", "user_games", "game_id")
	joinExecutor.SetLimit(limit)
	// joinExecutor.OrderBy("collections_id ", database.DESCENDING)
	err_ = joinExecutor.Find(metadata.DB, []database.QueryCondition{
		{TableName: "user_games", ColumnName: "collections_id", MatchValue: cursor, Operand: database.GT},
	}, &joinResults, "user_games", database.INNER_JOIN, map[string][]string{
		"user_games": {"collections_id"},
		"games":      {"game_id", "name", "display_picture_url"},
	})

	if err_ != nil {
		return responseerror.CreateInternalServiceError(err_)
	}

	var n int

	if len(joinResults) < 1 {
		n = cursor
	} else {
		// set new cursor
		n, err_ = strconv.Atoi(joinResults[len(joinResults)-1].CollectionsID)

		if err_ != nil {
			return responseerror.CreateInternalServiceError(err_)
		}
	}

	tmp2 := struct {
		Status      string                    `json:"status"`
		Cursor      int                       `json:"cursor"`
		Collections []payload.UserCollections `json:"games"`
	}{
		Status:      "success",
		Cursor:      n,
		Collections: joinResults,
	}

	json, err_ := jsonutil.EncodeToJson(&tmp2)
	if err_ != nil {
		return responseerror.CreateInternalServiceError(err_)
	}

	w.Write(json)
	return nil

}

func playGamesHandler(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {
	cf := metadata.Config.(*config.Config)

	body := r.Context().Value(middleware.PayloadKey).(*payload.UserGames)
	fmt.Println(body.GameID)

	// Check account status
	// check steamid
	req := &httpx.HTTPRequest{}
	req, err := req.CreateRequest(
		cf.Service.Account.Scheme,
		cf.Service.Account.Host,
		cf.Service.Account.Port,
		fmt.Sprintf(CHECK_ACCOUNT_STEAM_LINK_ENDPOINT, body.Username),
		http.MethodGet,
		http.StatusOK,
		nil,
		cf.Config,
	)

	if err != nil {
		return err
	}

	err = req.Send(nil)

	if err != nil {
		return err
	}

	// Join tables
	joinExecutor := querynator.PrepareJoinOperation()
	joinExecutor.UseExplicitCast()
	joinExecutor.AddJoinTable("games", "game_id", "user_games", "game_id")
	joinExecutor.AddJoinTable("storage_location", "storage_id", "games", "storage_id")

	dest := []struct {
		Username string `metadata.DB:"username"`
		GameID   int    `metadata.DB:"game_id"`
		Protocol string `metadata.DB:"protocol"`
		Port     int    `metadata.DB:"port"`
		Host     string `metadata.DB:"host"`
		Location string `metadata.DB:"location"`
	}{}

	err_ := joinExecutor.Find(metadata.DB, []database.QueryCondition{
		{TableName: "user_games", ColumnName: "username", MatchValue: body.Username, Operand: database.EQ},
		{TableName: "user_games", ColumnName: "game_id", MatchValue: body.GameID, Operand: database.EQ},
	}, &dest, "user_games", database.INNER_JOIN, map[string][]string{
		"user_games":       {"username", "game_id"},
		"storage_location": {"protocol", "port,int", "host", "location"},
	})

	if err_ != nil {
		return responseerror.CreateInternalServiceError(err_)
	}

	if len(dest) < 1 {
		return responseerror.CreateNotFoundError(map[string]string{
			"resourceName": "game_id",
		})
	}

	tmp1 := struct {
		Status    string `json:"status"`
		Message   string `json:"message,omitempty"`
		SessionID string `json:"session_id"`
	}{}

	// Send request to session
	req = &httpx.HTTPRequest{}
	req, err = req.CreateRequest(
		cf.Service.Session.Scheme,
		cf.Service.Session.Host,
		cf.Service.Session.Port,
		CREATE_NEW_SESSION_ENDPOINT,
		http.MethodPost,
		http.StatusOK,
		&SessionRequest{
			Username: dest[0].Username,
			Metadata: SessionMetadata{
				GameID: dest[0].GameID,
				GameLocation: GameLocation{
					Protocol: dest[0].Protocol,
					Server: Server{
						IP:   dest[0].Host,
						Port: dest[0].Port,
					},
					Location: dest[0].Location,
				},
			},
		},
		cf.Config,
	)

	if err != nil {
		return err
	}

	err = req.Send(&tmp1)

	if err != nil {
		return err
	}

	tmp1.Message = "session request has been successfully created"

	json, err_ := jsonutil.EncodeToJson(&tmp1)

	if err_ != nil {
		return responseerror.CreateInternalServiceError(err_)
	}

	w.Write(json)

	return nil
}

func SetGamesRoute(r *mux.Router, db *sql.DB, cache *cache.RedisClient, conf *config.Config) {
	certMiddleware := middleware.CertMiddleware(conf.RootCAs)
	authMiddleware := middleware.AuthMiddleware(conf.Service.Auth, conf.Config)

	playGamesPayloadMiddleware, err := middleware.PayloadCheckMiddleware(
		&payload.UserGames{},
		"Username",
		"GameID",
	)

	if err != nil {
		log.Fatal(err)
	}

	syncGamesPayloadMiddleware, err := middleware.PayloadCheckMiddleware(
		&payload.Collections{},
		"Games",
	)

	if err != nil {
		log.Fatal(err)
	}

	subrouter := r.PathPrefix("/games").Subrouter()

	subrouter.Use(middleware.RouteGetterMiddleware)

	syncGames := httpx.CreateHTTPHandler(db, conf, syncUserGamesHandler)
	subrouter.Handle("/{username}/sync", middleware.UseMiddleware(db, conf, syncGames,
		certMiddleware,
		syncGamesPayloadMiddleware,
	)).Methods("POST")

	listGames := httpx.CreateHTTPHandler(db, conf, listGamesHandler)
	listGames.SetCache(cache)

	subrouter.Handle("/{username}/collections", middleware.UseMiddleware(db, conf, listGames, authMiddleware)).Methods("GET")

	// subrouter.Handle("/{username}/sync", middleware.UseMiddleware(db, conf, syncGames, syncGamesPayloadMiddleware)).Methods("POST")

	playGames := httpx.CreateHTTPHandler(db, conf, playGamesHandler)
	subrouter.Handle("/play", middleware.UseMiddleware(db, conf, playGames, authMiddleware, playGamesPayloadMiddleware)).Methods("POST")
}
