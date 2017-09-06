package http

import (
	"io"
	"net"
	"net/http"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
)

var (
	marshaler   = &jsonpb.Marshaler{EnumsAsInts: true, EmitDefaults: true}
	unmarshaler = &jsonpb.Unmarshaler{}
)

type Method func(context.Context, proto.Message) (proto.Message, error)

func MakeHandler(method Method, in proto.Message) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := unmarshaler.Unmarshal(r.Body, in); err != nil {
			if err != io.EOF {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		out, err := method(nil, in)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := marshaler.Marshal(w, out); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

type Greeter struct {
	srv         pb.GreeterServer
	interceptor grpc.UnaryServerInterceptor
}

func NewGreeter(srv pb.GreeterServer, interceptor grpc.UnaryServerInterceptor) *Greeter {
	return &Greeter{srv: srv, interceptor: interceptor}
}

func (g *Greeter) HandlerMap() map[string]http.HandlerFunc {
	m := make(map[string]http.HandlerFunc)
	m["/greeter/say_hello"] = MakeHandler(g.SayHello, new(pb.HelloRequest))
	return m
}

func (g *Greeter) SayHello(ctx context.Context, in proto.Message) (proto.Message, error) {
	if g.interceptor == nil {
		return g.srv.SayHello(ctx, in.(*pb.HelloRequest))
	}
	out, err := g.interceptor(
		ctx,
		in.(*pb.HelloRequest),
		&grpc.UnaryServerInfo{
			Server:     g.srv,
			FullMethod: "/helloworld.Greeter/SayHello",
		},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return g.srv.SayHello(ctx, req.(*pb.HelloRequest))
		},
	)
	return out.(*pb.HelloReply), err
}

type Server struct {
	mux         *http.ServeMux
	interceptor grpc.UnaryServerInterceptor
}

func NewServer(interceptors ...grpc.UnaryServerInterceptor) *Server {
	var interceptor grpc.UnaryServerInterceptor
	switch len(interceptors) {
	case 0:
	case 1:
		interceptor = interceptors[0]
	default:
		panic("At most one unary server interceptor can be set.")
	}

	return &Server{
		mux:         http.NewServeMux(),
		interceptor: interceptor,
	}
}

func (s *Server) RegisterGreeterServer(srvGreeter pb.GreeterServer) {
	for pattern, handler := range NewGreeter(srvGreeter, s.interceptor).HandlerMap() {
		s.mux.Handle(pattern, handler)
	}
}

func (s *Server) Serve(l net.Listener) error {
	server := &http.Server{Handler: s.mux}
	return server.Serve(l)
}
