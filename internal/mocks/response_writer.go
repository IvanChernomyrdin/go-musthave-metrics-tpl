// Package mocks
package mocks

import (
	"net/http"

	"github.com/stretchr/testify/mock"
)

type ResponseWriter struct {
	mock.Mock
}

func (m *ResponseWriter) Header() http.Header {
	args := m.Called()
	return args.Get(0).(http.Header)
}

func (m *ResponseWriter) Write(data []byte) (int, error) {
	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

func (m *ResponseWriter) WriteHeader(statusCode int) {
	m.Called(statusCode)
}
