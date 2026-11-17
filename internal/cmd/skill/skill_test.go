package skill_test

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
	"sync"
	"testing"

	"github.com/partite-ai/partite-cli/internal/cmd/root"
	"github.com/partite-ai/partite-cli/internal/cmd/skill"
	"github.com/partite-ai/partite-cli/internal/cmdutil"
)

const skillMD = "---\nname: pdf-forms\ndescription: Fills PDF forms.\n---\nFill the form.\n"

// fakeMesh makes uploads and keeps what is PUT to them, and creates a skill
// from them. It returns SKILL.md's content, and every other file by a
// download URL, as it does for a file too large to return.
type fakeMesh struct {
	srv *httptest.Server

	mu       sync.Mutex
	uploads  map[string]string
	putAuth  []string
	created  []map[string]any
	nextID   int
	downAuth []string
}

func newFakeMesh(t *testing.T) *fakeMesh {
	m := &fakeMesh{uploads: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		switch req.OperationName {
		case "CreateUpload":
			m.nextID++
			id := fmt.Sprintf("up%d", m.nextID)
			fmt.Fprintf(w, `{"data":{"createUpload":{"id":%q,"url":%q,"headers":[]}}}`, id, m.srv.URL+"/uploads/"+id)
		case "CreateSkill":
			input := req.Variables["input"].(map[string]any)
			source := input["source"].(map[string]any)
			var files []map[string]any
			for _, f := range source["files"].([]any) {
				f := f.(map[string]any)
				m.created = append(m.created, f)
				path, upload := f["path"].(string), f["contentUpload"].(string)
				file := map[string]any{"path": path, "content": nil, "downloadURL": m.srv.URL + "/downloads/" + upload}
				if path == "SKILL.md" {
					file = map[string]any{"path": path, "content": m.uploads[upload], "downloadURL": nil}
				}
				files = append(files, file)
			}
			resp, _ := json.Marshal(map[string]any{"data": map[string]any{"createSkill": map[string]any{
				"id": "cskl_1", "name": input["name"], "declaredName": "pdf-forms", "description": "Fills PDF forms.",
				"instructions": "Fill the form.", "files": files, "createdAt": "2026-11-17T00:00:00Z", "updatedAt": "2026-11-17T00:00:00Z",
			}}})
			w.Write(resp)
		default:
			t.Errorf("unexpected operation %q", req.OperationName)
		}
	})
	mux.HandleFunc("PUT /uploads/{id}", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.uploads[r.PathValue("id")] = string(data)
		m.putAuth = append(m.putAuth, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("GET /downloads/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.downAuth = append(m.downAuth, r.Header.Get("Authorization"))
		io.WriteString(w, m.uploads[r.PathValue("id")])
	})
	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func partite(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
	t.Setenv("PARTITE_CONFIG_DIR", t.TempDir())
	t.Setenv("PARTITE_NO_KEYRING", "1")
	t.Setenv("PARTITE_CONTEXT", "local")
	t.Setenv("PARTITE_URL", srv.URL)
	t.Setenv("PARTITE_API_TOKEN", "")
	t.Setenv("PARTITE_API_KEY", "")
	t.Setenv("PARTITE_TOKEN", "the-token")
	var out, errOut bytes.Buffer
	app := cmdutil.NewApp()
	app.In, app.Out, app.Err = strings.NewReader(""), &out, &errOut
	cmd := root.New(app, skill.New)
	cmd.SetArgs(args)
	code := 0
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		e := app.Classify(err)
		app.PrintError(&errOut, e)
		code = e.Code
	}
	return out.String(), errOut.String(), code
}

// Each file of a folder is sent as an upload, without the CLI's credentials.
// A file the server returns only by a download URL is read from there.
func TestCreateSendsFilesAsUploads(t *testing.T) {
	m := newFakeMesh(t)
	dir := t.TempDir()
	for path, content := range map[string]string{"SKILL.md": skillMD, "reference/fields.md": "the fields"} {
		p := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, errOut, code := partite(t, m.srv, "skill", "create", "pdf-forms", "--dir", dir, "-o", "json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if len(m.created) != 2 {
		t.Fatalf("the skill was made from %d files, want 2", len(m.created))
	}
	for _, f := range m.created {
		if _, ok := f["text"]; ok {
			t.Errorf("%v was sent as text", f["path"])
		}
	}
	for _, auth := range append(m.putAuth, m.downAuth...) {
		if auth != "" {
			t.Errorf("an upload or download carried credentials: %q", auth)
		}
	}
	var s skill.Skill
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if s.Files["SKILL.md"] != skillMD || s.Files["reference/fields.md"] != "the fields" {
		t.Errorf("files = %v", s.Files)
	}
}
