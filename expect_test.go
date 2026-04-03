package expect

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestOutput(t *testing.T) {
	t.Parallel()

	Output(t, "Hello, World!\nThis is a test output.")
}

func TestJSON(t *testing.T) {
	t.Parallel()

	JSON(t, map[string]any{
		"name":   "John Doe",
		"email":  "john@example.com",
		"age":    30,
		"active": true,
		"tags":   []string{"developer", "golang", "testing"},
		"metadata": map[string]any{
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-15T12:00:00Z",
		},
	})
}

func TestRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "age": 25
}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token-123")
	req.Header.Set("User-Agent", "Go-Test-Client/1.0")

	Request(t, req)
}

func TestResponse(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "application/json")
	recorder.Header().Set("X-Custom-Header", "custom-value")
	recorder.Header().Set("fs-request-id", "req-123-should-be-removed")
	recorder.Header().Set("x-request-id", "req-456-should-be-removed")
	recorder.WriteHeader(http.StatusOK)
	recorder.Write([]byte(`{
  "status": "success",
  "data": {
    "id": "user-123",
    "name": "Test User"
  }
}`))

	Response(t, recorder.Result())
}

func TestResponseWithImage(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "image/jpeg")
	recorder.WriteHeader(http.StatusOK)
	recorder.Write([]byte("fake-image-binary-data"))

	Response(t, recorder.Result())
}

func TestHTML(t *testing.T) {
	t.Parallel()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Welcome</h1>
    <p>This is a test HTML document.</p>
</body>
</html>`

	HTML(t, html)
}

func TestOutputDiff(t *testing.T) {
	t.Parallel()

	before := `line 1
line 2
line 3
line 4
line 5`

	after := `line 1
line 2 modified
line 3
line 4 changed
line 5`

	OutputDiff(t, before, after)
}

func TestLogs(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	logger.Info("user login", zap.String("user_id", "123"), zap.String("ip", "192.168.1.1"))
	logger.Info("database query", zap.String("query", "SELECT * FROM users"), zap.Int("duration_ms", 42))
	logger.Warn("slow query detected", zap.String("table", "accounts"), zap.Int("rows", 10000))

	Logs(t, recorded.All())
}

type MockService struct {
	mock.Mock
}

func (m *MockService) CreateUser(name string, age int) (string, error) {
	args := m.Called(name, age)
	return args.String(0), args.Error(1)
}

func (m *MockService) DeleteUser(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func TestCalls(t *testing.T) {
	t.Parallel()

	mockSvc := new(MockService)
	mockSvc.On("CreateUser", "Alice", 30).Return("user-001", nil)
	mockSvc.On("CreateUser", "Bob", 25).Return("user-002", nil)
	mockSvc.On("DeleteUser", "user-999").Return(nil)

	mockSvc.CreateUser("Alice", 30)
	mockSvc.CreateUser("Bob", 25)
	mockSvc.DeleteUser("user-999")

	Calls(t, mockSvc.Calls)
}

func TestResponseStream(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	recorder.Header().Set("Cache-Control", "no-cache")
	recorder.Header().Set("Connection", "keep-alive")
	recorder.WriteHeader(http.StatusOK)

	sseData := `event: start
data: {"status":"started","timestamp":"2024-01-01T00:00:00Z"}

event: progress
data: {"status":"processing","step":1,"total":3}

event: progress
data: {"status":"processing","step":2,"total":3}

event: progress
data: {"status":"processing","step":3,"total":3}

event: complete
data: {"status":"completed","result":"success"}
id: evt-12345

`
	recorder.Write([]byte(sseData))

	ResponseStream(t, recorder.Result())
}

func TestResponseStreamWithComments(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	recorder.WriteHeader(http.StatusOK)

	sseData := `: This is a comment
data: {"message":"First event"}

: Another comment line
event: error
data: {"error":"Something went wrong","code":"ERR_001"}

data: {"message":"Recovery attempt"}

`
	recorder.Write([]byte(sseData))

	ResponseStream(t, recorder.Result())
}

func TestOutputWithExtension(t *testing.T) {
	t.Parallel()

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<root>
    <user id="123">
        <name>John Doe</name>
        <email>john@example.com</email>
    </user>
</root>`

	OutputWithExtension(t, ".xml", xmlContent)
}

func TestComplexJSON(t *testing.T) {
	t.Parallel()

	JSON(t, map[string]any{
		"user": map[string]any{
			"id":       "usr_12345",
			"name":     "Alice Smith",
			"email":    "alice@example.com",
			"verified": true,
			"roles":    []string{"admin", "developer"},
		},
		"organization": map[string]any{
			"id":   "org_67890",
			"name": "Acme Corp",
			"plan": "enterprise",
		},
		"settings": map[string]any{
			"notifications": map[string]bool{
				"email": true,
				"slack": false,
				"sms":   true,
			},
			"preferences": map[string]any{
				"theme":    "dark",
				"language": "en",
				"timezone": "America/Los_Angeles",
			},
		},
	})
}

func TestMultipleRequestTypes(t *testing.T) {
	t.Parallel()

	t.Run("GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users?page=1&limit=10", nil)
		req.Header.Set("Accept", "application/json")
		Request(t, req)
	})

	t.Run("PUT", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/users/123", strings.NewReader(`{"name":"Updated Name"}`))
		req.Header.Set("Content-Type", "application/json")
		Request(t, req)
	})

	t.Run("DELETE", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/users/123", nil)
		req.Header.Set("Authorization", "Bearer token")
		Request(t, req)
	})
}
