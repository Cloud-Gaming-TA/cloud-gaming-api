package httpx

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/AdityaP1502/Instant-Messanging/api/cache"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
)

type HandlerLogic func(metadata *Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError

type Metadata struct {
	DB     *sql.DB
	Cache  *cache.RedisClient
	Config interface{}
}
type Handler struct {
	Metadata *Metadata
	Handler  HandlerLogic
}

func (h *Handler) SetCache(client *cache.RedisClient) {
	h.Metadata.Cache = client
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	if err := h.Handler(h.Metadata, w, r); err != nil {
		if internalErr, ok := err.(*responseerror.InternalServiceError); ok {
			fmt.Println(internalErr.Description)
		}

		requestErr := err.Get()

		errorResponse := responseerror.FailedRequestResponse{
			Status:    "fail",
			ErrorType: requestErr.Name,
			Message:   requestErr.Message,
		}

		w.WriteHeader(requestErr.Code)

		json, err := jsonutil.EncodeToJson(&errorResponse)

		if err != nil {
			http.Error(w, "Something wrong with server!", 500)
		}

		w.Write(json)
	}
}

func CreateHTTPHandler(db *sql.DB, conf interface{}, logic HandlerLogic) *Handler {
	handler := &Handler{
		Metadata: &Metadata{
			DB:     db,
			Config: conf,
		},
		Handler: logic,
	}

	// if corsOptions != nil {
	// 	// add cors into the hadnler if provided non nil options
	// 	corsHandler := cors.New(*corsOptions).Handler(handler)
	// 	return corsHandler
	// }

	return handler
}
