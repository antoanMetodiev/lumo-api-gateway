package downstream

import (
	"context"
	"net/http"
	"net/url"
)

type languageKey struct{}

// LanguageMiddleware remembers the caller's ?lang (bg, el or es — the
// visitor's language on the website / in the app) for the length of the
// request, so Do can pass it on to every GET it makes downstream: the movies
// and series services then answer with the translated titles, descriptions,
// posters and trailers. It saves threading the parameter through each of the
// ~30 proxy call sites. English (no lang) is the services' default.
func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch lang := r.URL.Query().Get("lang"); lang {
		case "bg", "el", "es":
			r = r.WithContext(context.WithValue(r.Context(), languageKey{}, lang))
		}
		next.ServeHTTP(w, r)
	})
}

// withLanguage adds the request's language to a downstream GET URL that
// doesn't carry one already.
func withLanguage(ctx context.Context, method, rawURL string) string {
	lang, _ := ctx.Value(languageKey{}).(string)
	if lang == "" || method != http.MethodGet {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	if q.Get("lang") != "" {
		return rawURL
	}
	q.Set("lang", lang)
	u.RawQuery = q.Encode()
	return u.String()
}
