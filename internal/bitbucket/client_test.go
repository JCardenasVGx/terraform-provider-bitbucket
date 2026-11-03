package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient spins up a mock Bitbucket API server and returns a Client
// pointed at it, plus a handler mux the test can configure.
func newTestClient(t *testing.T, mux *http.ServeMux) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := &Client{
		baseURL:    srv.URL,
		baseURLV1:  srv.URL,
		workspace:  "test-workspace",
		username:   "test-user",
		password:   "test-pass",
		httpClient: http.DefaultClient,
	}
	return c, srv
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			t.Fatalf("encoding response: %v", err)
		}
	}
}

func checkBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	user, pass, ok := r.BasicAuth()
	if !ok || user != "test-user" || pass != "test-pass" {
		t.Fatalf("expected basic auth test-user:test-pass, got %q:%q (ok=%v)", user, pass, ok)
	}
}

func TestProjectCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/test-workspace/projects", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body Project
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if body.Key != "PLAT" {
			t.Fatalf("expected key PLAT, got %q", body.Key)
		}
		writeJSON(t, w, http.StatusCreated, Project{Key: "PLAT", Name: body.Name, IsPrivate: body.IsPrivate})
	})
	mux.HandleFunc("/workspaces/test-workspace/projects/PLAT", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, Project{Key: "PLAT", Name: "Platform", IsPrivate: true})
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, Project{Key: "PLAT", Name: "Platform Renamed", IsPrivate: true})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	mux.HandleFunc("/workspaces/test-workspace/projects/MISSING", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateProject(ctx, &Project{Key: "PLAT", Name: "Platform", IsPrivate: true})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if created.Key != "PLAT" {
		t.Errorf("expected key PLAT, got %q", created.Key)
	}

	got, err := client.GetProject(ctx, "PLAT")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Platform" {
		t.Errorf("expected name Platform, got %q", got.Name)
	}

	updated, err := client.UpdateProject(ctx, "PLAT", &Project{Key: "PLAT", Name: "Platform Renamed"})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.Name != "Platform Renamed" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}

	if err := client.DeleteProject(ctx, "PLAT"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	_, err = client.GetProject(ctx, "MISSING")
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound(err) to be true, got %v", err)
	}
}

func TestRepositoryCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/test-workspace/payments-api", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		isPrivate := true
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body Repository
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			writeJSON(t, w, http.StatusOK, Repository{
				Slug: "payments-api", Name: body.Name, IsPrivate: &isPrivate,
				Project: body.Project,
			})
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, Repository{
				Slug: "payments-api", Name: "payments-api", IsPrivate: &isPrivate,
				Project: &ProjectLink{Key: "PLAT"},
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateRepository(ctx, "payments-api", &Repository{
		Name:    "payments-api",
		Project: &ProjectLink{Key: "PLAT"},
	})
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}
	if created.Project == nil || created.Project.Key != "PLAT" {
		t.Errorf("expected project PLAT, got %+v", created.Project)
	}

	got, err := client.GetRepository(ctx, "payments-api")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if got.Slug != "payments-api" {
		t.Errorf("expected slug payments-api, got %q", got.Slug)
	}

	if _, err := client.UpdateRepository(ctx, "payments-api", &Repository{Name: "payments-api"}); err != nil {
		t.Fatalf("UpdateRepository: %v", err)
	}

	if err := client.DeleteRepository(ctx, "payments-api"); err != nil {
		t.Fatalf("DeleteRepository: %v", err)
	}
}

func TestBranchRestrictionCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/test-workspace/payments-api/branch-restrictions", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body BranchRestriction
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		body.ID = 42
		writeJSON(t, w, http.StatusCreated, body)
	})
	mux.HandleFunc("/repositories/test-workspace/payments-api/branch-restrictions/42", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, BranchRestriction{ID: 42, Kind: "force", Pattern: "main"})
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, BranchRestriction{ID: 42, Kind: "force", Pattern: "release/*"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateBranchRestriction(ctx, "payments-api", &BranchRestriction{Kind: "force", Pattern: "main"})
	if err != nil {
		t.Fatalf("CreateBranchRestriction: %v", err)
	}
	if created.ID != 42 {
		t.Errorf("expected id 42, got %d", created.ID)
	}

	got, err := client.GetBranchRestriction(ctx, "payments-api", 42)
	if err != nil {
		t.Fatalf("GetBranchRestriction: %v", err)
	}
	if got.Kind != "force" {
		t.Errorf("expected kind force, got %q", got.Kind)
	}

	updated, err := client.UpdateBranchRestriction(ctx, "payments-api", 42, &BranchRestriction{Kind: "force", Pattern: "release/*"})
	if err != nil {
		t.Fatalf("UpdateBranchRestriction: %v", err)
	}
	if updated.Pattern != "release/*" {
		t.Errorf("expected pattern release/*, got %q", updated.Pattern)
	}

	if err := client.DeleteBranchRestriction(ctx, "payments-api", 42); err != nil {
		t.Fatalf("DeleteBranchRestriction: %v", err)
	}
}

func TestPipelineConfig(t *testing.T) {
	mux := http.NewServeMux()
	state := &PipelinesConfig{Enabled: false}
	mux.HandleFunc("/repositories/test-workspace/payments-api/pipelines_config", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, state)
		case http.MethodPut:
			var body PipelinesConfig
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			state.Enabled = body.Enabled
			writeJSON(t, w, http.StatusOK, state)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	got, err := client.GetPipelineConfig(ctx, "payments-api")
	if err != nil {
		t.Fatalf("GetPipelineConfig: %v", err)
	}
	if got.Enabled {
		t.Errorf("expected initially disabled")
	}

	updated, err := client.UpdatePipelineConfig(ctx, "payments-api", true)
	if err != nil {
		t.Fatalf("UpdatePipelineConfig: %v", err)
	}
	if !updated.Enabled {
		t.Errorf("expected enabled after update")
	}
}

func TestRepoPipelineVariableCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/test-workspace/payments-api/pipelines_config/variables", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body PipelineVariable
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		body.UUID = "var-uuid"
		writeJSON(t, w, http.StatusCreated, body)
	})
	mux.HandleFunc("/repositories/test-workspace/payments-api/pipelines_config/variables/var-uuid", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, PipelineVariable{UUID: "var-uuid", Key: "DB_URL", Value: "postgres://x", Secured: true})
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, PipelineVariable{UUID: "var-uuid", Key: "DB_URL", Secured: true})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateRepoPipelineVariable(ctx, "payments-api", &PipelineVariable{Key: "DB_URL", Value: "postgres://x", Secured: true})
	if err != nil {
		t.Fatalf("CreateRepoPipelineVariable: %v", err)
	}
	if created.UUID != "var-uuid" {
		t.Errorf("expected uuid, got %q", created.UUID)
	}

	got, err := client.GetRepoPipelineVariable(ctx, "payments-api", "var-uuid")
	if err != nil {
		t.Fatalf("GetRepoPipelineVariable: %v", err)
	}
	if !got.Secured {
		t.Errorf("expected secured=true")
	}

	if _, err := client.UpdateRepoPipelineVariable(ctx, "payments-api", "var-uuid", &PipelineVariable{Key: "DB_URL", Secured: true}); err != nil {
		t.Fatalf("UpdateRepoPipelineVariable: %v", err)
	}

	if err := client.DeleteRepoPipelineVariable(ctx, "payments-api", "var-uuid"); err != nil {
		t.Fatalf("DeleteRepoPipelineVariable: %v", err)
	}
}

func TestWorkspacePipelineVariableCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/test-workspace/pipelines-config/variables", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body PipelineVariable
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		body.UUID = "var-uuid"
		writeJSON(t, w, http.StatusCreated, body)
	})
	mux.HandleFunc("/workspaces/test-workspace/pipelines-config/variables/var-uuid", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, PipelineVariable{UUID: "var-uuid", Key: "DOCKER_REGISTRY"})
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, PipelineVariable{UUID: "var-uuid", Key: "DOCKER_REGISTRY"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateWorkspacePipelineVariable(ctx, &PipelineVariable{Key: "DOCKER_REGISTRY"})
	if err != nil {
		t.Fatalf("CreateWorkspacePipelineVariable: %v", err)
	}
	if created.UUID != "var-uuid" {
		t.Errorf("expected uuid, got %q", created.UUID)
	}

	if _, err := client.GetWorkspacePipelineVariable(ctx, "var-uuid"); err != nil {
		t.Fatalf("GetWorkspacePipelineVariable: %v", err)
	}
	if _, err := client.UpdateWorkspacePipelineVariable(ctx, "var-uuid", &PipelineVariable{Key: "DOCKER_REGISTRY"}); err != nil {
		t.Fatalf("UpdateWorkspacePipelineVariable: %v", err)
	}
	if err := client.DeleteWorkspacePipelineVariable(ctx, "var-uuid"); err != nil {
		t.Fatalf("DeleteWorkspacePipelineVariable: %v", err)
	}
}

func TestWorkspaceRunnerCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/test-workspace/pipelines-config/runners", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body createRunnerRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		if body.Name != "shared-runner" {
			t.Fatalf("expected name shared-runner, got %q", body.Name)
		}
		writeJSON(t, w, http.StatusCreated, Runner{
			UUID: "runner-uuid", Name: body.Name, Labels: body.Labels,
			OauthClient: &RunnerOauthClient{ID: "client-id", Secret: "client-secret"},
		})
	})
	mux.HandleFunc("/workspaces/test-workspace/pipelines-config/runners/runner-uuid", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			// Secret is only returned once, at creation: subsequent GETs omit it.
			writeJSON(t, w, http.StatusOK, Runner{
				UUID: "runner-uuid", Name: "shared-runner",
				OauthClient: &RunnerOauthClient{ID: "client-id"},
			})
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, Runner{UUID: "runner-uuid", Name: "renamed-runner"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateWorkspaceRunner(ctx, "shared-runner", []string{"self.hosted"})
	if err != nil {
		t.Fatalf("CreateWorkspaceRunner: %v", err)
	}
	if created.OauthClient == nil || created.OauthClient.Secret != "client-secret" {
		t.Fatalf("expected oauth secret on creation, got %+v", created.OauthClient)
	}

	got, err := client.GetWorkspaceRunner(ctx, "runner-uuid")
	if err != nil {
		t.Fatalf("GetWorkspaceRunner: %v", err)
	}
	if got.OauthClient.Secret != "" {
		t.Errorf("expected secret to be empty on subsequent GET, got %q", got.OauthClient.Secret)
	}

	if _, err := client.UpdateWorkspaceRunner(ctx, "runner-uuid", "renamed-runner", nil); err != nil {
		t.Fatalf("UpdateWorkspaceRunner: %v", err)
	}
	if err := client.DeleteWorkspaceRunner(ctx, "runner-uuid"); err != nil {
		t.Fatalf("DeleteWorkspaceRunner: %v", err)
	}
}

func TestRepositoryRunnerCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/test-workspace/payments-api/pipelines-config/runners", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		writeJSON(t, w, http.StatusCreated, Runner{UUID: "runner-uuid", Name: "repo-runner"})
	})
	mux.HandleFunc("/repositories/test-workspace/payments-api/pipelines-config/runners/runner-uuid", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, Runner{UUID: "runner-uuid", Name: "repo-runner"})
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, Runner{UUID: "runner-uuid", Name: "repo-runner-renamed"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	if _, err := client.CreateRepositoryRunner(ctx, "payments-api", "repo-runner", nil); err != nil {
		t.Fatalf("CreateRepositoryRunner: %v", err)
	}
	if _, err := client.GetRepositoryRunner(ctx, "payments-api", "runner-uuid"); err != nil {
		t.Fatalf("GetRepositoryRunner: %v", err)
	}
	if _, err := client.UpdateRepositoryRunner(ctx, "payments-api", "runner-uuid", "repo-runner-renamed", nil); err != nil {
		t.Fatalf("UpdateRepositoryRunner: %v", err)
	}
	if err := client.DeleteRepositoryRunner(ctx, "payments-api", "runner-uuid"); err != nil {
		t.Fatalf("DeleteRepositoryRunner: %v", err)
	}
}

func TestEnvironmentCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/test-workspace/payments-api/environments/", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body DeploymentEnvironment
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		body.UUID = "env-uuid"
		writeJSON(t, w, http.StatusCreated, body)
	})
	mux.HandleFunc("/repositories/test-workspace/payments-api/environments/env-uuid", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, DeploymentEnvironment{
				UUID: "env-uuid", Name: "production",
				EnvironmentType: &EnvironmentType{Name: "Production"},
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	mux.HandleFunc("/repositories/test-workspace/payments-api/environments/env-uuid/changes/", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body environmentChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		writeJSON(t, w, http.StatusOK, DeploymentEnvironment{
			UUID: "env-uuid", Name: body.Change.Name,
		})
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateEnvironment(ctx, "payments-api", &DeploymentEnvironment{
		Name:            "production",
		EnvironmentType: &EnvironmentType{Name: "Production"},
	})
	if err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}
	if created.UUID != "env-uuid" {
		t.Errorf("expected uuid, got %q", created.UUID)
	}

	got, err := client.GetEnvironment(ctx, "payments-api", "env-uuid")
	if err != nil {
		t.Fatalf("GetEnvironment: %v", err)
	}
	if got.EnvironmentType.Name != "Production" {
		t.Errorf("expected Production, got %q", got.EnvironmentType.Name)
	}

	updated, err := client.UpdateEnvironment(ctx, "payments-api", "env-uuid", "prod-renamed", &EnvironmentRestrictions{AdminOnly: true})
	if err != nil {
		t.Fatalf("UpdateEnvironment: %v", err)
	}
	if updated.Name != "prod-renamed" {
		t.Errorf("expected renamed, got %q", updated.Name)
	}

	if err := client.DeleteEnvironment(ctx, "payments-api", "env-uuid"); err != nil {
		t.Fatalf("DeleteEnvironment: %v", err)
	}
}

func TestDeploymentVariableCRUDAndPagination(t *testing.T) {
	mux := http.NewServeMux()
	basePath := "/repositories/test-workspace/payments-api/deployments_config/environments/env-uuid/variables"

	mux.HandleFunc(basePath, func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body DeploymentVariable
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		body.UUID = "var-uuid-2"
		writeJSON(t, w, http.StatusCreated, body)
	})
	mux.HandleFunc(basePath+"/var-uuid-2", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodPut:
			writeJSON(t, w, http.StatusOK, DeploymentVariable{UUID: "var-uuid-2", Key: "DB_URL"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateDeploymentVariable(ctx, "payments-api", "env-uuid", &DeploymentVariable{Key: "DB_URL", Value: "x", Secured: true})
	if err != nil {
		t.Fatalf("CreateDeploymentVariable: %v", err)
	}
	if created.UUID != "var-uuid-2" {
		t.Errorf("expected uuid, got %q", created.UUID)
	}

	if _, err := client.UpdateDeploymentVariable(ctx, "payments-api", "env-uuid", "var-uuid-2", &DeploymentVariable{Key: "DB_URL"}); err != nil {
		t.Fatalf("UpdateDeploymentVariable: %v", err)
	}

	if err := client.DeleteDeploymentVariable(ctx, "payments-api", "env-uuid", "var-uuid-2"); err != nil {
		t.Fatalf("DeleteDeploymentVariable: %v", err)
	}
}

// GetDeploymentVariable has no get-by-id endpoint in the Bitbucket API: it
// must page through the list endpoint looking for a matching UUID. This
// test simulates two pages to exercise that pagination loop.
func TestGetDeploymentVariablePagination(t *testing.T) {
	mux := http.NewServeMux()
	listPath := "/repositories/test-workspace/payments-api/deployments_config/environments/env-uuid/variables"

	var srvURL string
	mux.HandleFunc(listPath, func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.URL.Query().Get("cursor") == "page2" {
			writeJSON(t, w, http.StatusOK, map[string]any{
				"values": []DeploymentVariable{{UUID: "target-uuid", Key: "FOUND", Value: "yes"}},
			})
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"values": []DeploymentVariable{{UUID: "other-uuid", Key: "OTHER"}},
			"next":   srvURL + listPath + "?cursor=page2",
		})
	})

	client, srv := newTestClient(t, mux)
	srvURL = srv.URL
	ctx := context.Background()

	found, err := client.GetDeploymentVariable(ctx, "payments-api", "env-uuid", "target-uuid")
	if err != nil {
		t.Fatalf("GetDeploymentVariable: %v", err)
	}
	if found.Key != "FOUND" {
		t.Errorf("expected to find variable on second page, got %+v", found)
	}

	_, err = client.GetDeploymentVariable(ctx, "payments-api", "env-uuid", "does-not-exist")
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound for an unmatched uuid, got %v", err)
	}
}

