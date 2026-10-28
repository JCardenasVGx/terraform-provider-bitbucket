package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccProtoV6ProviderFactories wires this package's provider into
// terraform-plugin-testing's acceptance test runner. Every acceptance test
// in this package points the provider at a local httptest mock server (via
// BITBUCKET_BASE_URL) instead of the real Bitbucket Cloud API, so these
// tests never touch production and need no real credentials.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"bitbucket": providerserver.NewProtocol6WithError(New("test")()),
}

// importStateIDFromAttrs builds an import ID by joining the named
// attributes of a resource's current state with sep, for resources whose
// import ID is a compound of several fields (e.g. "repository_slug/uuid")
// that aren't known until after Create.
func importStateIDFromAttrs(resourceAddr string, attrs []string, sep string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceAddr]
		if !ok {
			return "", fmt.Errorf("resource %s not found in state", resourceAddr)
		}
		parts := make([]string, len(attrs))
		for i, a := range attrs {
			v, ok := rs.Primary.Attributes[a]
			if !ok {
				return "", fmt.Errorf("attribute %s not found on %s", a, resourceAddr)
			}
			parts[i] = v
		}
		return strings.Join(parts, sep), nil
	}
}
