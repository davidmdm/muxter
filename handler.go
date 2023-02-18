package muxter

import (
	"context"
	"net/http"

	"github.com/davidmdm/muxter/internal"
)

type Context struct {
	params    *[]internal.Param
	ogReqPath string
	pattern   string
}

// Param returns the param value for the key. If no param exists for the key the empty string is returned.
func (c Context) Param(key string) string {
	for _, p := range *c.params {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}

// Params returns a copy of the param map
func (c Context) Params() map[string]string {
	if c.params == nil {
		return map[string]string{}
	}
	paramMap := make(map[string]string, len(*c.params))
	for _, param := range *c.params {
		paramMap[param.Key] = param.Value
	}

	return paramMap
}

// Pattern returns the registered route pattern that was matched.
func (c Context) Pattern() string {
	return c.pattern
}

//go:generate moq -out handler_mock_test.go --stub . Handler
type Handler interface {
	// ServeHTTPx is the equivalent of the standard http.Handler's ServeHTTP but includes the muxter Context
	ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context)
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request, c Context)

func (fn HandlerFunc) ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context) {
	fn(w, r, c)
}

type adaptorOptions struct {
	noContext bool
}

type AdaptorOption func(*adaptorOptions)

// NoContext is an option for the Adaptor API. Using it when adapting stand http.Handlers to muxter.Handlers
// will not copy the muxter Context onto the request context. This saves the muxter from doing an allocation but
// means that there is no way to access params and so on from inside the standard handler.
var NoContext AdaptorOption = func(ao *adaptorOptions) {
	ao.noContext = true
}

// Adaptor adapts a standard http.Handler into a muxter.Handler. By default it will copy
// the muxter.Context into the request context, allowing params to be used from standard http handlers
// albeit at a slight performance cost.
func Adaptor(h http.Handler, opts ...AdaptorOption) Handler {
	var options adaptorOptions
	for _, apply := range opts {
		apply(&options)
	}

	return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
		if !options.noContext {
			*r = *r.WithContext(context.WithValue(r.Context(), cKey, c))
		}
		h.ServeHTTP(w, r)
	})
}

type ctxKetType struct{}

var cKey ctxKetType

// Param reads path params from the request
func Param(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	c, _ := r.Context().Value(cKey).(Context)
	return c.Param(key)
}

// Params returns all path params in a map. Prefer the simple Param to avoid memory allocations.
// Only works on standard handlers that have been through the Adaptor interface. Prefer using muxter.Context directly.
func Params(r *http.Request) map[string]string {
	if r == nil {
		return nil
	}
	c, _ := r.Context().Value(cKey).(Context)
	return c.Params()
}

// Pattern returns the matched registered route pattern.
// Only works on standard handlers that have been through the Adaptor interface. Prefer using muxter.Context directly.
func Pattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	c, _ := r.Context().Value(cKey).(Context)
	return c.Pattern()
}
