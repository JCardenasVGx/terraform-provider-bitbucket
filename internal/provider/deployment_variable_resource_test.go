package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockDeploymentVariableServer(t *testing.T) *httptest.Server {
	t.Helper()

	type variable struct {
		UUID    string `json:"uuid,omitempty"`
		Key     string `json:"key,omitempty"`
		Value   string `json:"value,omitempty"`
		Secured bool   `json:"secured"`
	}

	var mu sync.Mutex
	store := map[string]variable{}
	nextID := 1

	basePath := "/repositories/acc-test-workspace/acc-repo/deployments_config/environments/acc-env/variables"

	mux := http.NewServeMux()
	mux.HandleFunc(basePath, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body variable
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			body.UUID = "depvar-" + strconv.Itoa(nextID)
			nextID++
			store[body.UUID] = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(body)
		case http.MethodGet:
			// Read() has no get-by-id endpoint: it pages through this list
			// looking for a matching UUID.
			mu.Lock()
			values := make([]variable, 0, len(store))
			for _, v := range store {
				values = append(values, v)
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"values": values})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(basePath+"/", func(w http.ResponseWriter, r *http.Request) {
		uuid := strings.TrimPrefix(r.URL.Path, basePath+"/")

		switch r.Method {
		case http.MethodPut:
			var body variable
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			body.UUID = uuid
			mu.Lock()
			store[uuid] = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case http.MethodDelete:
			mu.Lock()
			delete(store, uuid)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAccDeploymentVariableResource(t *testing.T) {
	srv := newMockDeploymentVariableServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_deployment_variable" "test" {
  repository_slug  = "acc-repo"
  environment_uuid = "acc-env"
  key              = "DB_URL"
  value            = "postgres://prod-example"
  secured          = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_deployment_variable.test", "key", "DB_URL"),
					resource.TestCheckResourceAttr("bitbucket_deployment_variable.test", "value", "postgres://prod-example"),
					resource.TestCheckResourceAttr("bitbucket_deployment_variable.test", "secured", "true"),
				),
			},
			{
				ResourceName:            "bitbucket_deployment_variable.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
				ImportStateIdFunc:       importStateIDFromAttrs("bitbucket_deployment_variable.test", []string{"repository_slug", "environment_uuid", "variable_uuid"}, "/"),
			},
		},
	})
}
