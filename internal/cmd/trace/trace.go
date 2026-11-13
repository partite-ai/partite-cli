// Package trace shows and deletes traces: the record of what a mesh did for
// each request of a conversation.
package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/partite-ai/partite-cli/internal/cmd/conversation/convapi"
	"github.com/partite-ai/partite-cli/internal/cmd/conversation/convui"
	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

// Mesh is the mesh version a trace ran on, as it was then.
type Mesh struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	VersionID      string `json:"version_id"`
	VersionOrdinal int    `json:"version"`
	ModelSet       string `json:"model_set,omitempty"`
}

// Summary is a trace as list prints it.
type Summary struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversation_id"`
	CreatedAt      string           `json:"created_at"`
	Mesh           Mesh             `json:"mesh"`
	Requests       []RequestSummary `json:"requests"`
}

type RequestSummary struct {
	ID        string `json:"id"`
	RequestID string `json:"request_id"`
	CreatedAt string `json:"created_at"`
	Status    string `json:"status,omitempty"`
}

func status(s *gql.RequestStatus) string {
	if s == nil {
		return ""
	}
	return strings.ToLower(string(*s))
}

func summaryFromAPI(t gql.RuntimeTraceSummary) Summary {
	reqs := make([]RequestSummary, len(t.Requests))
	for i, r := range t.Requests {
		reqs[i] = RequestSummary{ID: r.Id, RequestID: r.ConversationRequestID, CreatedAt: r.CreatedAt, Status: status(r.Status)}
	}
	m := t.MeshSnapshot
	return Summary{
		ID:             t.Id,
		ConversationID: t.ConversationID,
		CreatedAt:      t.CreatedAt,
		Mesh:           Mesh{ID: m.Id, Name: m.Name, VersionID: m.VersionID, VersionOrdinal: m.VersionOrdinal},
		Requests:       reqs,
	}
}

func New(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "trace",
		Aliases: []string{"traces"},
		GroupID: "runtime",
		Short:   "Show what a mesh did: requests, agents, model calls, tokens and cost",
		Long: `A trace records what a mesh did in a conversation: for each request, which
agents ran, what they were asked and said, the tools they called, and every
model call with its tokens and cost. A conversation has one trace, unless it
was started with tracing off.

A trace is named by its id (ttrc_...) or by its conversation's id (rcnv_...).`,
	}
	cmd.AddCommand(newList(app), newGet(app), newView(app), newDelete(app))
	return cmd
}

// parseTime reads a time for --since and --until: a duration back from now,
// such as 90m, 6h or 2d, or a date or RFC 3339 time.
func parseTime(flag, s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if strings.HasSuffix(s, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil {
			return time.Now().Add(-time.Duration(n) * 24 * time.Hour).UTC().Format(time.RFC3339), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d).UTC().Format(time.RFC3339), nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.UTC().Format(time.RFC3339), nil
		}
	}
	return "", cmdutil.UsageError("%s %q is not a duration (such as 6h or 2d), a date (2006-01-02) or an RFC 3339 time", flag, s)
}