func TestRepositoryGroupPermissionCRUD(t *testing.T) {
	mux := http.NewServeMux()
	var granted *RepositoryGroupPermission
	mux.HandleFunc("/repositories/test-workspace/payments-api/permissions-config/groups/release-managers", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			if granted == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeJSON(t, w, http.StatusOK, granted)
		case http.MethodPut:
			var body RepositoryGroupPermission
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			body.Group = GroupReference{Slug: "release-managers"}
			granted = &body
			writeJSON(t, w, http.StatusOK, granted)
		case http.MethodDelete:
			granted = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	if _, err := client.GetRepositoryGroupPermission(ctx, "payments-api", "release-managers"); !IsNotFound(err) {
		t.Fatalf("expected not found before grant, got %v", err)
	}

	got, err := client.SetRepositoryGroupPermission(ctx, "payments-api", "release-managers", "write")
	if err != nil {
		t.Fatalf("SetRepositoryGroupPermission: %v", err)
	}
	if got.Permission != "write" {
		t.Errorf("expected permission write, got %q", got.Permission)
	}

	if err := client.DeleteRepositoryGroupPermission(ctx, "payments-api", "release-managers"); err != nil {
		t.Fatalf("DeleteRepositoryGroupPermission: %v", err)
	}
	if _, err := client.GetRepositoryGroupPermission(ctx, "payments-api", "release-managers"); !IsNotFound(err) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestRepositoryUserPermissionCRUD(t *testing.T) {
	const selectedUser = "jcardenas"

	mux := http.NewServeMux()
	var granted *RepositoryUserPermission
	mux.HandleFunc("/repositories/test-workspace/payments-api/permissions-config/users/"+selectedUser, func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			if granted == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeJSON(t, w, http.StatusOK, granted)
		case http.MethodPut:
			var body RepositoryUserPermission
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			body.User = AccountReference{UUID: "{11111111-2222-3333-4444-555555555555}"}
			granted = &body
			writeJSON(t, w, http.StatusOK, granted)
		case http.MethodDelete:
			granted = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	got, err := client.SetRepositoryUserPermission(ctx, "payments-api", selectedUser, "admin")
	if err != nil {
		t.Fatalf("SetRepositoryUserPermission: %v", err)
	}
	if got.Permission != "admin" {
		t.Errorf("expected permission admin, got %q", got.Permission)
	}

	fetched, err := client.GetRepositoryUserPermission(ctx, "payments-api", selectedUser)
	if err != nil {
		t.Fatalf("GetRepositoryUserPermission: %v", err)
	}
	if fetched.Permission != "admin" {
		t.Errorf("expected permission admin, got %q", fetched.Permission)
	}

	if err := client.DeleteRepositoryUserPermission(ctx, "payments-api", selectedUser); err != nil {
		t.Fatalf("DeleteRepositoryUserPermission: %v", err)
	}
}

func TestDefaultReviewerCRUD(t *testing.T) {
	mux := http.NewServeMux()
	exists := false
	mux.HandleFunc("/repositories/test-workspace/payments-api/default-reviewers/jcardenas", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeJSON(t, w, http.StatusOK, DefaultReviewer{UUID: "{u}", DisplayName: "Jesus Cardenas"})
		case http.MethodPut:
			exists = true
			writeJSON(t, w, http.StatusOK, DefaultReviewer{UUID: "{u}", DisplayName: "Jesus Cardenas"})
		case http.MethodDelete:
			exists = false
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	if _, err := client.GetDefaultReviewer(ctx, "payments-api", "jcardenas"); !IsNotFound(err) {
		t.Fatalf("expected not found before add, got %v", err)
	}
	if _, err := client.AddDefaultReviewer(ctx, "payments-api", "jcardenas"); err != nil {
		t.Fatalf("AddDefaultReviewer: %v", err)
	}
	if _, err := client.GetDefaultReviewer(ctx, "payments-api", "jcardenas"); err != nil {
		t.Fatalf("GetDefaultReviewer: %v", err)
	}
	if err := client.RemoveDefaultReviewer(ctx, "payments-api", "jcardenas"); err != nil {
		t.Fatalf("RemoveDefaultReviewer: %v", err)
	}
	if _, err := client.GetDefaultReviewer(ctx, "payments-api", "jcardenas"); !IsNotFound(err) {
		t.Fatalf("expected not found after remove, got %v", err)
	}
}

func TestProjectGroupPermissionCRUD(t *testing.T) {
	mux := http.NewServeMux()
	var permission string
	mux.HandleFunc("/workspaces/test-workspace/projects/PROJ/permissions-config/groups/release-managers", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodPut:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			permission = body["permission"]
			writeJSON(t, w, http.StatusOK, ProjectGroupPermission{Permission: permission})
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, ProjectGroupPermission{Permission: permission})
		case http.MethodDelete:
			permission = ""
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	got, err := client.SetProjectGroupPermission(ctx, "PROJ", "release-managers", "admin")
	if err != nil {
		t.Fatalf("SetProjectGroupPermission: %v", err)
	}
	if got.Permission != "admin" {
		t.Errorf("expected admin, got %q", got.Permission)
	}
	if err := client.DeleteProjectGroupPermission(ctx, "PROJ", "release-managers"); err != nil {
		t.Fatalf("DeleteProjectGroupPermission: %v", err)
	}
}

func TestProjectUserPermissionCRUD(t *testing.T) {
	mux := http.NewServeMux()
	var permission string
	mux.HandleFunc("/workspaces/test-workspace/projects/PROJ/permissions-config/users/jcardenas", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodPut:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			permission = body["permission"]
			writeJSON(t, w, http.StatusOK, ProjectUserPermission{Permission: permission})
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, ProjectUserPermission{Permission: permission})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	got, err := client.SetProjectUserPermission(ctx, "PROJ", "jcardenas", "write")
	if err != nil {
		t.Fatalf("SetProjectUserPermission: %v", err)
	}
	if got.Permission != "write" {
		t.Errorf("expected write, got %q", got.Permission)
	}
	fetched, err := client.GetProjectUserPermission(ctx, "PROJ", "jcardenas")
	if err != nil {
		t.Fatalf("GetProjectUserPermission: %v", err)
	}
	if fetched.Permission != "write" {
		t.Errorf("expected write, got %q", fetched.Permission)
	}
}

func TestHookCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/test-workspace/payments-api/hooks", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		var body Hook
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding body: %v", err)
		}
		body.UUID = "hook-1"
		writeJSON(t, w, http.StatusCreated, body)
	})
	mux.HandleFunc("/repositories/test-workspace/payments-api/hooks/hook-1", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, Hook{UUID: "hook-1", URL: "https://example.com/hook", Active: true, Events: []string{"repo:push"}})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateHook(ctx, "payments-api", &Hook{URL: "https://example.com/hook", Active: true, Events: []string{"repo:push"}})
	if err != nil {
		t.Fatalf("CreateHook: %v", err)
	}
	if created.UUID != "hook-1" {
		t.Errorf("expected uuid hook-1, got %q", created.UUID)
	}

	fetched, err := client.GetHook(ctx, "payments-api", "hook-1")
	if err != nil {
		t.Fatalf("GetHook: %v", err)
	}
	if fetched.URL != "https://example.com/hook" {
		t.Errorf("expected url, got %q", fetched.URL)
	}

	if err := client.DeleteHook(ctx, "payments-api", "hook-1"); err != nil {
		t.Fatalf("DeleteHook: %v", err)
	}
}

