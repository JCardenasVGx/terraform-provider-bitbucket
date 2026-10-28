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

func newMockBranchRestrictionServer(t *testing.T) *httptest.Server {
	t.Helper()

	type restriction struct {
		ID              int64  `json:"id,omitempty"`
		Kind            string `json:"kind"`
		BranchMatchKind string `json:"branch_match_kind,omitempty"`
		Pattern         string `json:"pattern,omitempty"`
		Value           *int64 `json:"value,omitempty"`
	}

	var mu sync.Mutex
	store := map[int64]restriction{}
	nextID := int64(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/branch-restrictions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body restriction
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		body.ID = nextID
		nextID++
		store[body.ID] = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(body)
	})
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/branch-restrictions/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/repositories/acc-test-workspace/acc-repo/branch-restrictions/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			rr, ok := store[id]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rr)
		case http.MethodPut:
			var body restriction
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			body.ID = id
			mu.Lock()
			store[id] = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case http.MethodDelete:
			mu.Lock()
			delete(store, id)
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

func TestAccBranchRestrictionResource(t *testing.T) {
	srv := newMockBranchRestrictionServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_branch_restriction" "test" {
  repository_slug   = "acc-repo"
  kind              = "require_approvals_to_merge"
  branch_match_kind = "glob"
  pattern           = "main"
  value             = 2
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_branch_restriction.test", "kind", "require_approvals_to_merge"),
					resource.TestCheckResourceAttr("bitbucket_branch_restriction.test", "pattern", "main"),
					resource.TestCheckResourceAttr("bitbucket_branch_restriction.test", "value", "2"),
				),
			},
			{
				ResourceName:      "bitbucket_branch_restriction.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDFromAttrs("bitbucket_branch_restriction.test", []string{"repository_slug", "restriction_id"}, "/"),
			},
			{
				Config: `
resource "bitbucket_branch_restriction" "test" {
  repository_slug   = "acc-repo"
  kind              = "require_approvals_to_merge"
  branch_match_kind = "glob"
  pattern           = "main"
  value             = 3
}
`,
				Check: resource.TestCheckResourceAttr("bitbucket_branch_restriction.test", "value", "3"),
			},
		},
	})
}