func newList(app *cmdutil.App) *cobra.Command {
	var mesh, meshVersion, since, until, statusFlag, search string
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List traces, newest first",
		Example: `  partite trace list --mesh support --since 1d
  partite trace list --status active
  partite trace list --search "refund" --limit 10 -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := app.GQL()
			if err != nil {
				return err
			}
			var filter gql.TraceFilter
			if mesh != "" {
				id, err := convui.ResolveMeshID(ctx, app, mesh)
				if err != nil {
					return err
				}
				filter.MeshID = &id
			}
			filter.MeshVersionID = cmdutil.OptString(meshVersion)
			after, err := parseTime("--since", since)
			if err != nil {
				return err
			}
			before, err := parseTime("--until", until)
			if err != nil {
				return err
			}
			filter.CreatedAfter, filter.CreatedBefore = cmdutil.OptString(after), cmdutil.OptString(before)
			switch statusFlag {
			case "":
			case "active", "cancelled", "complete", "failed":
				s := gql.RequestStatus(strings.ToUpper(statusFlag))
				filter.Status = &s
			default:
				return cmdutil.UsageError("--status must be active, cancelled, complete or failed")
			}
			traces, err := cmdutil.ListAll(ctx, limit, func(ctx context.Context, first int, cursor *string) (cmdutil.Page[Summary], error) {
				resp, err := gql.RuntimeListTraces(ctx, c, first, cursor, cmdutil.OptString(search), &filter)
				if err != nil {
					return cmdutil.Page[Summary]{}, err
				}
				conn := resp.Traces
				items := make([]Summary, len(conn.Edges))
				for i, e := range conn.Edges {
					items[i] = summaryFromAPI(e.Node)
				}
				return cmdutil.Page[Summary]{Items: items, EndCursor: conn.PageInfo.EndCursor, HasNext: conn.PageInfo.HasNextPage}, nil
			})
			if err != nil {
				return err
			}
			if traces == nil {
				traces = []Summary{}
			}
			rows := make([][]string, len(traces))
			for i, t := range traces {
				last := ""
				if n := len(t.Requests); n > 0 {
					last = t.Requests[n-1].Status
				}
				rows[i] = []string{t.ID, t.ConversationID, t.Mesh.Name, strconv.Itoa(t.Mesh.VersionOrdinal), strconv.Itoa(len(t.Requests)), last, cmdutil.FormatTime(t.CreatedAt)}
			}
			return app.Printer().List(traces, []string{"id", "conversation", "mesh", "version", "requests", "status", "created"}, rows)
		},
	}
	f := cmd.Flags()
	f.StringVar(&mesh, "mesh", "", "only traces of this mesh (name or id)")
	f.StringVar(&meshVersion, "mesh-version", "", "only traces of this mesh version (id)")
	f.StringVar(&since, "since", "", "only traces created since then: a duration such as 6h or 2d, or a time")
	f.StringVar(&until, "until", "", "only traces created before then: a duration such as 6h or 2d, or a time")
	f.StringVar(&statusFlag, "status", "", "only traces with a request in this status: active, cancelled, complete or failed")
	f.StringVar(&search, "search", "", "only traces matching this text")
	f.IntVar(&limit, "limit", 50, "most traces to list; 0 for all")
	_ = cmd.RegisterFlagCompletionFunc("status", cobra.FixedCompletions([]string{"active", "cancelled", "complete", "failed"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

// fetch reads a trace by its id, its conversation's id, or its
// conversation's external id.
func fetch(ctx context.Context, app *cmdutil.App, ref string) (gql.RuntimeTraceDetail, error) {
	c, err := app.GQL()
	if err != nil {
		return gql.RuntimeTraceDetail{}, err
	}
	var id, conv *string
	switch {
	case cmdutil.IsID(ref, "ttrc"):
		id = &ref
	case cmdutil.IsID(ref, "rcnv"):
		conv = &ref
	default:
		rest, err := app.REST()
		if err != nil {
			return gql.RuntimeTraceDetail{}, err
		}
		convID, err := convui.ResolveConversation(ctx, &convapi.Client{REST: rest}, ref)
		if err != nil {
			return gql.RuntimeTraceDetail{}, &cmdutil.Error{Code: cmdutil.ExitNotFound, Msg: fmt.Sprintf("trace %q not found", ref), Hint: "Name a trace by its id (ttrc_...) or its conversation's id (rcnv_...). List them with: partite trace list"}
		}
		conv = &convID
	}
	resp, err := gql.RuntimeGetTrace(ctx, c, id, conv)
	if err != nil {
		e := app.Classify(err)
		if e.Code == cmdutil.ExitNotFound {
			e.Msg = fmt.Sprintf("trace %q not found", ref)
			e.Hint = "A conversation started with tracing off has no trace. List traces with: partite trace list"
		}
		return gql.RuntimeTraceDetail{}, e
	}
	return resp.Trace, nil
}

func newGet(app *cmdutil.App) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:     "get <trace|conversation>",
		Aliases: []string{"show"},
		Short:   "Show a trace as a tree of requests, agents and model calls",
		Long: `Show a trace: each request, the agents that ran for it (nested where one
agent handed work to another), what they said and the tools they called, and
each model call with its time, tokens and cost. Totals are at the top.

Long text is cut to one line; --full shows all of it, and the prompts too.
With -o json the output has everything.`,
		Example: `  partite trace get rcnv_8Xk2
  partite trace get ttrc_4Hn1 --full
  partite trace get rcnv_8Xk2 -o json | jq .usage`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := fetch(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			v := fromAPI(t)
			if app.Format() == output.Table {
				writeTree(app.Out, convui.NewStyle(app.Out), v, full)
				return nil
			}
			return app.Printer().Object(v, v.ID, nil)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "show long text and prompts in full")
	return cmd
}

func newView(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "view [trace|conversation]",
		Short: "Open a trace in the Partite web app",
		Long: `Open a trace in the Partite web app, or the list of traces when none is
given. It prints the page's URL too. The context must have a web app, which
the cloud context has.`,
		Example: `  partite trace view
  partite trace view rcnv_8Xk2`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return app.OpenWeb("observability/traces")
			}
			id := args[0]
			if !cmdutil.IsID(id, "ttrc") {
				t, err := fetch(cmd.Context(), app, id)
				if err != nil {
					return err
				}
				id = t.Id
			}
			return app.OpenRecord("trace", "observability/traces/%s", id)
		},
	}
}

func newDelete(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <trace|conversation>...",
		Aliases: []string{"rm"},
		Short:   "Delete traces",
		Example: "  partite trace delete ttrc_4Hn1\n  partite trace delete rcnv_8Xk2 rcnv_2Lm4 --yes",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ids := make([]string, len(args))
			for i, ref := range args {
				if cmdutil.IsID(ref, "ttrc") {
					ids[i] = ref
					continue
				}
				t, err := fetch(ctx, app, ref)
				if err != nil {
					return err
				}
				ids[i] = t.Id
			}
			what := "trace " + strings.Join(args, ", ")
			if len(args) > 1 {
				what = cmdutil.Plural(len(args), "trace") + ": " + strings.Join(args, ", ")
			}
			if err := app.Confirm(fmt.Sprintf("Delete %s?", what)); err != nil {
				return err
			}
			c, err := app.GQL()
			if err != nil {
				return err
			}
			resp, err := gql.RuntimeDeleteTraces(ctx, c, ids)
			if err != nil {
				return err
			}
			app.Printer().Done("Deleted %s.", what)
			if app.Format() != output.Table {
				deleted := resp.DeleteTraces.Ids
				if deleted == nil {
					deleted = []string{}
				}
				return app.Printer().Data(map[string]any{"deleted": deleted})
			}
			return nil
		},
	}
}

// jsonOrNil drops JSON nulls, so that omitempty leaves them out.
func jsonOrNil(raw *json.RawMessage) json.RawMessage {
	if raw == nil || string(*raw) == "null" {
		return nil
	}
	return *raw
}
