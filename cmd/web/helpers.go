package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (e envelope) JSON() string {
	json, err := json.MarshalIndent(e, "", "\t")
	if err != nil {
		return ""
	}

	return string(json)
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	res, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	res = append(res, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(res)

	return nil
}

func (app *application) backgroundTask(fn func()) {
	app.wg.Add(1)

	go func() {
		defer app.wg.Done()

		defer func() {
			if err := recover(); err != nil {
				app.logger.Error(fmt.Sprintf("%v", err))
			}
		}()

		fn()
	}()
}

func (app *application) extractTokenFromHeader(authHeader string) string {
	data := strings.Split(authHeader, " ")
	if len(data) != 2 || data[0] != "Bearer" {
		return ""
	}

	return data[1]
}

func (app *application) readStringParam(r *http.Request, key string) (*string, error) {
	params := httprouter.ParamsFromContext(r.Context())

	username := params.ByName(key)
	if username == "" {
		return nil, fmt.Errorf("missing %s parameter", key)
	}

	return &username, nil
}
