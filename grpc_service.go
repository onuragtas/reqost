package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

// GRPCService makes dynamic unary gRPC calls using server reflection — no .proto
// files needed, like grpcurl. Plaintext only for now (TLS comes later).
type GRPCService struct{}

func NewGRPCService() *GRPCService { return &GRPCService{} }

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
