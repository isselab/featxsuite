package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWriteThenReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	out := NewConn(nil, &buf)
	if err := out.Notify("textDocument/didOpen", map[string]string{"uri": "file:///x.ftx"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	in := NewConn(&buf, io.Discard)

	msg, err := in.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if msg.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want %q", msg.JSONRPC, "2.0")
	}
	if msg.Method != "textDocument/didOpen" {
		t.Errorf("Method = %q", msg.Method)
	}
	if !msg.IsNotification() {
		t.Error("IsNotification = false, want true for a message with no id")
	}

	var params map[string]string
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["uri"] != "file:///x.ftx" {
		t.Errorf("params = %v", params)
	}
}

func TestWriteFramesContentLength(t *testing.T) {
	var buf bytes.Buffer

	if err := NewConn(nil, &buf).Write(&Message{Method: "ping"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw := buf.String()

	header, body, found := strings.Cut(raw, "\r\n\r\n")
	if !found {
		t.Fatalf("no blank line separating headers from body in %q", raw)
	}

	if want := "Content-Length: " + itoa(len(body)); header != want {
		t.Errorf("header = %q, want %q", header, want)
	}
}

func TestReadAcceptsLowercaseHeader(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"ping"}`
	raw := "content-length: " + itoa(len(body)) + "\r\n\r\n" + body

	msg, err := NewConn(strings.NewReader(raw), io.Discard).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msg.Method != "ping" {
		t.Errorf("Method = %q, want %q", msg.Method, "ping")
	}
}

func TestReadIgnoresExtraHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"ping"}`
	raw := "Content-Type: application/vscode-jsonrpc; charset=utf-8\r\n" +
		"Content-Length: " + itoa(len(body)) + "\r\n\r\n" + body

	msg, err := NewConn(strings.NewReader(raw), io.Discard).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if msg.Method != "ping" {
		t.Errorf("Method = %q", msg.Method)
	}
}

func TestReadRejectsMissingContentLength(t *testing.T) {
	raw := "X-Thing: 1\r\n\r\n{}"

	if _, err := NewConn(strings.NewReader(raw), io.Discard).Read(); err == nil {
		t.Error("expected an error for a message with no Content-Length")
	}
}

func TestReadRejectsMalformedHeader(t *testing.T) {
	raw := "garbage\r\n\r\n{}"

	if _, err := NewConn(strings.NewReader(raw), io.Discard).Read(); err == nil {
		t.Error("expected an error for a header with no colon")
	}
}

func TestReadReportsEOF(t *testing.T) {
	if _, err := NewConn(strings.NewReader(""), io.Discard).Read(); !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want io.EOF", err)
	}
}

func TestReadShortBody(t *testing.T) {
	raw := "Content-Length: 99\r\n\r\n{}"

	if _, err := NewConn(strings.NewReader(raw), io.Discard).Read(); err == nil {
		t.Error("expected an error when the body is shorter than Content-Length")
	}
}

func TestReplyCarriesID(t *testing.T) {
	var buf bytes.Buffer

	if err := NewConn(nil, &buf).Reply(json.RawMessage("7"), map[string]int{"n": 1}); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	msg, err := NewConn(&buf, io.Discard).Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if string(msg.ID) != "7" {
		t.Errorf("ID = %q, want %q", msg.ID, "7")
	}
	if msg.IsNotification() {
		t.Error("a reply must not look like a notification")
	}
}

func TestReplyErrorCarriesCode(t *testing.T) {
	var buf bytes.Buffer

	err := NewConn(nil, &buf).ReplyError(json.RawMessage("1"), CodeMethodNotFound, "no such method %q", "frob")
	if err != nil {
		t.Fatalf("ReplyError: %v", err)
	}

	msg, readErr := NewConn(&buf, io.Discard).Read()
	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}

	if msg.Error == nil {
		t.Fatal("Error is nil")
	}
	if msg.Error.Code != CodeMethodNotFound {
		t.Errorf("Code = %d, want %d", msg.Error.Code, CodeMethodNotFound)
	}
	if msg.Error.Message != `no such method "frob"` {
		t.Errorf("Message = %q", msg.Error.Message)
	}
}

// itoa avoids pulling strconv into the test for one conversion.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	return string(digits)
}
