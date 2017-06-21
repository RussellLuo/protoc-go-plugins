package base

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	gen "github.com/golang/protobuf/protoc-gen-go/generator"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

var camel = regexp.MustCompile("(^[^A-Z0-9]*|[A-Z0-9]*)([A-Z0-9][^A-Z]+|$)")

type fileMaker interface {
	Make(*google_protobuf.FileDescriptorProto) (*plugin.CodeGeneratorResponse_File, error)
}

type Generator struct {
	*gen.Generator
	indent string

	reader io.Reader
	writer io.Writer
}

// New creates a new base generator
func New() *Generator {
	return &Generator{
		Generator: gen.New(),
		reader:    os.Stdin,
		writer:    os.Stdout,
	}
}

// P prints the arguments to the generated output.  It handles strings and int32s, plus
// handling indirections because they may be *string, etc.
func (g *Generator) P(str ...interface{}) {
	g.WriteString(g.indent)
	for _, v := range str {
		switch s := v.(type) {
		case string:
			g.WriteString(s)
		case *string:
			g.WriteString(*s)
		case bool:
			fmt.Fprintf(g, "%t", s)
		case *bool:
			fmt.Fprintf(g, "%t", *s)
		case int:
			fmt.Fprintf(g, "%d", s)
		case *int32:
			fmt.Fprintf(g, "%d", *s)
		case *int64:
			fmt.Fprintf(g, "%d", *s)
		case float64:
			fmt.Fprintf(g, "%g", s)
		case *float64:
			fmt.Fprintf(g, "%g", *s)
		default:
			g.Fail(fmt.Sprintf("unknown type in printer: %T", v))
		}
	}
	g.WriteByte('\n')
}

// In Indents the output one tab stop.
func (g *Generator) In() { g.indent += "\t" }

// Out unindents the output one tab stop.
func (g *Generator) Out() {
	if len(g.indent) > 0 {
		g.indent = g.indent[1:]
	}
}

// Error reports a problem, including an error, and exits the program.
func (g *Generator) Error(err error, msgs ...string) {
	s := strings.Join(msgs, " ") + ":" + err.Error()
	log.Print("protoc-gen-gohttp: error:", s)
	os.Exit(1)
}

// Fail reports a problem and exits the program.
func (g *Generator) Fail(msgs ...string) {
	s := strings.Join(msgs, " ")
	log.Print("protoc-gen-gohttp: error:", s)
	os.Exit(1)
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (g *Generator) objectNamed(name string) gen.Object {
	g.RecordTypeUse(name)
	return g.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (g *Generator) TypeName(str string) string {
	return g.Generator.TypeName(g.objectNamed(str))
}

// sideEffect calls some methods of the embedded generator from protoc-gen-go
// to make it possible to get object name by type name (via TypeName).
func (g *Generator) sideEffect() {
	g.CommandLineParameters(g.Request.GetParameter())

	g.WrapTypes()

	g.SetPackageNames()
	g.BuildTypeNameMap()

	g.GenerateAllFiles()

	// Resets the buffer to be empty
	// to ignore the output of the embedded generator
	g.Reset()
}

func (g *Generator) Underscore(s string) string {
	var a []string
	for _, sub := range camel.FindAllStringSubmatch(s, -1) {
		if sub[1] != "" {
			a = append(a, sub[1])
		}
		if sub[2] != "" {
			a = append(a, sub[2])
		}
	}
	return strings.ToLower(strings.Join(a, "_"))
}

func (g *Generator) generate(maker fileMaker, request *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	response := new(plugin.CodeGeneratorResponse)
	for _, protoFile := range request.ProtoFile {
		file, err := maker.Make(protoFile)
		if err != nil {
			return response, err
		}
		response.File = append(response.File, file)
	}
	return response, nil
}

func (g *Generator) Generate(maker fileMaker) {
	input, err := ioutil.ReadAll(g.reader)
	if err != nil {
		g.Error(err, "reading input")
	}

	request := g.Request
	if err := proto.Unmarshal(input, request); err != nil {
		g.Error(err, "parsing input proto")
	}

	if len(request.FileToGenerate) == 0 {
		g.Fail("no files to generate")
	}

	g.sideEffect()

	response, err := g.generate(maker, request)
	if err != nil {
		g.Error(err, "failed to generate files from proto")
	}

	output, err := proto.Marshal(response)
	if err != nil {
		g.Error(err, "failed to marshal output proto")
	}
	_, err = g.writer.Write(output)
	if err != nil {
		g.Error(err, "failed to write output proto")
	}
}
