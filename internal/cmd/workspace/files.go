package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

// File is one entry of a workspace folder.
type File struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	SizeBytes *int   `json:"size_bytes,omitempty"`
}

// splitPath turns a workspace path into its folder and name. Paths are
// relative to the workspace root, with or without a leading slash.
func splitPath(p string) (folder, name string, err error) {
	p = path.Clean("/" + p)
	if p == "/" {
		return "", "", cmdutil.UsageError("a file path is required, not the workspace root")
	}
	return path.Dir(p), path.Base(p), nil
}

// operationError turns a failed OperationResult into an error.
func operationError(success bool, message *string, what string) error {
	if success {
		return nil
	}
	return errors.New(what + ": " + cmdutil.Deref(message))
}

func addConversationFlag(cmd *cobra.Command, conversation *string) {
	cmd.Flags().StringVar(conversation, "conversation", "", "conversation whose copy of a CONVERSATION workspace to use")
}

func newFiles(app *cmdutil.App) *cobra.Command {
	var conversation string
	var limit int
	cmd := &cobra.Command{
		Use:   "files <workspace> [folder]",
		Short: "List the files in a workspace folder",
		Example: `  partite workspace files notes
  partite workspace files notes /reports
  partite workspace files scratch --conversation conv-123 -o json`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := Resource.Resolve(ctx, app, args[0])
			if err != nil {
				return err
			}
			folder := "/"
			if len(args) == 2 {
				folder = path.Clean("/" + args[1])
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			files, err := cmdutil.ListAll(ctx, limit, func(ctx context.Context, first int, after *string) (cmdutil.Page[File], error) {
				resp, err := gql.ToolsWorkspaceFiles(ctx, c, id, cmdutil.OptString(conversation), folder, first, after)
				if err != nil {
					return cmdutil.Page[File]{}, err
				}
				conn := resp.WorkspaceFiles
				items := make([]File, len(conn.Edges))
				for i, e := range conn.Edges {
					items[i] = File{Name: e.Node.Name, Type: strings.ToLower(string(e.Node.FileType))}
					// A folder's size is the filesystem's, not its contents'.
					if e.Node.FileType == gql.WorkspaceFileTypeFile {
						items[i].SizeBytes = e.Node.SizeBytes
					}
				}
				return cmdutil.Page[File]{Items: items, EndCursor: conn.PageInfo.EndCursor, HasNext: conn.PageInfo.HasNextPage}, nil
			})
			if err != nil {
				return err
			}
			if files == nil {
				files = []File{}
			}
			rows := make([][]string, len(files))
			for i, f := range files {
				size := ""
				if f.SizeBytes != nil {
					size = fmt.Sprint(*f.SizeBytes)
				}
				name := f.Name
				if f.Type == "folder" {
					name += "/"
				}
				rows[i] = []string{name, f.Type, size}
			}
			return app.Printer().List(files, []string{"name", "type", "size"}, rows)
		},
	}
	addConversationFlag(cmd, &conversation)
	cmd.Flags().IntVar(&limit, "limit", 0, "most results to return (default: all)")
	return cmd
}

func newUpload(app *cmdutil.App) *cobra.Command {
	var conversation string
	cmd := &cobra.Command{
		Use:   "upload <workspace> <local-file> [remote-path]",
		Short: "Upload a file to a workspace",
		Long: `Upload a local file to a workspace. It fails if a file already exists at
that path. The remote path defaults to the file's name in the root folder. A
remote path that ends in / is a folder, and the file keeps its name.`,
		Example: `  partite workspace upload notes report.pdf
  partite workspace upload notes report.pdf /reports/
  partite workspace upload notes report.pdf /reports/2026-q3.pdf`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := Resource.Resolve(ctx, app, args[0])
			if err != nil {
				return err
			}
			local := args[1]
			remote := "/" + filepath.Base(local)
			if len(args) == 3 {
				remote = args[2]
				if strings.HasSuffix(remote, "/") {
					remote += filepath.Base(local)
				}
			}
			folder, name, err := splitPath(remote)
			if err != nil {
				return err
			}
			f, err := os.Open(local)
			if err != nil {
				return cmdutil.UsageError("open %s: %s", local, err)
			}
			defer f.Close()
			if err := uploadFile(ctx, app, id, conversation, folder, name, f); err != nil {
				return err
			}
			full := path.Join(folder, name)
			app.Printer().Done("Uploaded %s to %s.", local, full)
			if app.Format() != output.Table {
				return app.Printer().Data(map[string]string{"workspace_id": id, "path": full})
			}
			return nil
		},
	}
	addConversationFlag(cmd, &conversation)
	return cmd
}

