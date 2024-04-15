package middleware

import (
	"io"
	"testing"

	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/httputil"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
)

type Server struct {
	IP   string `json:"ip"`
	Port int    `json:"port,string"`
	Host string `json:"-" db:"game_location_server_host"`
}

type GameLocation struct {
	Protocol string `json:"protocol" db:"game_location_protocol"`
	Path     string `json:"path" db:"game_location_path"`
	Server   Server `json:"server"`
}

type SessionMetadata struct {
	MetadataID   int          `db:"metadata_id"`
	CreatedAt    string       `db:"created_at"`
	GameID       int          `json:"game_id" db:"game_id"`
	GameLocation GameLocation `json:"game_location"`
}

type UserSession struct {
	SessionID         []byte          `json:"-" db:"session_id"`
	SessionIDString   string          `json:"session_id" db:"-"`
	Username          string          `json:"username" db:"username"`
	RequestStatus     string          `json:"status" db:"request_status"`
	HostID            int             `json:"-" db:"host_id"`
	SessionMetadata   SessionMetadata `json:"session_metadata" db:"-"` // disable recurse
	MarkedForDeletion bool            `json:"-" db:"marked_for_deletion"`
}

func (c *UserSession) FromJSON(r io.Reader, checkRequired bool, requiredFields []string) responseerror.HTTPCustomError {
	err := jsonutil.DecodeJSON(r, c)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if checkRequired {
		return httputil.CheckParametersUnity(c, requiredFields)
	}

	return nil
}

func (c *UserSession) ToJSON(checkRequired bool, requiredFields []string) ([]byte, error) {
	var err error

	if checkRequired {
		if err = httputil.CheckParametersUnity(c, requiredFields); err != nil {
			return nil, err
		}
	}

	return jsonutil.EncodeToJson(c)
}

func TestRecurseValidityCheck(t *testing.T) {
	_, err := PayloadCheckMiddleware(&UserSession{}, &httpx.Metadata{},
		"Username",
		"SessionMetadata:GameID",
		"SessionMetadata:GameLocation:Protocol",
		"SessionMetadata:GameLocation:Path",
		"SessionMetadata:GameLocation:Server:IP",
		"SessionMetadata:GameLocation:Server:Port",
	)

	if err != nil {
		t.Log(err)
	}
}

func TestRecurseParameterUnity(t *testing.T) {
	data := &UserSession{
		Username: "xd234",
		SessionMetadata: SessionMetadata{
			GameID: 132132,
			GameLocation: GameLocation{
				Protocol: "nas",
				Path:     "/games/132132/data",
				Server: Server{
					IP:   "",
					Port: 3000,
				},
			},
		},
	}

	err := httputil.CheckParametersUnity(data, []string{
		"Username",
		"SessionMetadata:GameID",
		"SessionMetadata:GameLocation:Protocol",
		"SessionMetadata:GameLocation:Path",
		"SessionMetadata:GameLocation:Server:Port",
	})

	if err != nil {
		t.Log(err)
		return
	}
}
