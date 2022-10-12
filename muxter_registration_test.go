package muxter

import (
	"net/http"
	"testing"
)

func TestRegistration(t *testing.T) {
	testcases := []struct {
		Name          string
		Routes        []string
		ExpectedError string
	}{
		{
			Name:          "register same route twice",
			Routes:        []string{"/api", "/api"},
			ExpectedError: "muxter: failed to register route /api - multiple registrations",
		},
		{
			Name:          "cannot register route without slash prefix",
			Routes:        []string{"api"},
			ExpectedError: "muxter: route pattern must begin with a forward-slash: '/' but got: api",
		},
		{
			Name:          "register same route twice wildcard",
			Routes:        []string{"/api/:id", "/api/:id"},
			ExpectedError: "muxter: failed to register route /api/:id - multiple registrations",
		},
		{
			Name:          "conflicting wild cards",
			Routes:        []string{"/api/:id", "/api/:resource/value"},
			ExpectedError: "muxter: failed to register route /api/:resource/value - mismatched wild cards :id and :resource",
		},
		{
			Name:   "no errors",
			Routes: []string{"/api", "/api/", "/api/:id", "/api/:id/other"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			defer func() {
				err, _ := recover().(string)
				if tc.ExpectedError != err {
					t.Errorf("expected error %q but got %q", tc.ExpectedError, err)
				}
			}()
			mux := New()
			for _, route := range tc.Routes {
				mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request, c Context) {})
			}
		})
	}

	t.Run("cannot register a nil handler", func(t *testing.T) {
		defer func() {
			actual, _ := recover().(string)
			expected := "muxter: handler cannot be nil"
			if expected != actual {
				t.Errorf("expected error %q but got %q", expected, actual)
			}
		}()

		New().Handle("/", nil)
	})
}
