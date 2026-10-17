package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"example.edu/6cc545/cw1/internal/state"
	"io"
	"mime"
	"net/http"
	"strings"
)

const MaxBody = 4096

type InputError struct {
	Status int
	Label  string
}

func (e *InputError) Error() string {
	return e.Label
}

// DecodeOne checks the media type, body size and JSON input.
// It does not change application state.
func DecodeOne(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		return &InputError{http.StatusUnsupportedMediaType, "unsupported_media_type"}
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBody))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &InputError{http.StatusRequestEntityTooLarge, "body_too_large"}
		}
		return &InputError{http.StatusBadRequest, "invalid_body"}
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return &InputError{http.StatusBadRequest, "invalid_json"}
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return &InputError{http.StatusBadRequest, "invalid_json"}
	}

	// A second value or trailing non-whitespace data is not allowed.
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return &InputError{http.StatusBadRequest, "invalid_json"}
	}

	return nil
}

func RejectInput(w http.ResponseWriter, err error) {
	var inputErr *InputError
	if errors.As(err, &inputErr) {
		ErrorJSON(w, inputErr.Status, inputErr.Label)
		return
	}
	ErrorJSON(w, http.StatusBadRequest, "invalid_input")
}

func StatusResource(s *state.Status) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			WriteJSON(w, http.StatusOK, map[string]string{
				"status": s.Get(),
			})

		case http.MethodPost:
			var request struct {
				Status string `json:"status"`
			}

			if err := DecodeOne(w, r, &request); err != nil {
				RejectInput(w, err)
				return
			}

			if err := s.Set(request.Status); err != nil {
				ErrorJSON(w, http.StatusBadRequest, "invalid_status")
				return
			}

			WriteJSON(w, http.StatusOK, map[string]string{
				"status": s.Get(),
			})

		default:
			w.Header().Set("Allow", "GET, POST")
			ErrorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed")
		}
	}
}
