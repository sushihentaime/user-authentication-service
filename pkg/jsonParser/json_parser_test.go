package jsonParser

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	t.Run("Valid JSON", func(t *testing.T) {
		// Create a mock HTTP request with a valid JSON body
		body := bytes.NewBufferString(`{"name": "John", "age": 30}`)
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err != nil {
			t.Errorf("ParseJSON returned an error: %v", err)
		}

		// Assert the parsed JSON data
		expectedName := "John"
		if data.Name != expectedName {
			t.Errorf("Expected name to be %q, but got %q", expectedName, data.Name)
		}

		expectedAge := 30
		if data.Age != expectedAge {
			t.Errorf("Expected age to be %d, but got %d", expectedAge, data.Age)
		}
	})

	t.Run("Badly-formed JSON", func(t *testing.T) {
		// Create a mock HTTP request with a badly-formed JSON body
		body := bytes.NewBufferString(`{"name": "John", "age": 30`)
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err == nil {
			t.Error("Expected ParseJSON to return an error for badly-formed JSON")
		}
	})

	t.Run("Invalid JSON type", func(t *testing.T) {
		// Create a mock HTTP request with an invalid JSON type
		body := bytes.NewBufferString(`{"name": "John", "age": "thirty"}`)
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err == nil {
			t.Error("Expected ParseJSON to return an error for invalid JSON type")
		}
	})

	t.Run("Empty request body", func(t *testing.T) {
		// Create a mock HTTP request with an empty body
		body := bytes.NewBufferString("")
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err == nil {
			t.Error("Expected ParseJSON to return an error for empty request body")
		}
	})

	t.Run("Unknown field in JSON", func(t *testing.T) {
		// Create a mock HTTP request with an unknown field in JSON
		body := bytes.NewBufferString(`{"name": "John", "age": 30, "city": "New York"}`)
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err == nil {
			t.Error("Expected ParseJSON to return an error for unknown field in JSON")
		}
	})

	t.Run("Request body larger than limit", func(t *testing.T) {
		// Create a mock HTTP request with a large request body
		body := bytes.NewBufferString(strings.Repeat("a", 1_048_577))
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err == nil {
			t.Error("Expected ParseJSON to return an error for request body larger than limit")
		}
	})

	t.Run("Multiple JSON values in request body", func(t *testing.T) {
		// Create a mock HTTP request with multiple JSON values in the body
		body := bytes.NewBufferString(`{"name": "John"}{"age": 30}`)
		req, err := http.NewRequest("POST", "/api", body)
		if err != nil {
			t.Fatal(err)
		}

		// Create a mock HTTP response
		recorder := httptest.NewRecorder()

		// Call the ParseJSON function
		var data struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		err = ParseJSON(recorder, req, &data)
		if err == nil {
			t.Error("Expected ParseJSON to return an error for multiple JSON values in request body")
		}
	})
}
