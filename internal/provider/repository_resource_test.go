package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockRepositoryServer(t *testing.T) *httptest.Server {
	t.Helper()

	type repo struct {
		Slug        string `json:"slug,omitempty"`
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
		IsPrivate   *bool  `json:"is_private,omitempty"`
		ForkPolicy  string `json:"fork_policy,omitempty"`
		HasIssues   *bool  `json:"has_issues,omitempty"`
		HasWiki     *bool  `json:"has_wiki,omitempty"`
		Project     *struct {
			Key string `json:"key"`
		} `json:"project,omitempty"`
	}

	var mu sync.Mutex
	store := map[string]repo{}

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Path[len("/repositories/acc-test-workspace/"):]

		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body repo
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			body.Slug = slug
			mu.Lock()
			store[slug] = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case http.MethodGet:
			mu.Lock()
			r2, ok := store[slug]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(r2)
		case http.MethodDelete:
			mu.Lock()
			delete(store, slug)
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

func TestAccRepositoryResource(t *testing.T) {
	srv := newMockRepositoryServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_repository" "test" {
  slug        = "acc-repo"
  name        = "acc-repo"
  description = "created by acceptance test"
  project_key = "ACC"
  is_private  = true
  fork_policy = "no_forks"
  has_issues  = true
  has_wiki    = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_repository.test", "slug", "acc-repo"),
					resource.TestCheckResourceAttr("bitbucket_repository.test", "project_key", "ACC"),
					resource.TestCheckResourceAttr("bitbucket_repository.test", "fork_policy", "no_forks"),
					resource.TestCheckResourceAttr("bitbucket_repository.test", "id", "acc-test-workspace/acc-repo"),
				),
			},
			{
				ResourceName:      "bitbucket_repository.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "acc-repo",
			},
			{
				Config: `
resource "bitbucket_repository" "test" {
  slug        = "acc-repo"
  name        = "acc-repo"
  description = "updated by acceptance test"
  project_key = "ACC"
  is_private  = true
  fork_policy = "no_forks"
  has_issues  = true
  has_wiki    = false
}
`,
				Check: resource.TestCheckResourceAttr("bitbucket_repository.test", "description", "updated by acceptance test"),
			},
		},
	})
}
