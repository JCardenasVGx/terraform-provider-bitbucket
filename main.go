package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/JCardenasVGx/terraform-provider-bitbucket/internal/provider"
)

// version is set via -ldflags at build time (e.g. by goreleaser).
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/JCardenasVGx/bitbucket",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
