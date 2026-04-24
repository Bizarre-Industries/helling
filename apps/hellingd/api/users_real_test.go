package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// bearerPut is the PUT counterpart to bearerPost defined in
// auth_real_mfa_test.go. Kept local to this file to avoid pulling more
// helpers into the shared test-file surface.
func bearerPut(t *testing.T, srv *httptest.Server, path, access string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, srv.URL+path, bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestRealUsers_CreateListGetDeleteScope(t *testing.T) {
	srv := spinUp(t)
	access := setupAdmin(t, srv, "admin", "usertests12345")

	createResp := bearerPost(t, srv, "/api/v1/users", access, map[string]string{
		"username": "bob",
		"role":     "user",
		"password": "anotherpw1",
	})
	defer func() { _ = createResp.Body.Close() }()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create: %d body=%s", createResp.StatusCode, mustReadBody(createResp))
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	readJSON(t, createResp, &created)
	if created.Data.ID == "" {
		t.Fatal("created id missing")
	}

	dup := bearerPost(t, srv, "/api/v1/users", access, map[string]string{
		"username": "bob",
		"role":     "user",
		"password": "x1234567",
	})
	defer func() { _ = dup.Body.Close() }()
	if dup.StatusCode != http.StatusConflict {
		t.Fatalf("dup: %d", dup.StatusCode)
	}

	listResp := bearerGet(t, srv, "/api/v1/users?limit=10", access)
	defer func() { _ = listResp.Body.Close() }()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list: %d", listResp.StatusCode)
	}
	body := mustReadBody(listResp)
	if !strings.Contains(body, `"admin"`) || !strings.Contains(body, `"bob"`) {
		t.Fatalf("list missing users: %s", body)
	}

	getResp := bearerGet(t, srv, "/api/v1/users/"+created.Data.ID, access)
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get: %d", getResp.StatusCode)
	}

	scopeResp := bearerPut(t, srv, "/api/v1/users/"+created.Data.ID+"/scope", access, map[string]string{"scope": "restricted"})
	defer func() { _ = scopeResp.Body.Close() }()
	if scopeResp.StatusCode != http.StatusOK {
		t.Fatalf("set scope: %d body=%s", scopeResp.StatusCode, mustReadBody(scopeResp))
	}

	bad := bearerPut(t, srv, "/api/v1/users/"+created.Data.ID+"/scope", access, map[string]string{"scope": "owner"})
	defer func() { _ = bad.Body.Close() }()
	if bad.StatusCode != http.StatusBadRequest && bad.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad scope: %d", bad.StatusCode)
	}

	delResp := bearerDelete(t, srv, "/api/v1/users/"+created.Data.ID, access)
	defer func() { _ = delResp.Body.Close() }()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete: %d", delResp.StatusCode)
	}

	gone := bearerGet(t, srv, "/api/v1/users/"+created.Data.ID, access)
	defer func() { _ = gone.Body.Close() }()
	if gone.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: %d", gone.StatusCode)
	}
}

func TestRealUsers_Pagination(t *testing.T) {
	srv := spinUp(t)
	access := setupAdmin(t, srv, "admin", "paginationtest1")

	// Create five additional users so paging kicks in with limit=2.
	for _, name := range []string{"u1", "u2", "u3", "u4", "u5"} {
		resp := bearerPost(t, srv, "/api/v1/users", access, map[string]string{
			"username": name,
			"role":     "user",
			"password": "userpassword123",
		})
		_ = resp.Body.Close()
	}

	first := bearerGet(t, srv, "/api/v1/users?limit=2", access)
	defer func() { _ = first.Body.Close() }()
	var p1 struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Page struct {
				HasNext    bool   `json:"has_next"`
				NextCursor string `json:"next_cursor"`
			} `json:"page"`
		} `json:"meta"`
	}
	readJSON(t, first, &p1)
	if !p1.Meta.Page.HasNext || p1.Meta.Page.NextCursor == "" {
		t.Fatalf("expected paging, got %+v", p1.Meta)
	}

	second := bearerGet(t, srv, "/api/v1/users?limit=2&cursor="+p1.Meta.Page.NextCursor, access)
	defer func() { _ = second.Body.Close() }()
	var p2 struct {
		Data []map[string]any `json:"data"`
	}
	readJSON(t, second, &p2)
	if len(p2.Data) != 2 {
		t.Fatalf("expected 2 on page 2, got %d", len(p2.Data))
	}
}

func TestRealUsers_MissingFieldsReject(t *testing.T) {
	srv := spinUp(t)
	access := setupAdmin(t, srv, "admin", "usertests12345")
	resp := bearerPost(t, srv, "/api/v1/users", access, map[string]string{
		"role":     "user",
		"password": "x1234567",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnprocessableEntity && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400/422 for missing username, got %d", resp.StatusCode)
	}
}

func TestRealUsers_RequiresAuth(t *testing.T) {
	srv := spinUp(t)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/v1/users", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