// uploadFile asks for an upload link and PUTs the file's bytes to it. The
// file is streamed rather than read into memory, and may be as large as the
// organization's storage allows.
func uploadFile(ctx context.Context, app *cmdutil.App, id, conversation, folder, name string, f *os.File) error {
	c, err := app.GQL()
	if err != nil {
		return err
	}
	resp, err := gql.ToolsCreateWorkspaceFileUpload(ctx, c, id, cmdutil.OptString(conversation), folder, name)
	if err != nil {
		return err
	}
	// The URL carries its own authorization, so no credentials are sent.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, resp.CreateWorkspaceFileUpload.Url, f)
	if err != nil {
		return err
	}
	// A regular file's length is known; anything else is sent chunked.
	if st, err := f.Stat(); err == nil && st.Mode().IsRegular() {
		req.ContentLength = st.Size()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	up, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer up.Body.Close()
	if up.StatusCode/100 != 2 {
		data, _ := io.ReadAll(io.LimitReader(up.Body, 64<<10))
		return &cmdutil.StatusError{StatusCode: up.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	return nil
}

func newDownload(app *cmdutil.App) *cobra.Command {
	var conversation string
	cmd := &cobra.Command{
		Use:   "download <workspace> <remote-path> [local-path]",
		Short: "Download a file from a workspace",
		Long: `Download a file from a workspace. The local path defaults to the file's name
in the current folder. Use - to write to standard output.`,
		Example: `  partite workspace download notes /reports/2026-q3.pdf
  partite workspace download notes todo.md - | less`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := Resource.Resolve(ctx, app, args[0])
			if err != nil {
				return err
			}
			folder, name, err := splitPath(args[1])
			if err != nil {
				return err
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			resp, err := gql.ToolsWorkspaceFileDownload(ctx, c, id, cmdutil.OptString(conversation), folder, name)
			if err != nil {
				return err
			}
			link := resp.WorkspaceFileDownload
			local := link.FileName
			if len(args) == 3 {
				local = args[2]
			}
			// The URL carries its own authorization, so no credentials are sent.
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.Url, nil)
			if err != nil {
				return err
			}
			dl, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer dl.Body.Close()
			if dl.StatusCode/100 != 2 {
				data, _ := io.ReadAll(io.LimitReader(dl.Body, 64<<10))
				return &cmdutil.StatusError{StatusCode: dl.StatusCode, Body: string(data)}
			}
			if local == "-" {
				_, err := io.Copy(app.Out, dl.Body)
				return err
			}
			if st, err := os.Stat(local); err == nil && st.IsDir() {
				local = filepath.Join(local, link.FileName)
			}
			out, err := os.Create(local)
			if err != nil {
				return cmdutil.UsageError("create %s: %s", local, err)
			}
			n, err := io.Copy(out, dl.Body)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			app.Printer().Done("Downloaded %s to %s (%d bytes).", path.Join(folder, name), local, n)
			if app.Format() != output.Table {
				return app.Printer().Data(map[string]any{"path": local, "size_bytes": n})
			}
			return nil
		},
	}
	addConversationFlag(cmd, &conversation)
	return cmd
}

func newMkdir(app *cmdutil.App) *cobra.Command {
	var conversation string
	cmd := &cobra.Command{
		Use:     "mkdir <workspace> <path>",
		Short:   "Create a folder in a workspace",
		Example: "  partite workspace mkdir notes /reports",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := Resource.Resolve(ctx, app, args[0])
			if err != nil {
				return err
			}
			folder, name, err := splitPath(args[1])
			if err != nil {
				return err
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			resp, err := gql.ToolsCreateWorkspaceFolder(ctx, c, id, cmdutil.OptString(conversation), folder, name)
			if err != nil {
				return err
			}
			r := resp.CreateWorkspaceFolder
			if err := operationError(r.Success, r.Message, "could not create the folder"); err != nil {
				return err
			}
			full := path.Join(folder, name)
			app.Printer().Done("Created folder %s.", full)
			if app.Format() != output.Table {
				return app.Printer().Data(map[string]string{"workspace_id": id, "path": full})
			}
			return nil
		},
	}
	addConversationFlag(cmd, &conversation)
	return cmd
}

func newRm(app *cmdutil.App) *cobra.Command {
	var conversation string
	cmd := &cobra.Command{
		Use:     "rm <workspace> <path>...",
		Short:   "Delete files or folders from a workspace",
		Example: "  partite workspace rm notes /reports/old.pdf --yes",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := Resource.Resolve(ctx, app, args[0])
			if err != nil {
				return err
			}
			paths := args[1:]
			for _, p := range paths {
				if _, _, err := splitPath(p); err != nil {
					return err
				}
			}
			if err := app.Confirm(fmt.Sprintf("Delete %s from workspace %s?", strings.Join(paths, ", "), args[0])); err != nil {
				return err
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			deleted := make([]string, 0, len(paths))
			for _, p := range paths {
				folder, name, _ := splitPath(p)
				resp, err := gql.ToolsDeleteWorkspaceFile(ctx, c, id, cmdutil.OptString(conversation), folder, name)
				if err != nil {
					return err
				}
				r := resp.DeleteWorkspaceFile
				if err := operationError(r.Success, r.Message, "could not delete "+p); err != nil {
					return err
				}
				deleted = append(deleted, path.Join(folder, name))
			}
			app.Printer().Done("Deleted %s.", strings.Join(deleted, ", "))
			if app.Format() != output.Table {
				return app.Printer().Data(map[string]any{"deleted": deleted})
			}
			return nil
		},
	}
	addConversationFlag(cmd, &conversation)
	return cmd
}
