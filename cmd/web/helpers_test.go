package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	// Create a new HTTP recorder
	recorder := httptest.NewRecorder()

	// Create a mock application
	app := &application{}

	// Create a mock data envelope
	data := envelope{"Message": "Hello, World!"}

	// Create mock headers
	headers := http.Header{
		"X-Custom-Header": []string{"value1", "value2"},
	}

	// Call the writeJSON function
	err := app.writeJSON(recorder, http.StatusOK, data, headers)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check the response status code
	if recorder.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, recorder.Code)
	}

	// Check the response body
	expectedBody := "{\n\t\"Message\": \"Hello, World!\"\n}\n"
	if recorder.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, recorder.Body.String())
	}

	// Check the response headers
	expectedHeaders := http.Header{
		"Content-Type":    []string{"application/json"},
		"X-Custom-Header": []string{"value1", "value2"},
	}
	for key, values := range expectedHeaders {
		actualValues := recorder.Header()[key]
		if !reflect.DeepEqual(actualValues, values) {
			t.Errorf("expected header %q with values %v, got %v", key, values, actualValues)
		}
	}
}
