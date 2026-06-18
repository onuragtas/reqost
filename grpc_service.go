package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// GRPCService makes dynamic gRPC calls (unary + streaming) using server
// reflection — no .proto files needed, like grpcurl.
type GRPCService struct {
	emitter EventEmitter

	mu      sync.Mutex
	streams map[string]*grpcStream
}

type grpcStream struct {
	cancel context.CancelFunc
	send   func(string) error // nil for server-streaming
	closeS func() error       // CloseSend for client/bidi
}

func NewGRPCService() *GRPCService { return &GRPCService{streams: map[string]*grpcStream{}} }

func (s *GRPCService) setEmitter(e EventEmitter) { s.emitter = e }

func (s *GRPCService) emit(streamID, typ, data string) {
	if s.emitter == nil {
		return
	}
	s.emitter.Emit("grpc:event", map[string]any{
		"streamId": streamID,
		"type":     typ,
		"data":     data,
		"ts":       time.Now().UnixMilli(),
	})
}

// GRPCResult is the response of a unary call.
type GRPCResult struct {
	Body  string `json:"body"`  // response message as JSON
	Error string `json:"error"` // RPC/error message, if any
}

// parseTarget strips a grpc/grpcs scheme and reports whether TLS is requested.
// "grpcs://host:443" → TLS; "grpc://host:50051" or bare "host:50051" → plaintext.
func parseTarget(target string) (hostport string, useTLS bool) {
	switch {
	case strings.HasPrefix(target, "grpcs://"):
		return strings.TrimPrefix(target, "grpcs://"), true
	case strings.HasPrefix(target, "grpc://"):
		return strings.TrimPrefix(target, "grpc://"), false
	default:
		return target, false
	}
}

