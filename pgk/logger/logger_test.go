// Package runtime
package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewHTTPLogger(t *testing.T) {
	logger := NewHTTPLogger()
	assert.NotNil(t, logger)
	assert.NotNil(t, logger.Logger)
}

func TestHTTPLogger_LogRequest(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := &HTTPLogger{Logger: zap.New(core)}

	logger.LogRequest("GET", "/api/test", 200, 1024, 15.5)

	assert.Equal(t, 1, recorded.Len())

	entries := recorded.All()
	entry := entries[0]

	assert.Equal(t, "HTTP request", entry.Message)

	fields := entry.ContextMap()
	assert.Equal(t, "GET", fields["method"])
	assert.Equal(t, "/api/test", fields["uri"])
	assert.Equal(t, int64(200), fields["status"])
	assert.Equal(t, int64(1024), fields["response_size"])
	assert.InDelta(t, 15.5, fields["duration_ms"], 0.1)
}

func TestHTTPLogger_LogRequest_MultipleCalls(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := &HTTPLogger{Logger: zap.New(core)}

	logger.LogRequest("POST", "/api/users", 201, 256, 8.2)
	logger.LogRequest("DELETE", "/api/users/123", 204, 0, 3.1)

	assert.Equal(t, 2, recorded.Len())

	entries := recorded.All()

	firstEntry := entries[0]
	firstFields := firstEntry.ContextMap()
	assert.Equal(t, "POST", firstFields["method"])
	assert.Equal(t, "/api/users", firstFields["uri"])
	assert.Equal(t, int64(201), firstFields["status"])

	secondEntry := entries[1]
	secondFields := secondEntry.ContextMap()
	assert.Equal(t, "DELETE", secondFields["method"])
	assert.Equal(t, "/api/users/123", secondFields["uri"])
	assert.Equal(t, int64(204), secondFields["status"])
}

func TestHTTPLogger_LogRequest_ErrorStatus(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := &HTTPLogger{Logger: zap.New(core)}

	logger.LogRequest("PUT", "/api/items", 400, 128, 12.7)

	assert.Equal(t, 1, recorded.Len())

	entry := recorded.All()[0]
	fields := entry.ContextMap()
	assert.Equal(t, int64(400), fields["status"])
	assert.Equal(t, "PUT", fields["method"])
}

func TestHTTPLogger_LogRequest_ZeroDuration(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := &HTTPLogger{Logger: zap.New(core)}

	logger.LogRequest("GET", "/health", 200, 2, 0.0)

	assert.Equal(t, 1, recorded.Len())

	entry := recorded.All()[0]
	fields := entry.ContextMap()
	assert.Equal(t, float64(0), fields["duration_ms"])
}

func TestHTTPLogger_LogRequest_ZeroSize(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := &HTTPLogger{Logger: zap.New(core)}

	logger.LogRequest("HEAD", "/", 200, 0, 1.2)

	assert.Equal(t, 1, recorded.Len())

	entry := recorded.All()[0]
	fields := entry.ContextMap()
	assert.Equal(t, int64(0), fields["response_size"])
}
