package downstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithLanguage(t *testing.T) {
	var got string
	handler := LanguageMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = withLanguage(r.Context(), http.MethodGet, "http://movies/get-movie-details?movieId=27205")
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/get-movie-details?movieId=27205&lang=bg", nil))
	if got != "http://movies/get-movie-details?lang=bg&movieId=27205" {
		t.Fatalf("bg: got %q", got)
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/get-movie-details?movieId=27205&lang=fr", nil))
	if got != "http://movies/get-movie-details?movieId=27205" {
		t.Fatalf("unsupported language should not be forwarded: got %q", got)
	}

	if url := withLanguage(context.Background(), http.MethodGet, "http://movies/x"); url != "http://movies/x" {
		t.Fatalf("no language: got %q", url)
	}
}
