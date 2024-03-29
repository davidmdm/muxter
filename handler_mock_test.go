// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package muxter

import (
	"net/http"
	"sync"
)

// Ensure, that HandlerMock does implement Handler.
// If this is not the case, regenerate this file with moq.
var _ Handler = &HandlerMock{}

// HandlerMock is a mock implementation of Handler.
//
//	func TestSomethingThatUsesHandler(t *testing.T) {
//
//		// make and configure a mocked Handler
//		mockedHandler := &HandlerMock{
//			ServeHTTPxFunc: func(w http.ResponseWriter, r *http.Request, c Context)  {
//				panic("mock out the ServeHTTPx method")
//			},
//		}
//
//		// use mockedHandler in code that requires Handler
//		// and then make assertions.
//
//	}
type HandlerMock struct {
	// ServeHTTPxFunc mocks the ServeHTTPx method.
	ServeHTTPxFunc func(w http.ResponseWriter, r *http.Request, c Context)

	// calls tracks calls to the methods.
	calls struct {
		// ServeHTTPx holds details about calls to the ServeHTTPx method.
		ServeHTTPx []struct {
			// W is the w argument value.
			W http.ResponseWriter
			// R is the r argument value.
			R *http.Request
			// C is the c argument value.
			C Context
		}
	}
	lockServeHTTPx sync.RWMutex
}

// ServeHTTPx calls ServeHTTPxFunc.
func (mock *HandlerMock) ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context) {
	callInfo := struct {
		W http.ResponseWriter
		R *http.Request
		C Context
	}{
		W: w,
		R: r,
		C: c,
	}
	mock.lockServeHTTPx.Lock()
	mock.calls.ServeHTTPx = append(mock.calls.ServeHTTPx, callInfo)
	mock.lockServeHTTPx.Unlock()
	if mock.ServeHTTPxFunc == nil {
		return
	}
	mock.ServeHTTPxFunc(w, r, c)
}

// ServeHTTPxCalls gets all the calls that were made to ServeHTTPx.
// Check the length with:
//
//	len(mockedHandler.ServeHTTPxCalls())
func (mock *HandlerMock) ServeHTTPxCalls() []struct {
	W http.ResponseWriter
	R *http.Request
	C Context
} {
	var calls []struct {
		W http.ResponseWriter
		R *http.Request
		C Context
	}
	mock.lockServeHTTPx.RLock()
	calls = mock.calls.ServeHTTPx
	mock.lockServeHTTPx.RUnlock()
	return calls
}
