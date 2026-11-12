package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/partite-ai/partite-cli/internal/cmd/root"
	"github.com/partite-ai/partite-cli/internal/cmd/workspace"
	"github.com/partite-ai/partite-cli/internal/cmdutil"
)

const workspaceID = "cwsp_11AL552WdT69LrwqHzj3GPTb"

// upload is what the fake mesh received for one upload.
type upload struct {
	variables     map[string]any
	body          string
	contentLength int64
	authorization string
}

// fakeMesh issues upload links for /reports/q3.pdf and takes PUTs to them,
// answering them with status.
func fakeMesh(t *testing.T, status int) (*httptest.Server, *upload) {
	got := &upload{}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.OperationName != "ToolsCreateWorkspaceFileUpload" {
			t.Errorf("operation %q, want ToolsCreateWorkspaceFileUpload", req.OperationName)
		}
		got.variables = req.Variables
		fmt.Fprintf(w, `{"data":{"createWorkspaceFileUpload":{"url":%q,"expiresAt":"2026-11-12T00:00:00Z"}}}`, srv.URL+"/file-uploads/tok")
	})
	mux.HandleFunc("PUT /file-uploads/tok", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		got.body, got.contentLength, got.authorization = string(data), r.ContentLength, r.Header.Get("Authorization")
		if status != http.StatusCreated {
			http.Error(w, "this organization has no storage left for the file", status)
			return
		}
		w.WriteHeader(status)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, got
}

func partite(t *testing.T, srv *httptest.Server, args ...string) (string, int) {
	t.Setenv("PARTITE_CONFIG_DIR", t.TempDir())
	t.Setenv("PARTITE_NO_KEYRING", "1")
	t.Setenv("PARTITE_CONTEXT", "local")
	t.Setenv("PARTITE_URL", srv.URL)
	t.Setenv("PARTITE_API_TOKEN", "")
	t.Setenv("PARTITE_API_KEY", "")
	t.Setenv("PARTITE_TOKEN", "")
	var out, errOut bytes.Buffer
	app := cmdutil.NewApp()
	app.In, app.Out, app.Err = strings.NewReader(""), &out, &errOut
	cmd := root.New(app, workspace.New)
	cmd.SetArgs(args)
	code := 0
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		e := app.Classify(err)
		app.PrintError(&errOut, e)
		code = e.Code
	}
	return errOut.String(), code
}

func localFile(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The CLI asks for a link to the remote path, then PUTs the file to it with
// its length and without the CLI's credentials.
func TestUploadPutsToTheLink(t *testing.T) {
	srv, got := fakeMesh(t, http.StatusCreated)
	errOut, code := partite(t, srv, "workspace", "upload", workspaceID, localFile(t, "the report"), "/reports/q3.pdf")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if got.variables["id"] != workspaceID || got.variables["folder"] != "/reports" || got.variables["name"] != "q3.pdf" {
		t.Errorf("asked for a link with %v", got.variables)
	}
	if got.body != "the report" || got.contentLength != int64(len("the report")) {
		t.Errorf("PUT %q with length %d", got.body, got.contentLength)
	}
	if got.authorization != "" {
		t.Errorf("the PUT carried credentials: %q", got.authorization)
	}
}

// A refused upload fails the command with the server's reason.
func TestUploadRefused(t *testing.T) {
	srv, _ := fakeMesh(t, http.StatusInsufficientStorage)
	errOut, code := partite(t, srv, "workspace", "upload", workspaceID, localFile(t, "the report"), "/reports/q3.pdf")
	if code == 0 {
		t.Fatal("the command succeeded")
	}
	if !strings.Contains(errOut, "no storage left") {
		t.Errorf("error output %q doesn't give the reason", errOut)
	}
}
