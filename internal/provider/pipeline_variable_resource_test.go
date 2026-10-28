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

func newMockPipelineVariableServer(t *testing.T) *httptest.Server {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/pipelines_config/variables", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body variable
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		body.UUID = "uuid-" + strconv.Itoa(nextID)
		nextID++
		store[body.UUID] = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(body)
	})
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/pipelines_config/variables/", func(w http.ResponseWriter, r *http.Request) {
		uuid := strings.TrimPrefix(r.URL.Path, "/repositories/acc-test-workspace/acc-repo/pipelines_config/variables/")

		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			v, ok := store[uuid]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
				return
			}
			out := v
			if out.Secured {
				out.Value = "" // Bitbucket never returns secured values.
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
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

func TestAccPipelineVariableResource(t *testing.T) {
	srv := newMockPipelineVariableServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_pipeline_variable" "test" {
  repository_slug = "acc-repo"
  key             = "DB_URL"
  value           = "postgres://example"
  secured         = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_pipeline_variable.test", "key", "DB_URL"),
					resource.TestCheckResourceAttr("bitbucket_pipeline_variable.test", "secured", "true"),
					// Even though Bitbucket never returns the value for a
					// secured variable, the resource must keep trusting the
					// config/state value instead of blanking it out.
					resource.TestCheckResourceAttr("bitbucket_pipeline_variable.test", "value", "postgres://example"),
				),
			},
			{
				ResourceName:            "bitbucket_pipeline_variable.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
				ImportStateIdFunc:       importStateIDFromAttrs("bitbucket_pipeline_variable.test", []string{"repository_slug", "variable_uuid"}, "/"),
			},
		},
	})
}
