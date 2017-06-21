package main

import (
	"github.com/RussellLuo/protoc-go-plugins/protoc-gen-gohttp/generator"
)

func main() {
	g := generator.New()
	g.Generate()
}
