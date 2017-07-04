package http

import (
	"io"
	"net/http"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	context "golang.org/x/net/context"
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
	srv pb.GreeterServer
}

func NewGreeter(srv pb.GreeterServer) *Greeter {
	return &Greeter{srv: srv}
}

func (g *Greeter) HandlerMap() map[string]http.HandlerFunc {
	m := make(map[string]http.HandlerFunc)
	m["/greeter/say_hello"] = MakeHandler(g.SayHello, new(pb.HelloRequest))
	return m
}

func (g *Greeter) SayHello(ctx context.Context, in proto.Message) (proto.Message, error) {
	return g.srv.SayHello(ctx, in.(*pb.HelloRequest))
}

type Server struct {
	mux *http.ServeMux
}

func NewServer(srvGreeter pb.GreeterServer) *Server {
	mux := http.NewServeMux()
	for pattern, handler := range NewGreeter(srvGreeter).HandlerMap() {
		mux.Handle(pattern, handler)
	}
	return &Server{mux: mux}
}

func (s *Server) Serve(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
