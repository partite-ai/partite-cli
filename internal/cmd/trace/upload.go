package trace

import (
	"errors"
	"net/http"
	"strings"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

func newUpload(app *cmdutil.App) *cobra.Command {
	var conversations []string
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Send the traces a mesh with cloud config recorded to the cloud",
		Long: `A mesh with cloud config records its traces itself. With
CLOUD_TRACE_UPLOAD=manual, it sends them to the cloud only when asked to. This
asks it to: it queues the rows of the traces that haven't been sent, of the
conversations named or of every conversation, and the mesh sends them in the
background.

Run it against the mesh, with a context whose server is the mesh.`,
		Example: "  partite --context my-mesh trace upload\n  partite --context my-mesh trace upload --conversation rcnv_8Xk2",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rest, err := app.REST()
			if err != nil {
				return err
			}
			body := map[string][]string{"conversation_ids": conversations}
			var result struct {
				QueuedRows            int64    `json:"queued_rows"`
				UntracedConversations []string `json:"untraced_conversations,omitempty"`
			}
			err = rest.JSON(cmd.Context(), http.MethodPost, "/traces/upload", body, &result)
			var status *cmdutil.StatusError
			if errors.As(err, &status) && status.StatusCode == http.StatusConflict {
				return &cmdutil.Error{Code: cmdutil.ExitConflict, Msg: "this mesh doesn't upload traces", Hint: "Set CLOUD_TRACE_UPLOAD=manual or automatic on the mesh, which must have cloud config."}
			}
			if err != nil {
				return err
			}
			p := app.Printer()
			p.Done("Queued %d rows to send to the cloud.", result.QueuedRows)
			if len(result.UntracedConversations) > 0 {
				p.Done("Conversations with no trace: %s.", strings.Join(result.UntracedConversations, ", "))
			}
			if app.Format() == output.Table {
				return nil
			}
			return p.Data(result)
		},
	}
	cmd.Flags().StringArrayVar(&conversations, "conversation", nil, "only the trace of this conversation (id); repeat for more")
	return cmd
}
