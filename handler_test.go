package muxter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStdAdaptor(t *testing.T) {
	mux := New()

	mux.StdHandle(
		"/country/:country/city/:city",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := Params(r)
			if len(p) != 2 {
				t.Errorf("expected params to have length 2 but got: %d", len(p))
			}

			expected := "ca"
			if actual := Param(r, "country"); actual != expected {
				t.Errorf("expected country to be %q but got %q", expected, actual)
			}

			expected = "mtl"
			if actual := Param(r, "city"); actual != expected {
				t.Errorf("expected city to be %q but got %q", expected, actual)
			}

			expected = "/country/:country/city/:city"
			if actual := MatchedPath(r); actual != expected {
				t.Errorf("expected matched path to be %q but got %q", expected, actual)
			}
		}),
		true,
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/country/ca/city/mtl", nil)

	mux.ServeHTTP(w, r)
}
