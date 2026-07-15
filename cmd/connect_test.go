package cmd

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConnectStreamIsNotBoundByRequestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/process.Process/Start" {
			http.NotFound(writer, request)
			return
		}
		time.Sleep(20 * time.Millisecond)
		payload, err := json.Marshal(ProcessStreamFrame{Event: ProcessEvent{
			Start: &ProcessStartEvent{PID: 42, CmdID: "cmd-42"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		header := make([]byte, 5)
		binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
		writer.Header().Set("Content-Type", "application/connect+json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(header)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	service, err := NewService(server.URL, "runtime-token")
	if err != nil {
		t.Fatal(err)
	}
	service.httpClient.Timeout = time.Millisecond
	stream, err := service.Start(context.Background(), &ProcessStartRequest{
		Process: &ProcessConfig{Cmd: "bash"},
	}, nil)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer stream.Close()
	frame, err := stream.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if frame.Event.Start == nil || frame.Event.Start.PID != 42 {
		t.Fatalf("start frame = %#v", frame)
	}
}
