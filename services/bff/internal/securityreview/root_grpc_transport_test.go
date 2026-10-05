package securityreview

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Root authored. Actual grpc-go HTTP/2 transport on a transient loopback listener.
// No xDS service or application authentication is added; this checks the upstream early rejection.
func TestRootGRPCMissingAuthorityRejected(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := grpc.NewServer()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("gRPC server did not stop")
		}
	}()
	raw, e := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	if e = raw.SetDeadline(time.Now().Add(5 * time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e = raw.Write([]byte(http2.ClientPreface)); e != nil {
		t.Fatal(e)
	}
	framer := http2.NewFramer(raw, raw)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if e = framer.WriteSettings(); e != nil {
		t.Fatal(e)
	}
	var headers bytes.Buffer
	encoder := hpack.NewEncoder(&headers)
	for _, h := range []hpack.HeaderField{{Name: ":method", Value: "POST"}, {Name: ":scheme", Value: "http"}, {Name: ":path", Value: "/weaveos.security.Probe/Check"}, {Name: "content-type", Value: "application/grpc"}, {Name: "te", Value: "trailers"}} {
		if e = encoder.WriteField(h); e != nil {
			t.Fatal(e)
		}
	}
	if e = framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 1, BlockFragment: headers.Bytes(), EndStream: true, EndHeaders: true}); e != nil {
		t.Fatal(e)
	}
	gotStatus, gotGRPC := "", ""
	for i := 0; i < 16 && gotStatus == ""; i++ {
		frame, e := framer.ReadFrame()
		if e != nil {
			t.Fatal(e)
		}
		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				if e = framer.WriteSettingsAck(); e != nil {
					t.Fatal(e)
				}
			}
		case *http2.MetaHeadersFrame:
			if f.StreamID == 1 {
				for _, h := range f.Fields {
					if h.Name == ":status" {
						gotStatus = h.Value
					}
					if h.Name == "grpc-status" {
						gotGRPC = h.Value
					}
				}
			}
		}
	}
	if gotStatus != "400" || gotGRPC != "13" {
		t.Fatalf("missing authority and Host must be rejected by patched transport: HTTP=%q grpc=%q", gotStatus, gotGRPC)
	}
	// A well-formed follow-up reaches the live server rather than observing a crashed process.
	conn, e := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e = conn.Invoke(ctx, "/weaveos.security.Probe/Check", &emptypb.Empty{}, &emptypb.Empty{})
	if status.Code(e) != codes.Unimplemented {
		t.Fatalf("server must remain live after malformed headers: %v", e)
	}
}
