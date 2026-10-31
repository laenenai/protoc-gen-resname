// protoc-gen-resname is a protoc/buf plugin generating typed Go resource
// names from google.api.resource annotations. See docs/spec.md.
package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/laenenai/protoc-gen-resname/internal/plugin"
)

// version is set with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("protoc-gen-resname %s\n", version)
		return
	}
	cfg := plugin.DefaultConfig()
	protogen.Options{ParamFunc: cfg.Set}.Run(func(gen *protogen.Plugin) error {
		return plugin.Run(gen, cfg, plugin.Env{Version: version, Warnings: os.Stderr})
	})
}
