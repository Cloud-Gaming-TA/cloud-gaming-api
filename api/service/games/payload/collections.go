package payload

import (
	"io"

	"github.com/AdityaP1502/Instant-Messanging/api/http/httputil"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
)

type Games struct {
	Name    string `json:"name"`
	GameID  string `json:"app_id"`
	IconURL string `json:"icon_url"`
}

type Collections struct {
	Games []Games `json:"games"`
}

func (c *Collections) FromJSON(r io.Reader, checkRequired bool, requiredFields []string) responseerror.HTTPCustomError {
	err := jsonutil.DecodeJSON(r, c)

	if err != nil {
		return responseerror.CreateInternalServiceError(err)
	}

	if checkRequired {
		return httputil.CheckParametersUnity(c, requiredFields)
	}

	return nil
}

func (c *Collections) ToJSON(checkRequired bool, requiredFields []string) ([]byte, error) {
	var err error

	if checkRequired {
		if err = httputil.CheckParametersUnity(c, requiredFields); err != nil {
			return nil, err
		}
	}

	return jsonutil.EncodeToJson(c)
}
