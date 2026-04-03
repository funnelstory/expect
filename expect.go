// Package expect provides helpers for test snapshots.
package expect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest/observer"
)

// File reads the contents of a file and snapshots it for testing.
// It reads the file at filePath and compares it against a stored snapshot.
func File(t *testing.T, filePath string) {
	t.Helper()
	b, err := os.ReadFile(filePath)
	if err != nil {
		t.Error(err)
		return
	}

	Output(t, string(b))
}

// Output snapshots the given value for testing.
// It compares the output against a stored snapshot.
func Output(t *testing.T, out any) {
	t.Helper()
	cupaloy.SnapshotT(t, out)
}

// JSON marshals the given value to JSON and snapshots it for testing.
// The JSON is formatted with indentation for readability.
func JSON(t *testing.T, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		t.Error(err)
		return
	}

	Output(t, b)
}

// HTML snapshots the given value with an .html file extension for testing.
// This is useful for snapshotting HTML content that should be viewed in a browser.
func HTML(t *testing.T, out any) {
	t.Helper()
	OutputWithExtension(t, ".html", out)
}

// OutputWithExtension snapshots the given value with a custom file extension.
// The extension parameter specifies the file extension to use for the snapshot file.
func OutputWithExtension(t *testing.T, ext string, v any) {
	t.Helper()
	cupaloy.New(cupaloy.SnapshotFileExtension(ext)).SnapshotT(t, v)
}

// Request snapshots an HTTP request for testing.
// It dumps the request including headers and body, normalizes line endings,
// and compares it against a stored snapshot.
func Request(t *testing.T, req *http.Request) {
	t.Helper()
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		t.Fatal(err)
	}
	reqDump = bytes.ReplaceAll(reqDump, []byte("\r\n"), []byte("\n"))
	cupaloy.SnapshotT(t, string(reqDump))
}

// Response snapshots an HTTP response for testing.
// It removes volatile headers (date, Vary, CORS headers, request IDs) before
// dumping the response. For image content types, only headers are included.
func Response(t *testing.T, resp *http.Response) {
	t.Helper()
	// Cleanup headers
	resp.Header.Del("date")
	resp.Header.Del("Vary")
	resp.Header.Del("Access-Control-Allow-Credentials")
	resp.Header.Del("Access-Control-Allow-Origin")

	// Remove any header ending with "-request-id" (e.g., fs-request-id, x-request-id, etc.)
	for key := range resp.Header {
		if strings.HasSuffix(strings.ToLower(key), "-request-id") {
			resp.Header.Del(key)
		}
	}

	var (
		respDump []byte
		err      error
	)

	if resp.Header.Get("Content-Type") == "image/jpeg" || resp.Header.Get("Content-Type") == "image/svg+xml" {
		respDump, err = httputil.DumpResponse(resp, false)
	} else {
		respDump, err = httputil.DumpResponse(resp, true)
	}
	if err != nil {
		t.Fatal(err)
	}
	respDump = bytes.ReplaceAll(respDump, []byte("\r\n"), []byte("\n"))
	cupaloy.SnapshotT(t, string(respDump))
}

// OutputDiff generates a unified diff between two strings and snapshots it.
// This is useful for comparing two outputs and seeing their differences.
func OutputDiff(t *testing.T, out1, out2 string) {
	t.Helper()
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(out1),
		B:        difflib.SplitLines(out2),
		FromFile: "Previous",
		FromDate: "",
		ToFile:   "Current",
		ToDate:   "",
		Context:  1,
	})
	Output(t, diff)
}

// Logs snapshots a slice of logged entries from a zap logger observer.
// Each log entry is formatted as "message\t{json_fields}\n" for easy comparison.
func Logs(t *testing.T, logs []observer.LoggedEntry) {
	t.Helper()
	output := &strings.Builder{}

	for _, log := range logs {
		fieldsMap := log.ContextMap()
		fields, _ := json.Marshal(fieldsMap)

		output.WriteString(log.Message + "\t" + string(fields) + "\n")
	}

	Output(t, output.String())
}

// Calls snapshots a slice of mock calls as JSON.
// Each call is represented with its method name and arguments for comparison.
func Calls(t *testing.T, calls []mock.Call) {
	t.Helper()
	callsJSON := []any{}
	for _, call := range calls {
		callsJSON = append(callsJSON, map[string]any{
			"method":    call.Method,
			"arguments": call.Arguments,
		})
	}
	JSON(t, callsJSON)
}

// SSEEvent represents a Server-Sent Events message with structured data.
type SSEEvent struct {
	// Event is the event type (optional, from "event:" field)
	Event string `json:"event,omitempty"`
	// Data is the raw event data (from "data:" field)
	Data []byte `json:"data,omitempty"`
	// ID is the event ID (optional, from "id:" field)
	ID string `json:"id,omitempty"`
}

// ResponseStream reads an SSE stream response, snapshots it, and returns parsed JSON events for additional assertions.
// The returned events can be used for structured assertions on the stream data.
func ResponseStream(t *testing.T, resp *http.Response) []SSEEvent {
	t.Helper()

	// Read the entire body first
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Restore body for Response() to consume
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Snapshot the entire response
	Response(t, resp)

	// Parse SSE events from the body
	scanner := bufio.NewScanner(bytes.NewReader(bodyBytes))
	var events []SSEEvent
	var currentEvent SSEEvent

	for scanner.Scan() {
		line := scanner.Text()

		// Comments in SSE start with ":"
		if strings.HasPrefix(line, ":") {
			continue
		}

		if value, ok := strings.CutPrefix(line, "event:"); ok {
			currentEvent.Event = strings.TrimSpace(value)
		} else if value, ok := strings.CutPrefix(line, "data:"); ok {
			dataStr := strings.TrimSpace(value)
			if dataStr != "" {
				currentEvent.Data = []byte(dataStr)
			}
		} else if value, ok := strings.CutPrefix(line, "id:"); ok {
			currentEvent.ID = strings.TrimSpace(value)
		} else if line == "" {
			// Empty line signals end of event
			if len(currentEvent.Data) > 0 {
				events = append(events, currentEvent)
				currentEvent = SSEEvent{}
			}
		}
	}

	// Add last event if it didn't end with empty line
	if len(currentEvent.Data) > 0 {
		events = append(events, currentEvent)
	}

	return events
}
