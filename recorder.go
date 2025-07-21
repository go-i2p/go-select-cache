package selectcache

import (
	"net/http"
)

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// ResponseRecorder captures HTTP responses for caching
type ResponseRecorder struct {
	http.ResponseWriter
	statusCode int
	headers    http.Header
	body       []byte
	written    bool
}

// NewResponseRecorder creates a new response recorder
func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		statusCode:     200, // Default status
		headers:        make(http.Header),
	}
}

// WriteHeader captures the status code and headers
func (r *ResponseRecorder) WriteHeader(code int) {
	if r.written {
		return // Prevent multiple calls
	}

	r.statusCode = code

	// Copy headers from underlying ResponseWriter
	for k, v := range r.ResponseWriter.Header() {
		r.headers[k] = v
	}

	r.ResponseWriter.WriteHeader(code)
	r.written = true
}

// Write captures the response body and writes to underlying ResponseWriter
func (r *ResponseRecorder) Write(data []byte) (int, error) {
	if !r.written {
		r.WriteHeader(200) // Implicit 200 if not set
	}

	// Store body for caching
	r.body = append(r.body, data...)

	// Write to actual response
	return r.ResponseWriter.Write(data)
}

// Header returns the header map that will be sent by WriteHeader
func (r *ResponseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

// StatusCode returns the recorded status code
func (r *ResponseRecorder) StatusCode() int {
	return r.statusCode
}

// Headers returns a copy of the recorded headers
func (r *ResponseRecorder) Headers() http.Header {
	headers := make(http.Header)
	for k, v := range r.headers {
		headers[k] = v
	}
	return headers
}

// Body returns a copy of the recorded response body
func (r *ResponseRecorder) Body() []byte {
	body := make([]byte, len(r.body))
	copy(body, r.body)
	return body
}

// Size returns the size of the recorded response body
func (r *ResponseRecorder) Size() int {
	return len(r.body)
}
