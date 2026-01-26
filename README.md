# expect

A Go testing library that provides snapshot testing helpers to make your tests more maintainable and readable.

## What is Snapshot Testing?

Snapshot testing is a technique where the output of a function, API, SQL query, or any other operation is captured and saved as a _snapshot_. On subsequent test runs, the output is compared against the existing snapshot. If they differ, the test fails, signaling an unintended change.

This approach ensures that changes to your system are explicit and deliberate. If an update is intentional, you can regenerate the snapshots to reflect the new expected behavior.

## Why Snapshot Testing?

Snapshot testing provides several key benefits:

- **Easier to use than assertions**: Instead of writing dozens of individual `assert.Equal()` calls, you snapshot the entire output. This is much easier to maintain.
- **Visual verification**: Snapshots are stored as files you can read and review, making it easy to visually verify the output is correct.
- **Catches all changes**: Snapshots detect any difference in output, not just the specific fields you thought to assert on. This catches unexpected changes you might have missed.

However, because snapshots catch _any_ diff, you need to control sources of nondeterminism (timestamps, UUIDs, random values, etc.) to prevent flaky tests. See the [Handling Nondeterminism](#handling-nondeterminism) section below for guidance.

## Features

- **Snapshot Testing**: Automatically create and compare test output snapshots
- **HTTP Testing**: Helpers for testing HTTP requests and responses
- **SSE Stream Testing**: Parse and test Server-Sent Events streams with structured data
- **SQL Formatting**: Format and snapshot SQL queries
- **Mock Testing**: Snapshot mock call history
- **Log Testing**: Snapshot structured log output

## Installation

```bash
go get github.com/funnelstory/expect
```

## Quick Start

```go
import (
    "testing"
    "github.com/funnelstory/expect"
)

func TestMyFunction(t *testing.T) {
    result := myFunction()
    expect.Output(t, result)
}
```

## Usage

### Basic Output Snapshots

The most basic usage is to snapshot any output:

```go
func TestOutput(t *testing.T) {
    result := "Hello, World!"
    expect.Output(t, result)
}
```

On the first run, this creates a snapshot file. On subsequent runs, it compares the output against the snapshot.

### JSON Snapshots

Format and snapshot JSON data with proper indentation:

```go
func TestJSON(t *testing.T) {
    data := map[string]any{
        "name": "John Doe",
        "email": "john@example.com",
        "age": 30,
    }
    expect.JSON(t, data)
}
```

### HTTP Request Testing

Snapshot HTTP request details including headers and body:

```go
func TestHTTPRequest(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/api/users",
        strings.NewReader(`{"name":"John"}`))
    req.Header.Set("Content-Type", "application/json")

    expect.Request(t, req)
}
```

### HTTP Response Testing

Snapshot HTTP responses with automatic cleanup of volatile headers:

```go
func TestHTTPResponse(t *testing.T) {
    recorder := httptest.NewRecorder()
    recorder.Header().Set("Content-Type", "application/json")
    recorder.WriteHeader(http.StatusOK)
    recorder.Write([]byte(`{"status":"ok"}`))

    resp := recorder.Result()
    expect.Response(t, resp)
}
```

The `Response` function automatically removes common volatile headers:

- `Date`
- `Vary`
- `Access-Control-Allow-Credentials`
- `Access-Control-Allow-Origin`
- Any header ending with `-request-id` (e.g., `fs-request-id`, `x-request-id`, `custom-request-id`)

### Server-Sent Events (SSE) Testing

Test SSE streams with structured event parsing:

```go
func TestEventStream(t *testing.T) {
    recorder := httptest.NewRecorder()

    // Create SSE stream
    recorder.Header().Set("Content-Type", "text/event-stream")
    recorder.WriteHeader(http.StatusOK)
    recorder.Write([]byte(`event: message
data: {"text":"Hello"}

event: update
data: {"count":42}
id: evt-123

`))

    resp := recorder.Result()

    // Parse and snapshot the stream
    events := expect.ResponseStream(t, resp)

    // Make structured assertions on the events
    require.Len(t, events, 2)

    // First event
    assert.Equal(t, "message", events[0].Event)
    var data1 map[string]string
    json.Unmarshal(events[0].Data, &data1)
    assert.Equal(t, "Hello", data1["text"])

    // Second event
    assert.Equal(t, "update", events[1].Event)
    assert.Equal(t, "evt-123", events[1].ID)
    var data2 map[string]float64
    json.Unmarshal(events[1].Data, &data2)
    assert.Equal(t, 42.0, data2["count"])
}
```

The `SSEEvent` struct provides structured access to event data:

```go
type SSEEvent struct {
    Event string `json:"event,omitempty"` // Event type (from "event:" field)
    Data  []byte `json:"data,omitempty"`  // Raw event data (from "data:" field)
    ID    string `json:"id,omitempty"`    // Event ID (from "id:" field)
}
```

SSE comments (lines starting with `:`) are automatically filtered out during parsing.

### SQL Query Snapshots

Format and snapshot SQL queries with consistent formatting:

```go
func TestSQLQuery(t *testing.T) {
    query := `
        SELECT u.id, u.name, o.total
        FROM users u
        JOIN orders o ON u.id = o.user_id
        WHERE u.active = true
        ORDER BY o.created_at DESC
    `
    expect.SQL(t, query)
}
```

### Mock Call History

Snapshot mock call history for verification:

```go
func TestMockCalls(t *testing.T) {
    mockObj := new(MockService)
    mockObj.On("DoSomething", "arg1", 123).Return(nil)

    // Execute code that uses the mock
    mockObj.DoSomething("arg1", 123)

    // Snapshot the call history
    expect.Calls(t, mockObj.Calls)
}
```

### Structured Log Testing

Snapshot structured log output:

```go
func TestLogs(t *testing.T) {
    core, recorded := observer.New(zap.InfoLevel)
    logger := zap.New(core)

    // Generate some logs
    logger.Info("user created", zap.String("user_id", "123"))
    logger.Info("email sent", zap.String("to", "user@example.com"))

    // Snapshot the logs
    expect.Logs(t, recorded.All())
}
```

### File Content Snapshots

Snapshot file contents:

```go
func TestFileGeneration(t *testing.T) {
    // Generate a file
    generateConfigFile("/tmp/config.json")

    // Snapshot its contents
    expect.File(t, "/tmp/config.json")
}
```

### Custom Extensions

Snapshot with custom file extensions:

```go
func TestHTML(t *testing.T) {
    html := "<html><body><h1>Test</h1></body></html>"
    expect.HTML(t, html)

    // Or use custom extension
    expect.OutputWithExtension(t, ".xml", xmlContent)
}
```

### Diff Output

Compare and snapshot differences between two outputs:

```go
func TestDiff(t *testing.T) {
    before := "version 1.0\nfeature: basic"
    after := "version 2.0\nfeature: advanced"

    expect.OutputDiff(t, before, after)
}
```

## Real-World Examples

### Testing an API Endpoint

```go
func TestUserCreation(t *testing.T) {
    server := setupTestServer(t)

    // Create request
    reqBody := `{"name":"Jane Doe","email":"jane@example.com"}`
    req := httptest.NewRequest(http.MethodPost, "/api/users",
        strings.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")

    // Execute request
    recorder := httptest.NewRecorder()
    server.ServeHTTP(recorder, req)

    // Snapshot the response
    expect.Response(t, recorder.Result())
}
```

### Testing Event Stream Responses

```go
func TestFlowExecution(t *testing.T) {
    server := setupTestServer(t)

    // Execute flow that returns SSE stream
    req := httptest.NewRequest(http.MethodPost, "/api/flows/run",
        strings.NewReader(`{"flow_id":"test-flow"}`))
    recorder := httptest.NewRecorder()
    server.ServeHTTP(recorder, req)

    // Parse and verify stream events
    events := expect.ResponseStream(t, recorder.Result())

    // Verify specific events
    require.Len(t, events, 3)
    assert.Equal(t, "started", events[0].Event)
    assert.Equal(t, "progress", events[1].Event)
    assert.Equal(t, "completed", events[2].Event)
}
```

## Snapshot Management

Snapshots are stored in `.snapshots/` directories next to your test files. The first time a test runs, it creates a snapshot. On subsequent runs, it compares against that snapshot.

To update snapshots when your code changes intentionally:

```bash
# Update all snapshots
UPDATE_SNAPSHOTS=true go test ./...

# Or use the cupaloy environment variable
CUPALOY_UPDATE=true go test ./...
```

## Dependencies

This library uses:

- [cupaloy](https://github.com/bradleyjkemp/cupaloy) for snapshot testing
- [testify](https://github.com/stretchr/testify) for assertions and mocks
- [zap](https://github.com/uber-go/zap) for log testing utilities
- [cockroachdb-parser](https://github.com/cockroachdb/cockroachdb-parser) for SQL formatting
- [sqlfmt](https://github.com/mjibson/sqlfmt) for SQL formatting

## Handling Nondeterminism

Snapshot testing forces you to remove sources of randomness. Here are common sources and how to handle them:

- **UUIDs**: Generate them deterministically in your tests. Use a mock UUID generator or seed-based random number generator.

- **Time**: Never call `time.Now()` directly in code you're snapshotting. Instead, control time in your tests using a time provider interface or mock clock to simulate time progression.

- **Map Iteration**: Go's map iteration order is randomized. Ensure maps are iterated in a consistent order (e.g., sort keys before iteration) to prevent snapshot mismatches.

- **Floating-Point Precision**: On different CPU architectures, floating-point calculations can produce slight variations. Round numbers in your algorithms before snapshotting, especially in ML or scientific computing contexts.

- **Request IDs**: The `Response` function automatically removes headers ending with `-request-id`, but you may need to handle other request-specific identifiers in your application code.

## Best Practices

1. **Use descriptive test names**: Snapshot files are named after your test functions
2. **Keep snapshots small**: Break large outputs into multiple test cases
3. **Review snapshot changes**: Always review snapshot diffs in code review
4. **Don't snapshot timestamps**: Use the built-in header cleanup for volatile data
5. **Combine with assertions**: Use snapshots for overall structure, specific assertions for critical values
6. **Control randomness**: Make your tests deterministic by controlling time, UUIDs, and other sources of randomness

## License

MIT License - See LICENSE file for details
