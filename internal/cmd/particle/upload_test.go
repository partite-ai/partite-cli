package particle_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/partite-ai/partite-cli/internal/cmd/particle"
	"github.com/partite-ai/partite-cli/internal/cmd/root"
	"github.com/partite-ai/partite-cli/internal/cmdutil"
)

// sent is what the fake mesh received.
type sent struct {
	createUpload  map[string]any
	claimed       any
	body          string
	contentLength int64
	checksum      string
	authorization string
}

// fakeMesh makes uploads, takes PUTs to them, and stores a particle artifact
// from one.
func fakeMesh(t *testing.T) (*httptest.Server, *sent) {
	got := &sent{}
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
		if r.Header.Get("Authorization") == "" {
			t.Errorf("%s was sent without credentials", req.OperationName)
		}
		switch req.OperationName {
		case "CreateUpload":
			got.createUpload = req.Variables
			fmt.Fprintf(w, `{"data":{"createUpload":{"id":"up1","url":%q,"headers":[{"name":"X-Amz-Checksum-Sha256","value":"signed"}]}}}`, srv.URL+"/uploads/tok")
		case "UploadParticleArtifact":
			got.claimed = req.Variables["upload"]
			fmt.Fprint(w, `{"data":{"uploadParticleArtifact":{"id":"cpar_1","name":"hello","version":"1.0.0","sha256":"abc","description":null,"capabilities":{"http":null},"credentials":[],"tools":[],"createdAt":"2026-11-17T00:00:00Z"}}}`)
		default:
			t.Errorf("unexpected operation %q", req.OperationName)
		}
	})
	mux.HandleFunc("PUT /uploads/tok", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		got.body, got.contentLength = string(data), r.ContentLength
		got.checksum, got.authorization = r.Header.Get("X-Amz-Checksum-Sha256"), r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
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
	t.Setenv("PARTITE_TOKEN", "the-token")
	var out, errOut bytes.Buffer
	app := cmdutil.NewApp()
	app.In, app.Out, app.Err = strings.NewReader(""), &out, &errOut
	cmd := root.New(app, particle.New)
	cmd.SetArgs(args)
	code := 0
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		e := app.Classify(err)
		app.PrintError(&errOut, e)
		code = e.Code
	}
	return errOut.String(), code
}

// An artifact is sent to an upload, with its size and digest and without the
// CLI's credentials, and the upload is then stored as the artifact.
func TestUploadSendsTheArtifactToAnUpload(t *testing.T) {
	srv, got := fakeMesh(t)
	const content = "the particle's bytes"
	path := filepath.Join(t.TempDir(), "hello.particle")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if errOut, code := partite(t, srv, "particle", "upload", path); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	sum := sha256.Sum256([]byte(content))
	if got.createUpload["size"] != float64(len(content)) || got.createUpload["sha256"] != hex.EncodeToString(sum[:]) {
		t.Errorf("made an upload with %v", got.createUpload)
	}
	if got.body != content || got.contentLength != int64(len(content)) || got.checksum != "signed" {
		t.Errorf("PUT %q with length %d and checksum header %q", got.body, got.contentLength, got.checksum)
	}
	if got.authorization != "" {
		t.Errorf("the PUT carried credentials: %q", got.authorization)
	}
	if got.claimed != "up1" {
		t.Errorf("uploadParticleArtifact was given upload %v, want up1", got.claimed)
	}
}
