package generator

import (
	"fmt"
	"strings"

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
	"encoding/json"
	"net/http"

	"%s"
	context "golang.org/x/net/context"
)`, g.Param["pb_pkg_path"]))
}

func (g *generator) generateMethodInterface() {
	g.P()
	g.P("type Method func(context.Context, interface{}) (interface{}, error)")
}

func (g *generator) generateMakeHandlerFunc() {
	g.P(`
func MakeHandler(method Method, in interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		out, err := method(nil, in)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bytes, err := json.Marshal(out)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)
	}
}`)
}

func (g *generator) generateService(serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	g.genServiceStructure(serviceName)
	g.genServiceNewFunc(serviceName)
	g.genServiceHandlerMapMethod(serviceName, methods)
	g.genServiceWrapperMethods(serviceName, methods)
}

func (g *generator) genServiceStructure(serviceName string) {
	g.P()
	g.P("type ", serviceName, " struct {")
	g.In()
	g.P("srv pb.", serviceName, "Server")
	g.Out()
	g.P("}")
}

func (g *generator) genServiceNewFunc(serviceName string) {
	g.P()
	g.P("func New", serviceName, "(srv pb.", serviceName, "Server) *", serviceName, " {")
	g.In()
	g.P("return &", serviceName, "{srv: srv}")
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

func (g *generator) genServiceWrapperMethods(serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	receiverName := g.ReceiverName(serviceName)

	for _, method := range methods {
		inputTypeName := g.TypeName(method.GetInputType())
		g.P()
		g.P("func (", receiverName, " *", serviceName, ") ", method.Name, "(ctx context.Context, in interface{}) (interface{}, error) {")
		g.In()
		g.P("return ", receiverName, ".srv.", method.Name, "(ctx, in.(*pb.", inputTypeName, "))")
		g.Out()
		g.P("}")
	}
}

func (g *generator) generateServer(serviceNames []string) {
	g.genServerStructure()
	g.genServerNewFunc(serviceNames)
	g.genServerServeMethod()
}

func (g *generator) genServerStructure() {
	g.P(`
type Server struct {
	mux *http.ServeMux
}`)
}

func (g *generator) genServerNewFunc(serviceNames []string) {
	g.P()

	args := make([]string, len(serviceNames))
	for i, serviceName := range serviceNames {
		args[i] = fmt.Sprintf("srv%s pb.%sServer", serviceName, serviceName)
	}
	g.P("func NewServer(", strings.Join(args, ", "), ") *Server {")

	g.In()
	g.P("mux := http.NewServeMux()")

	for _, serviceName := range serviceNames {
		g.P("for pattern, handler := range New", serviceName, "(srv", serviceName, ").HandlerMap() {")
		g.In()
		g.P("mux.Handle(pattern, handler)")
		g.Out()
		g.P("}")
	}

	g.P("return &Server{mux: mux}")
	g.Out()
	g.P("}")
}

func (g *generator) genServerServeMethod() {
	g.P(`
func (s *Server) Serve(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}`)
}

func (g *generator) validateParameters() {
	if _, ok := g.Param["pb_pkg_path"]; !ok {
		g.Fail("parameter `pb_pkg_path` is required (e.g. --gohttp_out=pb_pkg_path=<pb package path>:<output path>)")
	}
}

func (g *generator) Make(protoFile *google_protobuf.FileDescriptorProto) (*plugin.CodeGeneratorResponse_File, error) {
	g.validateParameters()

	g.generatePackageName()
	g.generateImports()
	g.generateMethodInterface()
	g.generateMakeHandlerFunc()

	serviceNames := make([]string, len(protoFile.Service))
	for i, service := range protoFile.Service {
		serviceNames[i] = gen.CamelCase(service.GetName())
		g.generateService(serviceNames[i], service.Method)
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
