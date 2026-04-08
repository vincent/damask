package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/tests_helpers"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// createFolder creates a folder in the given project and returns its parsed response.
func createFolder(t *testing.T, env *th.TestEnv, cookie *http.Cookie, projectID, name string, parentID *string) api.FolderResponse {
	t.Helper()
	req := th.AuthRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/folders",
		th.JsonBody(api.CreateFolderRequest{Name: name, ParentID: parentID}), cookie)
	res, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out api.FolderResponse
	_ = json.Unmarshal(b, &out)
	return out
}

func TestCreateFolder_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "My Project"}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer projRes.Body.Close()
	if projRes.StatusCode != http.StatusCreated {
		t.Fatalf("create project: got %d", projRes.StatusCode)
	}
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	folderReq := th.AuthRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/folders",
		th.JsonBody(api.CreateFolderRequest{Name: "Assets"}), owner.Cookie)
	folderRes, err := env.App.Test(folderReq)
	if err != nil {
		t.Fatal(err)
	}
	defer folderRes.Body.Close()
	if folderRes.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(folderRes.Body)
		t.Fatalf("create folder: got %d, body: %s", folderRes.StatusCode, string(b))
	}
	var folder map[string]interface{}
	b, _ = io.ReadAll(folderRes.Body)
	_ = json.Unmarshal(b, &folder)
	if folder["name"] != "Assets" {
		t.Errorf("got name %v, want Assets", folder["name"])
	}
	if folder["project_id"] != projectID {
		t.Errorf("got project_id %v, want %s", folder["project_id"], projectID)
	}
}

func TestCreateFolder_SubfolderSuccess(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "P"}), owner.Cookie))
	defer projRes.Body.Close()
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	// Create root folder
	rootOut := createFolder(t, env, owner.Cookie, projectID, "Root", nil)
	rootID := rootOut.ID

	// Create subfolder — should succeed
	subReq := th.AuthRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/folders",
		th.JsonBody(api.CreateFolderRequest{Name: "Sub", ParentID: &rootID}), owner.Cookie)
	subRes, _ := env.App.Test(subReq)
	defer subRes.Body.Close()
	if subRes.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(subRes.Body)
		t.Fatalf("create subfolder: got %d, body: %s", subRes.StatusCode, string(b))
	}
}

func TestCreateFolder_MaxDepthEnforced(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "P"}), owner.Cookie))
	defer projRes.Body.Close()
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	// Create root folder (depth 0)
	rootOut := createFolder(t, env, owner.Cookie, projectID, "Root", nil)
	rootID := rootOut.ID

	// Create subfolder (depth 1) — should succeed
	subOut := createFolder(t, env, owner.Cookie, projectID, "Sub", &rootID)
	subID := subOut.ID

	// Try to create depth 2 subfolder — should return 422
	req := th.AuthRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/folders",
		th.JsonBody(api.CreateFolderRequest{Name: "Deep", ParentID: &subID}), owner.Cookie)
	deepRes, _ := env.App.Test(req)
	defer deepRes.Body.Close()
	if deepRes.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", deepRes.StatusCode)
	}
}

func TestCreateFolder_DuplicateName(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "P"}), owner.Cookie))
	defer projRes.Body.Close()
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	createFolder(t, env, owner.Cookie, projectID, "Dupe", nil)

	req := th.AuthRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/folders",
		th.JsonBody(api.CreateFolderRequest{Name: "Dupe"}), owner.Cookie)
	dupeRes, _ := env.App.Test(req)
	defer dupeRes.Body.Close()
	if dupeRes.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", dupeRes.StatusCode)
	}
}

func TestGetFolders_Tree(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "P"}), owner.Cookie))
	defer projRes.Body.Close()
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	rootOut := createFolder(t, env, owner.Cookie, projectID, "Root", nil)
	rootID := rootOut.ID
	createFolder(t, env, owner.Cookie, projectID, "Child", &rootID)

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/folders", nil, owner.Cookie)
	treeRes, _ := env.App.Test(req)
	defer treeRes.Body.Close()
	if treeRes.StatusCode != http.StatusOK {
		t.Fatalf("get folders: got %d", treeRes.StatusCode)
	}
	var tree []map[string]interface{}
	b, _ = io.ReadAll(treeRes.Body)
	_ = json.Unmarshal(b, &tree)
	if len(tree) != 1 {
		t.Fatalf("expected 1 root folder, got %d", len(tree))
	}
	children, ok := tree[0]["children"].([]interface{})
	if !ok {
		t.Fatalf("children is not an array")
	}
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
}

func TestUpdateFolder_Rename(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "P"}), owner.Cookie))
	defer projRes.Body.Close()
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	folderOut := createFolder(t, env, owner.Cookie, projectID, "OldName", nil)
	folderID := folderOut.ID

	newName := "NewName"
	req := th.AuthRequest(http.MethodPut, "/api/v1/folders/"+folderID,
		th.JsonBody(api.UpdateFolderRequest{Name: &newName}), owner.Cookie)
	updateRes, _ := env.App.Test(req)
	defer updateRes.Body.Close()
	if updateRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(updateRes.Body)
		t.Fatalf("update folder: got %d, body: %s", updateRes.StatusCode, string(b))
	}
	var updated map[string]interface{}
	b, _ = io.ReadAll(updateRes.Body)
	_ = json.Unmarshal(b, &updated)
	if updated["name"] != "NewName" {
		t.Errorf("got name %v, want NewName", updated["name"])
	}
}

func TestDeleteFolder_NullifiesAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	projRes, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "P"}), owner.Cookie))
	defer projRes.Body.Close()
	var proj map[string]interface{}
	b, _ := io.ReadAll(projRes.Body)
	_ = json.Unmarshal(b, &proj)
	projectID := proj["id"].(string)

	folderOut := createFolder(t, env, owner.Cookie, projectID, "ToDelete", nil)
	folderID := folderOut.ID

	// Upload a test asset
	assetID := uploadTestAsset(t, env, owner)

	// Move asset to folder
	patchReq := th.AuthRequest(http.MethodPatch, "/api/v1/assets/"+assetID,
		th.JsonBody(api.UpdateAssetFolderRequest{FolderID: &folderID}), owner.Cookie)
	patchRes, err2 := env.App.Test(patchReq)
	if err2 != nil {
		t.Fatalf("patch request error: %v", err2)
	}
	defer patchRes.Body.Close()
	if patchRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(patchRes.Body)
		t.Fatalf("move asset to folder: got %d, body: %s", patchRes.StatusCode, string(b))
	}

	// Delete folder
	delReq := th.AuthRequest(http.MethodDelete, "/api/v1/folders/"+folderID, nil, owner.Cookie)
	delRes, err3 := env.App.Test(delReq, fiber.TestConfig{Timeout: 5000})
	if err3 != nil {
		t.Fatalf("delete request error: %v", err3)
	}
	defer delRes.Body.Close()
	if delRes.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(delRes.Body)
		t.Fatalf("delete folder: got %d, body: %s", delRes.StatusCode, string(b))
	}

	// Verify asset's folder_id is now NULL
	var folderIDVal interface{}
	err := env.SqlDB.QueryRow("SELECT folder_id FROM assets WHERE id = ?", assetID).Scan(&folderIDVal)
	if err != nil {
		t.Fatalf("query asset: %v", err)
	}
	if folderIDVal != nil {
		t.Errorf("expected folder_id to be NULL, got %v", folderIDVal)
	}
}
