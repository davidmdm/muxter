// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package muxter

import (
	"net/http"
	"sync"
)

// Ensure, that HandlerMock does implement http.Handler.
// If this is not the case, regenerate this file with moq.
var _ http.Handler = &HandlerMock{}

// HandlerMock is a mock implementation of http.Handler.
//
// 	func TestSomethingThatUsesHandler(t *testing.T) {
//
// 		// make and configure a mocked http.Handler
// 		mockedHandler := &HandlerMock{
// 			ServeHTTPFunc: func(responseWriter http.ResponseWriter, request *http.Request)  {
// 				panic("mock out the ServeHTTP method")
// 			},
// 		}
//
// 		// use mockedHandler in code that requires http.Handler
// 		// and then make assertions.
//
// 	}
type HandlerMock struct {
	// ServeHTTPFunc mocks the ServeHTTP method.
	ServeHTTPFunc func(responseWriter http.ResponseWriter, request *http.Request)

	// calls tracks calls to the methods.
	calls struct {
		// ServeHTTP holds details about calls to the ServeHTTP method.
		ServeHTTP []struct {
			// ResponseWriter is the responseWriter argument value.
			ResponseWriter http.ResponseWriter
			// Request is the request argument value.
			Request *http.Request
		}
	}
	lockServeHTTP sync.RWMutex
}

// ServeHTTP calls ServeHTTPFunc.
func (mock *HandlerMock) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	callInfo := struct {
		ResponseWriter http.ResponseWriter
		Request        *http.Request
	}{
		ResponseWriter: responseWriter,
		Request:        request,
	}
	mock.lockServeHTTP.Lock()
	mock.calls.ServeHTTP = append(mock.calls.ServeHTTP, callInfo)
	mock.lockServeHTTP.Unlock()
	if mock.ServeHTTPFunc == nil {
		return
	}
	mock.ServeHTTPFunc(responseWriter, request)
}

// ServeHTTPCalls gets all the calls that were made to ServeHTTP.
// Check the length with:
//     len(mockedHandler.ServeHTTPCalls())
func (mock *HandlerMock) ServeHTTPCalls() []struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
} {
	var calls []struct {
		ResponseWriter http.ResponseWriter
		Request        *http.Request
	}
	mock.lockServeHTTP.RLock()
	calls = mock.calls.ServeHTTP
	mock.lockServeHTTP.RUnlock()
	return calls
}