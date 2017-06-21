package generator

import (
	"fmt"
	"path"

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
	name := *protoName
	if ext := path.Ext(name); ext == ".proto" || ext == ".protodevel" {
		name = name[:len(name)-len(ext)]
	}
	name += ".http.go"
	return name
}

func (g *generator) httpServiceName(serviceName string) string {
	return serviceName + "HTTP"
}

func (g *generator) generateImports() {
	g.P("package http")
	g.P()
	g.P("import (")
	g.In()
	g.P(`"encoding/json"`)
	g.P(`"net/http"`)
	g.P()
	g.P(`"`, g.Param["pb_pkg_path"], `"`)
	g.P(`context "golang.org/x/net/context"`)
	g.Out()
	g.P(")")
}

func (g *generator) generateMethodInterface() {
	g.P()
	g.P("type Method func(context.Context, interface{}) (interface{}, error)")
}

func (g *generator) generateMakeHandlerFunc() {
	g.P()
	g.P(`func MakeHandler(method Method, in interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func (g *generator) generateHTTPService(serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	g.generateStructure(serviceName)
	g.generateNewFunc(serviceName)
	g.generateHandlerMapMethod(serviceName, methods)
	g.generateWrapperMethods(serviceName, methods)
}

func (g *generator) generateStructure(serviceName string) {
	g.P()
	g.P("type ", g.httpServiceName(serviceName), " struct {")
	g.In()
	g.P("srv pb.", serviceName, "Server")
	g.Out()
	g.P("}")
}

func (g *generator) generateNewFunc(serviceName string) {
	httpServiceName := g.httpServiceName(serviceName)
	g.P()
	g.P("func New", httpServiceName, "(srv pb.", serviceName, "Server) *", httpServiceName, " {")
	g.In()
	g.P("return &", httpServiceName, "{srv: srv}")
	g.Out()
	g.P("}")
}

func (g *generator) generateHandlerMapMethod(serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	g.P()
	g.P("func (h *", g.httpServiceName(serviceName), ") HandlerMap() map[string]http.HandlerFunc {")
	g.In()
	g.P("m := make(map[string]http.HandlerFunc)")

	for _, method := range methods {
		inputTypeName := g.TypeName(method.GetInputType())
		methodName := method.GetName()
		pattern := fmt.Sprintf("/%s/%s", g.Underscore(serviceName), g.Underscore(methodName))
		g.P(`m["`, pattern, `"] = `, "MakeHandler(h.", methodName, ", new(pb.", inputTypeName, "))")
	}

	g.P("return m")
	g.Out()
	g.P("}")
}

func (g *generator) generateWrapperMethods(serviceName string, methods []*google_protobuf.MethodDescriptorProto) {
	for _, method := range methods {
		inputTypeName := g.TypeName(method.GetInputType())
		g.P()
		g.P("func (h *", g.httpServiceName(serviceName), ") ", method.Name, "(ctx context.Context, in interface{}) (interface{}, error) {")
		g.In()
		g.P("return h.srv.", method.Name, "(ctx, in.(*pb.", inputTypeName, "))")
		g.Out()
		g.P("}")
	}
}

func (g *generator) validateParameters() {
	if _, ok := g.Param["pb_pkg_path"]; !ok {
		g.Fail("parameter `pb_pkg_path` is required (e.g. --gohttp_out=pb_pkg_path=<pb package path>:<proto file path>)")
	}
}

func (g *generator) Make(protoFile *google_protobuf.FileDescriptorProto) (*plugin.CodeGeneratorResponse_File, error) {
	g.validateParameters()

	g.generateImports()
	g.generateMethodInterface()
	g.generateMakeHandlerFunc()
	for _, service := range protoFile.Service {
		serviceName := gen.CamelCase(service.GetName())
		g.generateHTTPService(serviceName, service.Method)
	}
	file := &plugin.CodeGeneratorResponse_File{
		Name:    proto.String(g.goFileName(protoFile.Name)),
		Content: proto.String(g.String()),
	}
	return file, nil
}

func (g *generator) Generate() {
	g.Generator.Generate(g)
}
