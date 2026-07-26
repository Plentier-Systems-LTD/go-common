package voice

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewWhisperClientRequiresAPIKey(t *testing.T) {
	if _, err := NewWhisperClient(Config{}); err != ErrNoAPIKey {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}

func TestTranscribe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("unexpected Authorization header: %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parsing multipart form: %v", err)
		}
		if model := r.FormValue("model"); model != "whisper-1" {
			t.Errorf("unexpected model field: %q", model)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("reading file field: %v", err)
		}
		defer file.Close() //nolint:errcheck

		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("reading file contents: %v", err)
		}
		if string(data) != "fake-audio-bytes" {
			t.Errorf("unexpected audio contents: %q", string(data))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text": "hello world"}`))
	}))
	defer server.Close()

	client, err := NewWhisperClient(Config{APIKey: "test-key", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Point the client at the test server instead of the real OpenAI
	// endpoint by constructing the request the same way Transcribe does,
	// via a wrapping RoundTripper.
	client.httpClient = &http.Client{Transport: rewriteHost(server.URL)}

	text, err := client.Transcribe(context.Background(), TranscribeRequest{
		Filename: "recording.m4a",
		Audio:    strings.NewReader("fake-audio-bytes"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("unexpected transcription: %q", text)
	}
}

// rewriteHost redirects every request to the given base URL, so Transcribe's
// hardcoded OpenAI endpoint can be exercised against an httptest server.
type rewriteHost string

func (h rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := req.URL.Parse(string(h))
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL = target
	req.Host = target.Host
	return http.DefaultTransport.RoundTrip(req)
}
