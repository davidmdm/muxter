package muxter

import (
	"net/http"
	"strings"
)

func StripDepth(depth int, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = stripDepth(r.URL.Path, depth)
		handler.ServeHTTP(w, r)
	})
}

func stripDepth(value string, depth int) string {
	if depth == 0 {
		return value
	}

	value = strings.TrimPrefix(value, "/")
	var seen int
	var i int

	for i = range value {
		if value[i] == '/' {
			seen++
		}
		if seen == depth {
			break
		}
	}
	if i == len(value) {
		return "/"
	}
	return "/" + value[i+1:]
}
