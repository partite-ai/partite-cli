package trace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/partite-ai/partite-cli/internal/cmd/root"
	"github.com/partite-ai/partite-cli/internal/cmd/trace"
	"github.com/partite-ai/partite-cli/internal/cmdutil"
)

// partite runs the CLI against srv, and returns what it printed and its exit
// code.
func partite(t *testing.T, srv *httptest.Server, args ...string) (string, string, int) {
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
	cmd := root.New(app, trace.New)
	cmd.SetArgs(args)
	code := 0
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		e := app.Classify(err)
		app.PrintError(&errOut, e)
		code = e.Code
	}
	return out.String(), errOut.String(), code
}

// trace upload asks the mesh to queue the traces of the conversations named,
// and prints how many rows it queued.
func TestUploadQueuesTheConversationsNamed(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/traces/upload" {
			http.NotFound(w, r)
			return
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"queued_rows":12}`))
	}))
	t.Cleanup(srv.Close)

	out, errOut, code := partite(t, srv, "traces", "upload", "--conversation", "rcnv_1", "-o", "json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var sent struct {
		ConversationIDs []string `json:"conversation_ids"`
	}
	if err := json.Unmarshal([]byte(body), &sent); err != nil || len(sent.ConversationIDs) != 1 || sent.ConversationIDs[0] != "rcnv_1" {
		t.Errorf("sent %s", body)
	}
	if !strings.Contains(out, `"queued_rows": 12`) {
		t.Errorf("printed %s", out)
	}
}

// trace upload names the conversations the mesh has no trace of.
func TestUploadNamesConversationsWithoutATrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"queued_rows":3,"untraced_conversations":["rcnv_2"]}`))
	}))
	t.Cleanup(srv.Close)

	out, errOut, code := partite(t, srv, "traces", "upload", "--conversation", "rcnv_1", "--conversation", "rcnv_2", "-o", "table")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "Queued 3 rows") || !strings.Contains(out, "no trace: rcnv_2.") {
		t.Errorf("printed %s", out)
	}
}

// A mesh that doesn't upload traces says so.
func TestUploadToAMeshThatDoesNotUpload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"status":409,"detail":"this mesh does not upload traces"}`))
	}))
	t.Cleanup(srv.Close)

	_, errOut, code := partite(t, srv, "traces", "upload")
	if code != cmdutil.ExitConflict || !strings.Contains(errOut, "CLOUD_TRACE_UPLOAD") {
		t.Errorf("exit %d: %s", code, errOut)
	}
}