func TestBranchingModelSettings(t *testing.T) {
	mux := http.NewServeMux()
	state := &BranchingModelSettings{
		Development: &BranchingModelBranch{UseMainbranch: true},
		Production:  &BranchingModelProduction{Enabled: false},
	}
	mux.HandleFunc("/repositories/test-workspace/payments-api/branching-model/settings", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, state)
		case http.MethodPut:
			var body BranchingModelSettings
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			state = &body
			writeJSON(t, w, http.StatusOK, state)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	got, err := client.GetBranchingModelSettings(ctx, "payments-api")
	if err != nil {
		t.Fatalf("GetBranchingModelSettings: %v", err)
	}
	if !got.Development.UseMainbranch {
		t.Errorf("expected development.use_mainbranch true")
	}

	updated, err := client.UpdateBranchingModelSettings(ctx, "payments-api", &BranchingModelSettings{
		Development: &BranchingModelBranch{Name: "develop"},
		Production:  &BranchingModelProduction{Enabled: true, Name: "main", UseMainbranch: true},
	})
	if err != nil {
		t.Fatalf("UpdateBranchingModelSettings: %v", err)
	}
	if !updated.Production.Enabled {
		t.Errorf("expected production.enabled true after update")
	}
}

func TestGroupCRUDAndMembership(t *testing.T) {
	mux := http.NewServeMux()
	var groups []Group
	members := map[string][]GroupMember{}

	mux.HandleFunc("/groups/test-workspace/", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, groups)
		case http.MethodPost:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			g := Group{Name: body["name"], Slug: "release-managers"}
			groups = append(groups, g)
			writeJSON(t, w, http.StatusOK, g)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	mux.HandleFunc("/groups/test-workspace/release-managers/", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodPut:
			var body Group
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding body: %v", err)
			}
			body.Slug = "release-managers"
			groups[0] = body
			writeJSON(t, w, http.StatusOK, body)
		case http.MethodDelete:
			groups = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	mux.HandleFunc("/groups/test-workspace/release-managers/members/", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		writeJSON(t, w, http.StatusOK, members["release-managers"])
	})
	mux.HandleFunc("/groups/test-workspace/release-managers/members/jcardenas/", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		switch r.Method {
		case http.MethodPut:
			members["release-managers"] = []GroupMember{{UUID: "jcardenas", Username: "jcardenas"}}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			members["release-managers"] = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	created, err := client.CreateGroup(ctx, &Group{Name: "Release Managers"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if created.Slug != "release-managers" {
		t.Fatalf("expected slug release-managers, got %q", created.Slug)
	}

	got, err := client.GetGroup(ctx, "release-managers")
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.Name != "Release Managers" {
		t.Errorf("expected name Release Managers, got %q", got.Name)
	}

	updated, err := client.UpdateGroup(ctx, "release-managers", &Group{Name: "Release Managers", Permission: "write"})
	if err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	if updated.Permission != "write" {
		t.Errorf("expected permission write, got %q", updated.Permission)
	}

	if err := client.AddGroupMember(ctx, "release-managers", "jcardenas"); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}
	isMember, err := client.IsGroupMember(ctx, "release-managers", "jcardenas")
	if err != nil {
		t.Fatalf("IsGroupMember: %v", err)
	}
	if !isMember {
		t.Errorf("expected jcardenas to be a member")
	}

	if err := client.RemoveGroupMember(ctx, "release-managers", "jcardenas"); err != nil {
		t.Fatalf("RemoveGroupMember: %v", err)
	}
	isMember, err = client.IsGroupMember(ctx, "release-managers", "jcardenas")
	if err != nil {
		t.Fatalf("IsGroupMember: %v", err)
	}
	if isMember {
		t.Errorf("expected jcardenas to no longer be a member")
	}

	if err := client.DeleteGroup(ctx, "release-managers"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
}

func TestUserAndWorkspaceLookups(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/jcardenas", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		writeJSON(t, w, http.StatusOK, User{UUID: "{u}", Username: "jcardenas", DisplayName: "Jesus Cardenas"})
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		writeJSON(t, w, http.StatusOK, User{UUID: "{me}", Username: "acting-user"})
	})
	mux.HandleFunc("/workspaces/test-workspace", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		writeJSON(t, w, http.StatusOK, Workspace{Slug: "test-workspace", Name: "Test Workspace", IsPrivate: true})
	})
	mux.HandleFunc("/workspaces/test-workspace/members", func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		writeJSON(t, w, http.StatusOK, map[string]any{
			"values": []map[string]any{
				{"user": map[string]string{"uuid": "{a}", "username": "alice"}},
				{"user": map[string]string{"uuid": "{b}", "username": "bob"}},
			},
		})
	})

	client, _ := newTestClient(t, mux)
	ctx := context.Background()

	user, err := client.GetUser(ctx, "jcardenas")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.DisplayName != "Jesus Cardenas" {
		t.Errorf("expected display name, got %q", user.DisplayName)
	}

	me, err := client.GetCurrentUser(ctx)
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if me.Username != "acting-user" {
		t.Errorf("expected acting-user, got %q", me.Username)
	}

	ws, err := client.GetWorkspace(ctx)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if ws.Name != "Test Workspace" {
		t.Errorf("expected Test Workspace, got %q", ws.Name)
	}

	members, err := client.ListWorkspaceMembers(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaceMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[0].Username != "alice" || members[1].Username != "bob" {
		t.Errorf("unexpected members: %+v", members)
	}
}

func TestIsNotFound(t *testing.T) {
	notFoundErr := &APIError{StatusCode: http.StatusNotFound, Body: "nope"}
	if !IsNotFound(notFoundErr) {
		t.Errorf("expected IsNotFound to be true for a 404 APIError")
	}

	otherErr := &APIError{StatusCode: http.StatusInternalServerError, Body: "boom"}
	if IsNotFound(otherErr) {
		t.Errorf("expected IsNotFound to be false for a 500 APIError")
	}

	if IsNotFound(fmt.Errorf("plain error")) {
		t.Errorf("expected IsNotFound to be false for a non-APIError")
	}

	if IsNotFound(nil) {
		t.Errorf("expected IsNotFound to be false for nil")
	}
}
