package generator

import (
	"fmt"

	"github.com/RussellLuo/protoc-go-plugins/base"
	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	gen "github.com/golang/protobuf/protoc-gen-go/generator"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

type generator struct {
	*base.Generator
}

func New() *generator {
	return &generator{Generator: base.New()}
}

func (g *generator) goFileName(protoName *string) string {
	return g.ProtoFileBaseName(*protoName) + ".http.go"
}

func (g *generator) generatePackageName() {
	g.P("package http")
}

func (g *generator) generateImports() {
	g.P(fmt.Sprintf(`
import (
	"io"
	"net"
	"net/http"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	pb "%s"
)`, g.Param["pb_pkg_path"]))
}

func (g *generator) generateVariables() {
	g.P(`
var (
	marshaler   = &jsonpb.Marshaler{EnumsAsInts: true, EmitDefaults: true}
	unmarshaler = &jsonpb.Unmarshaler{}
)`)
}

func (g *generator) generateMethodInterface() {
	g.P()
	g.P("type Method func(context.Context, proto.Message) (proto.Message, error)")
}

func (g *generator) generateMakeHandlerFunc() {
	g.P(`
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
}`)
}

func (g *generator) generateService(fullServiceName, serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	g.genServiceStructure(serviceName)
	g.genServiceNewFunc(serviceName)
	g.genServiceHandlerMapMethod(serviceName, methods)
	g.genServiceWrapperMethods(fullServiceName, serviceName, methods)
}

func (g *generator) genServiceStructure(serviceName string) {
	g.P()
	g.P("type ", serviceName, " struct {")
	g.In()
	g.P("srv         pb.", serviceName, "Server")
	g.P("interceptor grpc.UnaryServerInterceptor")
	g.Out()
	g.P("}")
}

func (g *generator) genServiceNewFunc(serviceName string) {
	g.P()
	g.P("func New", serviceName, "(srv pb.", serviceName, "Server, interceptor grpc.UnaryServerInterceptor) *", serviceName, " {")
	g.In()
	g.P("return &", serviceName, "{srv: srv, interceptor: interceptor}")
	g.Out()
	g.P("}")
}

func (g *generator) genServiceHandlerMapMethod(serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	receiverName := g.ReceiverName(serviceName)

	g.P()
	g.P("func (", receiverName, " *", serviceName, ") HandlerMap() map[string]http.HandlerFunc {")
	g.In()
	g.P("m := make(map[string]http.HandlerFunc)")

	for _, method := range methods {
		inputTypeName := g.TypeName(method.GetInputType())
		methodName := method.GetName()
		pattern := fmt.Sprintf("/%s/%s", g.Underscore(serviceName), g.Underscore(methodName))
		g.P(`m["`, pattern, `"] = `, "MakeHandler(", receiverName, ".", methodName, ", new(pb.", inputTypeName, "))")
	}

	g.P("return m")
	g.Out()
	g.P("}")
}

func (g *generator) genServiceWrapperMethods(fullServiceName, serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	receiverName := g.ReceiverName(serviceName)

	for _, method := range methods {
		inputTypeName := "*pb." + g.TypeName(method.GetInputType())
		outputTypeName := "*pb." + g.TypeName(method.GetOutputType())
		methodName := method.GetName()

		g.P()
		g.P("func (", receiverName, " *", serviceName, ") ", methodName, "(ctx context.Context, in proto.Message) (proto.Message, error) {")
		g.In()
		g.P("if ", receiverName, ".interceptor == nil {")
		g.In()
		g.P("return ", receiverName, ".srv.", method.Name, "(ctx, in.(", inputTypeName, "))")
		g.Out()
		g.P("}")
		g.P("out, err := ", receiverName, ".interceptor(")
		g.In()
		g.P("ctx,")
		g.P("in.(", inputTypeName, "),")
		g.P("&grpc.UnaryServerInfo{")
		g.In()
		g.P("Server:     ", receiverName, ".srv,")
		g.P(`FullMethod: "`, fmt.Sprintf("/%s/%s", fullServiceName, methodName), `",`)
		g.Out()
		g.P("},")
		g.P("func(ctx context.Context, req interface{}) (interface{}, error) {")
		g.In()
		g.P("return ", receiverName, ".srv.", method.Name, "(ctx, req.(", inputTypeName, "))")
		g.Out()
		g.P("},")
		g.Out()
		g.P(")")
		g.P("return out.(", outputTypeName, "), err")
		g.Out()
		g.P("}")
	}
}

func (g *generator) generateServer(serviceNames []string) {
	g.genServerStructure()
	g.genServerNewFunc()
	g.genServerRegisterMethods(serviceNames)
	g.genServerServeMethod()
}

func (g *generator) genServerStructure() {
	g.P(`
type Server struct {
	mux         *http.ServeMux
	interceptor grpc.UnaryServerInterceptor
}`)
}

func (g *generator) genServerNewFunc() {
	g.P(`
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
}`)
}

func (g *generator) genServerRegisterMethods(serviceNames []string) {
	for _, serviceName := range serviceNames {
		g.P()
		g.P("func (s *Server) Register", serviceName, "Server(srv", serviceName, " pb.", serviceName, "Server) {")
		g.In()
		g.P("for pattern, handler := range New", serviceName, "(srv", serviceName, ", s.interceptor).HandlerMap() {")
		g.In()
		g.P("s.mux.Handle(pattern, handler)")
		g.Out()
		g.P("}")
		g.Out()
		g.P("}")
	}
}

func (g *generator) genServerServeMethod() {
	g.P(`
func (s *Server) Serve(l net.Listener) error {
	server := &http.Server{Handler: s.mux}
	return server.Serve(l)
}`)
}

func (g *generator) genServerWithUnaryInterceptor() {
	g.P(`
func (s *Server) WithUnaryInterceptor(i grpc.UnaryServerInterceptor) {
	server := &http.Server{Handler: s.mux}
	return server.Serve(l)
}`)
}

func (g *generator) validateParameters() {
	if _, ok := g.Param["pb_pkg_path"]; !ok {
		g.Fail("parameter `pb_pkg_path` is required (e.g. --gohttp_out=pb_pkg_path=<pb package path>:<output path>)")
	}
}

func (g *generator) getFullServiceName(packageName, originalServiceName string) string {
	if packageName != "" {
		return packageName + "." + originalServiceName
	}
	return originalServiceName
}

func (g *generator) Make(protoFile *google_protobuf.FileDescriptorProto) (*plugin.CodeGeneratorResponse_File, error) {
	g.validateParameters()

	g.generatePackageName()
	g.generateImports()
	g.generateVariables()
	g.generateMethodInterface()
	g.generateMakeHandlerFunc()

	packageName := protoFile.GetPackage()
	serviceNames := make([]string, len(protoFile.Service))
	for i, service := range protoFile.Service {
		fullServiceName := g.getFullServiceName(packageName, service.GetName())
		serviceNames[i] = gen.CamelCase(service.GetName())
		g.generateService(fullServiceName, serviceNames[i], service.Method)
	}

	g.generateServer(serviceNames)

	file := &plugin.CodeGeneratorResponse_File{
		Name:    proto.String(g.goFileName(protoFile.Name)),
		Content: proto.String(g.String()),
	}
	return file, nil
}

func (g *generator) Generate() {
	g.Generator.Generate(g)
}