func dialGRPC(ctx context.Context, target string) (*grpc.ClientConn, error) {
	hostport, useTLS := parseTarget(target)
	creds := insecure.NewCredentials()
	if useTLS {
		creds = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return grpc.DialContext(ctx, hostport,
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
	)
}

// ListMethods returns fully-qualified method names ("pkg.Service/Method")
// exposed by the server, via reflection.
func (s *GRPCService) ListMethods(target string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialGRPC(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	defer client.Reset()

	services, err := client.ListServices()
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	var out []string
	for _, svc := range services {
		if strings.HasPrefix(svc, "grpc.reflection.") {
			continue
		}
		sd, err := client.ResolveService(svc)
		if err != nil {
			continue
		}
		for _, m := range sd.GetMethods() {
			out = append(out, svc+"/"+m.GetName())
		}
	}
	return out, nil
}

// Call invokes a unary method. fullMethod is "pkg.Service/Method"; reqJSON is the
// request message as JSON. Returns the response message as JSON.
func (s *GRPCService) Call(target, fullMethod, reqJSON string) (*GRPCResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	svcName, methodName, ok := splitMethod(fullMethod)
	if !ok {
		return &GRPCResult{Error: "method must be pkg.Service/Method"}, nil
	}

	conn, err := dialGRPC(ctx, target)
	if err != nil {
		return &GRPCResult{Error: fmt.Sprintf("dial: %v", err)}, nil
	}
	defer conn.Close()

	client := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	defer client.Reset()

	sd, err := client.ResolveService(svcName)
	if err != nil {
		return &GRPCResult{Error: fmt.Sprintf("resolve service %s: %v", svcName, err)}, nil
	}
	md := sd.FindMethodByName(methodName)
	if md == nil {
		return &GRPCResult{Error: fmt.Sprintf("method %s not found", methodName)}, nil
	}
	if md.IsClientStreaming() || md.IsServerStreaming() {
		return &GRPCResult{Error: "only unary methods are supported"}, nil
	}

	reqMsg := dynamic.NewMessage(md.GetInputType())
	if strings.TrimSpace(reqJSON) != "" {
		if err := reqMsg.UnmarshalJSON([]byte(reqJSON)); err != nil {
			return &GRPCResult{Error: fmt.Sprintf("parse request: %v", err)}, nil
		}
	}

	stub := grpcdynamic.NewStub(conn)
	resp, err := stub.InvokeRpc(ctx, md, reqMsg)
	if err != nil {
		return &GRPCResult{Error: err.Error()}, nil
	}
	dm, err := dynamic.AsDynamicMessage(resp)
	if err != nil {
		return &GRPCResult{Error: fmt.Sprintf("decode response: %v", err)}, nil
	}
	out, err := dm.MarshalJSONIndent()
	if err != nil {
		return &GRPCResult{Error: fmt.Sprintf("encode response: %v", err)}, nil
	}
	return &GRPCResult{Body: string(out)}, nil
}

// StreamCall opens a server-streaming (or client/bidi) RPC. Frames arrive on
// the `grpc:event` Wails event; for client-side streams, call StreamSend to
// push messages. StreamClose ends the call.
func (s *GRPCService) StreamCall(streamID, target, fullMethod, reqJSON string) error {
	if streamID == "" {
		return fmt.Errorf("streamID required")
	}
	ctx, cancel := context.WithCancel(context.Background())

	svcName, methodName, ok := splitMethod(fullMethod)
	if !ok {
		cancel()
		return fmt.Errorf("method must be pkg.Service/Method")
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	conn, err := dialGRPC(dialCtx, target)
	dialCancel()
	if err != nil {
		cancel()
		return fmt.Errorf("dial: %w", err)
	}

	client := grpcreflect.NewClientV1Alpha(ctx, reflectpb.NewServerReflectionClient(conn))
	sd, err := client.ResolveService(svcName)
	if err != nil {
		conn.Close()
		client.Reset()
		cancel()
		return fmt.Errorf("resolve service %s: %w", svcName, err)
	}
	md := sd.FindMethodByName(methodName)
	if md == nil {
		conn.Close()
		client.Reset()
		cancel()
		return fmt.Errorf("method %s not found", methodName)
	}

	stub := grpcdynamic.NewStub(conn)
	st := &grpcStream{cancel: func() { cancel(); client.Reset(); conn.Close() }}

	// All four streaming shapes go through the same dispatch — only the API
	// the stub returns differs.
	switch {
	case !md.IsClientStreaming() && md.IsServerStreaming():
		reqMsg := dynamic.NewMessage(md.GetInputType())
		if strings.TrimSpace(reqJSON) != "" {
			if err := reqMsg.UnmarshalJSON([]byte(reqJSON)); err != nil {
				st.cancel()
				return fmt.Errorf("parse request: %w", err)
			}
		}
		stream, err := stub.InvokeRpcServerStream(ctx, md, reqMsg)
		if err != nil {
			st.cancel()
			return fmt.Errorf("server stream: %w", err)
		}
		go s.pumpServerStream(streamID, stream, st)

	case md.IsClientStreaming() && !md.IsServerStreaming():
		stream, err := stub.InvokeRpcClientStream(ctx, md)
		if err != nil {
			st.cancel()
			return fmt.Errorf("client stream: %w", err)
		}
		st.send = func(j string) error {
			m := dynamic.NewMessage(md.GetInputType())
			if err := m.UnmarshalJSON([]byte(j)); err != nil {
				return err
			}
			return stream.SendMsg(m)
		}
		st.closeS = func() error {
			resp, err := stream.CloseAndReceive()
			if err != nil {
				return err
			}
			if dm, dmErr := dynamic.AsDynamicMessage(resp); dmErr == nil {
				out, _ := dm.MarshalJSONIndent()
				s.emit(streamID, "event", string(out))
			}
			s.emit(streamID, "close", "EOF")
			return nil
		}

	case md.IsClientStreaming() && md.IsServerStreaming():
		stream, err := stub.InvokeRpcBidiStream(ctx, md)
		if err != nil {
			st.cancel()
			return fmt.Errorf("bidi stream: %w", err)
		}
		st.send = func(j string) error {
			m := dynamic.NewMessage(md.GetInputType())
			if err := m.UnmarshalJSON([]byte(j)); err != nil {
				return err
			}
			return stream.SendMsg(m)
		}
		st.closeS = func() error { return stream.CloseSend() }
		go func() {
			for {
				msg, err := stream.RecvMsg()
				if err == io.EOF {
					s.emit(streamID, "close", "EOF")
					return
				}
				if err != nil {
					s.emit(streamID, "error", err.Error())
					return
				}
				if dm, dmErr := dynamic.AsDynamicMessage(msg); dmErr == nil {
					out, _ := dm.MarshalJSONIndent()
					s.emit(streamID, "event", string(out))
				}
			}
		}()
	default:
		st.cancel()
		return fmt.Errorf("method is unary — use Call instead")
	}

	s.mu.Lock()
	s.streams[streamID] = st
	s.mu.Unlock()
	s.emit(streamID, "open", fullMethod)
	return nil
}

// pumpServerStream forwards each Recv as a grpc:event until EOF/error.
func (s *GRPCService) pumpServerStream(streamID string, stream *grpcdynamic.ServerStream, st *grpcStream) {
	defer func() {
		s.mu.Lock()
		delete(s.streams, streamID)
		s.mu.Unlock()
	}()
	for {
		msg, err := stream.RecvMsg()
		if err == io.EOF {
			s.emit(streamID, "close", "EOF")
			return
		}
		if err != nil {
			s.emit(streamID, "error", err.Error())
			return
		}
		if dm, dmErr := dynamic.AsDynamicMessage(msg); dmErr == nil {
			out, _ := dm.MarshalJSONIndent()
			s.emit(streamID, "event", string(out))
		}
	}
}

// StreamSend pushes a JSON message into a client/bidi stream.
func (s *GRPCService) StreamSend(streamID, msgJSON string) error {
	s.mu.Lock()
	st := s.streams[streamID]
	s.mu.Unlock()
	if st == nil || st.send == nil {
		return fmt.Errorf("no client-streaming RPC with id %q", streamID)
	}
	return st.send(msgJSON)
}

// StreamCloseSend signals end-of-input on a client/bidi stream.
func (s *GRPCService) StreamCloseSend(streamID string) error {
	s.mu.Lock()
	st := s.streams[streamID]
	s.mu.Unlock()
	if st == nil || st.closeS == nil {
		return fmt.Errorf("no client-streaming RPC with id %q", streamID)
	}
	return st.closeS()
}

// StreamCancel hard-cancels a running stream.
func (s *GRPCService) StreamCancel(streamID string) {
	s.mu.Lock()
	st := s.streams[streamID]
	delete(s.streams, streamID)
	s.mu.Unlock()
	if st != nil {
		st.cancel()
	}
}

func splitMethod(full string) (svc, method string, ok bool) {
	full = strings.TrimPrefix(full, "/")
	if i := strings.LastIndex(full, "/"); i != -1 {
		return full[:i], full[i+1:], true
	}
	// Also accept "pkg.Service.Method".
	if i := strings.LastIndex(full, "."); i != -1 {
		return full[:i], full[i+1:], true
	}
	return "", "", false
}
