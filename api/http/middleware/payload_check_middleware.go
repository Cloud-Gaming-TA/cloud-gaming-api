package middleware

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/responseerror"
)

var PayloadKey ContextKey = "payload"

func checkFieldsValidity(template interface{}, requiredFields []string) error {
	// Check if the requiredFields is valid

	p := reflect.ValueOf(template)
	s := p

	if p.Kind() == reflect.Pointer {
		// Get the pointer type
		s = p.Elem()
	}

	sType := s.Type()

	fieldMap := make(map[string][]string)

	for _, field := range requiredFields {
		splitFieldName := strings.SplitN(field, ":", 2)

		fieldName := splitFieldName[0]
		v := s.FieldByName(fieldName)
		if !v.IsValid() {
			return fmt.Errorf("struct of %s don't have field named %s", s.Type(), field)
		}

		if len(splitFieldName) > 2 {
			fieldMap[splitFieldName[0]] = append(fieldMap[splitFieldName[0]], splitFieldName[1])
			continue
		}

		f, _ := sType.FieldByName(fieldName)

		tag := f.Tag.Get("json")

		if tag == "" || tag == "-" {
			return fmt.Errorf("required fields of %s have json tag of %s, which isn't valid", field, tag)
		}
	}

	for structName, fieldName := range fieldMap {
		err := checkFieldsValidity(s.FieldByName(structName).Interface(), fieldName)
		// Short circuit check
		if err != nil {
			return err
		}
	}

	return nil
}

func PayloadCheckMiddleware(template httpx.Payload, metadata *httpx.Metadata, requiredFields ...string) (Middleware, error) {
	var payload httpx.Payload

	p := reflect.ValueOf(template)

	if p.Kind() != reflect.Ptr {
		err := fmt.Errorf("cannot create middleware. template isn't a pointer")
		return nil, err
	}

	// Struct reflect
	s := p.Elem()
	sType := s.Type()

	// Check if the requiredFields is valid
	// for _, field := range requiredFields {
	// 	v := s.FieldByName(field)
	// 	if !v.IsValid() {
	// 		return nil, fmt.Errorf("struct of %s don't have field named %s", s.Type(), field)
	// 	}

	// 	f, _ := sType.FieldByName(field)

	// 	tag := f.Tag.Get("json")

	// 	if tag == "" || tag == "-" {
	// 		return nil, fmt.Errorf("required fields of %s have json tag of %s, which isn't valid", field, tag)
	// 	}
	// }

	err := checkFieldsValidity(template, requiredFields)

	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		fn := func(metadata *httpx.Metadata, w http.ResponseWriter, r *http.Request) responseerror.HTTPCustomError {

			if r.Header.Get("Content-Type") != "application/json" {
				return responseerror.CreateBadRequestError(
					responseerror.HeaderValueMistmatch,
					responseerror.HeaderValueMistmatchMessage,
					map[string]string{
						"name": "Content-Type",
					},
				)
			}

			// create a new struct with the same type as template
			payload = reflect.New(sType).Interface().(httpx.Payload)

			if r.Body == nil {
				return responseerror.CreateBadRequestError(
					responseerror.PayloadInvalid,
					responseerror.PayloadInvalidMessage,
					nil,
				)
			}

			err := payload.FromJSON(r.Body, true, requiredFields)

			if err != nil {
				return err
			}

			ctx := context.WithValue(r.Context(), PayloadKey, payload)
			next.ServeHTTP(w, r.WithContext(ctx))

			return nil
		}

		return &httpx.Handler{
			Metadata: metadata,
			Handler:  fn,
		}
	}, nil
}
