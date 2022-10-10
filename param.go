package muxter

import (
	"net/http"
)

type paramKeyType int

var paramKey paramKeyType

// Param reads path params from the request
func Param(r *http.Request, param string) string {
	if r == nil {
		return ""
	}

	if params, ok := r.Context().Value(paramKey).(map[string]string); ok && params != nil {
		return params[param]
	}

	return ""
}

// Params returns all path params in a map. Prefer the simple Param to avoid memory allocations.
func Params(r *http.Request) map[string]string {
	if r == nil {
		return nil
	}

	params, _ := r.Context().Value(paramKey).(map[string]string)
	if params == nil {
		return nil
	}

	// The params map belongs to a pool and will be put back and cleared once ServeHTTP is done.
	// Should a user capture the map in a variable that outlives the lifetime of the handler, it
	// would be very hard for them to understand where their params have gone. Hence return a copy
	// of the params.
	cpy := make(map[string]string, len(params))
	for k, v := range params {
		cpy[k] = v
	}

	return cpy
}
