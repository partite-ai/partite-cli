package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
	"github.com/spf13/cobra"
)

// Rubric is the view printed by get and read by -f.
type Rubric struct {
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Metrics     []RubricMetric `json:"metrics"`
	CreatedAt   string         `json:"created_at,omitempty"`
	UpdatedAt   string         `json:"updated_at,omitempty"`
}

type rubricAgentTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RubricMetric is one metric of a rubric. Exactly one of JudgeMetricID and
// ComputedMetricKey is set. On input, JudgeMetricID may be a name and
// ComputedMetricKey may be a computed metric's name.
type RubricMetric struct {
	JudgeMetricID     string          `json:"judge_metric_id,omitempty"`
	ComputedMetricKey string          `json:"computed_metric_key,omitempty"`
	MetricName        string          `json:"metric_name,omitempty"`
	Weight            *float64        `json:"weight,omitempty"`
	Tags              json.RawMessage `json:"tags"`

	// Judge metrics only.
	TrialCount  *int            `json:"trial_count,omitempty"`
	Complexity  string          `json:"complexity,omitempty"`
	InputValues json.RawMessage `json:"input_values,omitempty"`

	// Computed metrics only.
	LowerIsBetter *bool    `json:"lower_is_better,omitempty"`
	ScoreFloor    *float64 `json:"score_floor,omitempty"`
	ScoreCeiling  *float64 `json:"score_ceiling,omitempty"`

	// entryID is the id the server gives this metric within the rubric.
	entryID string
}

func (m RubricMetric) isJudge() bool { return m.JudgeMetricID != "" }

// key matches a wanted metric to one the rubric holds. A rubric has each
// metric once, so the metric itself is the identity.
func (m RubricMetric) key() string {
	if m.isJudge() {
		return "judge:" + m.JudgeMetricID
	}
	return "computed:" + m.ComputedMetricKey
}

func (m RubricMetric) label() string {
	if m.MetricName != "" {
		return m.MetricName
	}
	if m.isJudge() {
		return m.JudgeMetricID
	}
	return m.ComputedMetricKey
}

func rubricFromAPI(d gql.EnterpriseRubric) Rubric {
	r := Rubric{
		ID:          d.Id,
		Name:        d.Name,
		Description: cmdutil.Deref(d.Description),
		Metrics:     []RubricMetric{},
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
	for _, metric := range d.Metrics {
		switch m := metric.(type) {
		case *gql.EnterpriseRubricMetricsRubricJudgeMetric:
			trials := m.TrialCount
			tags := make([]rubricAgentTag, 0, len(m.Tags))
			for _, tag := range m.Tags {
				tags = append(tags, rubricAgentTag{Name: tag.Name, Value: tag.Value})
			}
			rawTags, _ := json.Marshal(tags)
			entry := RubricMetric{
				JudgeMetricID: m.JudgeMetric.Id,
				MetricName:    m.JudgeMetric.Name,
				Weight:        m.JudgeWeight,
				TrialCount:    &trials,
				Tags:          rawTags,
				entryID:       m.JudgeEntryID,
			}
			if m.Complexity != nil {
				entry.Complexity = string(*m.Complexity)
			}
			// The server stores {} for no input values; show it as none.
			if m.InputValues != nil && !isEmptyJSONObject(*m.InputValues) {
				entry.InputValues = *m.InputValues
			}
			r.Metrics = append(r.Metrics, entry)
		case *gql.EnterpriseRubricMetricsRubricComputedMetric:
			tags := make([]rubricAgentTag, 0, len(m.Tags))
			for _, tag := range m.Tags {
				tags = append(tags, rubricAgentTag{Name: tag.Name, Value: tag.Value})
			}
			rawTags, _ := json.Marshal(tags)
			r.Metrics = append(r.Metrics, RubricMetric{
				ComputedMetricKey: m.ComputedMetric.Key,
				MetricName:        m.ComputedMetric.Name,
				Weight:            m.ComputedWeight,
				Tags:              rawTags,
				LowerIsBetter:     m.LowerIsBetter,
				ScoreFloor:        m.ScoreFloor,
				ScoreCeiling:      m.ScoreCeiling,
				entryID:           m.ComputedEntryID,
			})
		}
	}
	return r
}

func isEmptyJSONObject(raw []byte) bool {
	var object map[string]any
	return json.Unmarshal(raw, &object) == nil && object != nil && len(object) == 0
}

func decodeTags(raw json.RawMessage, tags *[]*rubricAgentTag) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(tags)
}

// checkRubricMetric checks the fields of one metric and normalizes them: the
// complexity is upper-cased and input values given as a JSON string are
// unwrapped. It does not resolve names.
func checkRubricMetric(m RubricMetric) (RubricMetric, error) {
	switch {
	case m.JudgeMetricID != "" && m.ComputedMetricKey != "":
		return m, cmdutil.UsageError("a rubric metric has both judge_metric_id %q and computed_metric_key %q; give one", m.JudgeMetricID, m.ComputedMetricKey)
	case m.JudgeMetricID == "" && m.ComputedMetricKey == "":
		return m, cmdutil.UsageError("a rubric metric needs judge_metric_id or computed_metric_key")
	}
	if len(m.Tags) > 0 && string(bytes.TrimSpace(m.Tags)) != "null" {
		var tags []*rubricAgentTag
		if err := decodeTags(m.Tags, &tags); err != nil || tags == nil {
			return m, cmdutil.UsageError("metric %s: tags must be a list of name/value pairs", m.label())
		}
		for _, tag := range tags {
			if tag == nil || tag.Name == "" || tag.Value == "" {
				return m, cmdutil.UsageError("metric %s: tags require a non-empty name and value", m.label())
			}
		}
	}
	if m.isJudge() {
		if m.LowerIsBetter != nil || m.ScoreFloor != nil || m.ScoreCeiling != nil {
			return m, cmdutil.UsageError("judge metric %s: lower_is_better, score_floor and score_ceiling are only for computed metrics", m.JudgeMetricID)
		}
		if m.Complexity != "" {
			c, err := upperChoice("complexity", m.Complexity, gql.AllModelComplexity)
			if err != nil {
				return m, err
			}
			m.Complexity = string(c)
		}
		raw := bytes.TrimSpace(m.InputValues)
		if len(raw) > 0 && raw[0] == '"' {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return m, cmdutil.UsageError("judge metric %s: input_values: %s", m.JudgeMetricID, err)
			}
			raw = []byte(s)
		}
		if len(raw) > 0 {
			var object map[string]any
			if err := json.Unmarshal(raw, &object); err != nil || object == nil {
				return m, cmdutil.UsageError("judge metric %s: input_values must be a JSON object", m.JudgeMetricID)
			}
			if len(object) == 0 {
				raw = nil
			}
		}
		m.InputValues = json.RawMessage(raw)
		return m, nil
	}
	if m.TrialCount != nil || m.Complexity != "" || len(m.InputValues) > 0 {
		return m, cmdutil.UsageError("computed metric %s: trial_count, complexity and input_values are only for judge metrics", m.ComputedMetricKey)
	}
	return m, nil
}

// resolveRubricMetrics checks each metric and turns judge metric names and
// computed metric names into ids and keys.
func resolveRubricMetrics(ctx context.Context, app *cmdutil.App, metrics []RubricMetric) ([]RubricMetric, error) {
	var computed []ComputedMetric
	out := make([]RubricMetric, len(metrics))
	seen := map[string]bool{}
	for i, m := range metrics {
		m, err := checkRubricMetric(m)
		if err != nil {
			return nil, err
		}
		if m.isJudge() {
			id, err := judgeMetricResource.Resolve(ctx, app, m.JudgeMetricID)
			if err != nil {
				return nil, err
			}
			m.JudgeMetricID = id
		} else {
			if computed == nil {
				computed, err = computedMetricResource.List(ctx, app, cmdutil.ListOptions{})
				if err != nil {
					return nil, err
				}
			}
			cm, err := findComputedMetric(computed, m.ComputedMetricKey)
			if err != nil {
				return nil, err
			}
			m.ComputedMetricKey = cm.Key
		}
		if seen[m.key()] {
			return nil, cmdutil.UsageError("metric %s is in the rubric twice", m.label())
		}
		seen[m.key()] = true
		out[i] = m
	}
	return out, nil
}

// newRubricMetric is one metric as the create input takes it. The API refuses
// a field that does not apply to the kind of metric, even as null, so only
// the fields of its kind are set.
func newRubricMetric(m RubricMetric) gql.NewRubricMetric {
	in := gql.NewRubricMetric{Weight: m.Weight}
	in.Tags = m.Tags
	if m.isJudge() {
		in.JudgeMetricID = &m.JudgeMetricID
		in.TrialCount = m.TrialCount
		in.Complexity = complexityPtr(m.Complexity)
		in.InputValues = rawPtr(m.InputValues)
		return in
	}
	in.ComputedMetricKey = &m.ComputedMetricKey
	in.ScoreFloor = m.ScoreFloor
	in.ScoreCeiling = m.ScoreCeiling
	in.LowerIsBetter = m.LowerIsBetter
	return in
}

func updateRubricMetric(entryID string, m RubricMetric) gql.UpdateRubricMetric {
	in := gql.UpdateRubricMetric{ID: entryID, Weight: m.Weight}
	in.Tags = m.Tags
	if m.isJudge() {
		in.TrialCount = m.TrialCount
		in.Complexity = complexityPtr(m.Complexity)
		in.InputValues = rawPtr(m.InputValues)
		return in
	}
	in.ScoreFloor = m.ScoreFloor
	in.ScoreCeiling = m.ScoreCeiling
	in.LowerIsBetter = m.LowerIsBetter
	return in
}

func complexityPtr(s string) *gql.ModelComplexity {
	if s == "" {
		return nil
	}
	c := gql.ModelComplexity(s)
	return &c
}

func rawPtr(raw json.RawMessage) *json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return &raw
}

// rubricUpdate builds the update that turns the held metrics into the wanted
// ones. The API takes the difference: metrics to add, to change by entry id,
// and to remove. Both lists must already be resolved to ids and keys.
func rubricUpdate(name, description string, held, want []RubricMetric) gql.UpdateRubric {
	update := gql.UpdateRubric{
		Name:             &name,
		Description:      cmdutil.OptString(description),
		DeletedMetricIDs: []string{},
		UpdatedMetrics:   []gql.UpdateRubricMetric{},
		NewMetrics:       []gql.NewRubricMetric{},
	}
	heldIDs := make(map[string]string, len(held))
	for _, m := range held {
		heldIDs[m.key()] = m.entryID
	}
	wanted := make(map[string]bool, len(want))
	for _, m := range want {
		wanted[m.key()] = true
		if id, ok := heldIDs[m.key()]; ok {
			update.UpdatedMetrics = append(update.UpdatedMetrics, updateRubricMetric(id, m))
		} else {
			update.NewMetrics = append(update.NewMetrics, newRubricMetric(m))
		}
	}
	for _, m := range held {
		if !wanted[m.key()] {
			update.DeletedMetricIDs = append(update.DeletedMetricIDs, m.entryID)
		}
	}
	return update
}

var rubricResource = &cmdutil.Resource[Rubric]{
	WebList:    "observability/rubrics",
	WebPage:    "observability/rubrics/%s",
	WebLink:    "eval-rubric",
	Kind:       "rubric",
	Command:    "eval rubric",
	IDPrefix:   "eurb",
	Enterprise: true,
	List: func(ctx context.Context, app *cmdutil.App, opts cmdutil.ListOptions) ([]Rubric, error) {
		c, err := app.GQL()
		if err != nil {
			return nil, err
		}
		return cmdutil.ListAll(ctx, opts.Limit, func(ctx context.Context, first int, after *string) (cmdutil.Page[Rubric], error) {
			resp, err := gql.EnterpriseListRubrics(ctx, c, first, after, cmdutil.OptString(opts.Query))
			if err != nil {
				return cmdutil.Page[Rubric]{}, err
			}
			conn := resp.Rubrics
			items := make([]Rubric, len(conn.Edges))
			for i, e := range conn.Edges {
				items[i] = rubricFromAPI(e.Node)
			}
			return cmdutil.Page[Rubric]{Items: items, EndCursor: conn.PageInfo.EndCursor, HasNext: conn.PageInfo.HasNextPage}, nil
		})
	},
	Get: func(ctx context.Context, app *cmdutil.App, id string) (Rubric, error) {
		c, err := app.GQL()
		if err != nil {
			return Rubric{}, err
		}
		resp, err := gql.EnterpriseGetRubric(ctx, c, id)
		if err != nil {
			return Rubric{}, err
		}
		if resp.Rubric == nil {
			return Rubric{}, cmdutil.NotFound("rubric", id, "partite eval rubric list")
		}
		return rubricFromAPI(*resp.Rubric), nil
	},
	Delete: func(ctx context.Context, app *cmdutil.App, ids []string) error {
		c, err := app.GQL()
		if err != nil {
			return err
		}
		_, err = gql.EnterpriseDeleteRubrics(ctx, c, ids)
		return err
	},
	ID:      func(r Rubric) string { return r.ID },
	Name:    func(r Rubric) string { return r.Name },
	Columns: []string{"id", "name", "metrics", "description"},
	Row: func(r Rubric) []string {
		names := make([]string, len(r.Metrics))
		for i, m := range r.Metrics {
			names[i] = m.label()
		}
		return []string{r.ID, r.Name, strings.Join(names, ", "), r.Description}
	},
	Fields: func(r Rubric) []output.Field {
		lines := make([]string, len(r.Metrics))
		for i, m := range r.Metrics {
			lines[i] = describeRubricMetric(m)
		}
		return []output.Field{
			{Label: "ID", Value: r.ID},
			{Label: "Name", Value: r.Name},
			{Label: "Description", Value: r.Description},
			{Label: "Metrics", Value: multiline(lines)},
			{Label: "Created", Value: r.CreatedAt},
			{Label: "Updated", Value: r.UpdatedAt},
		}
	},
}

func describeRubricMetric(m RubricMetric) string {
	var parts []string
	if m.Weight != nil {
		parts = append(parts, "weight "+formatScore(m.Weight))
	}
	var tags []rubricAgentTag
	if err := json.Unmarshal(m.Tags, &tags); err == nil && len(tags) > 0 {
		labels := make([]string, 0, len(tags))
		for _, tag := range tags {
			labels = append(labels, tag.Name+"="+tag.Value)
		}
		parts = append(parts, "agent tags "+strings.Join(labels, " or "))
	}
	if m.isJudge() {
		if m.TrialCount != nil {
			parts = append(parts, cmdutil.Plural(*m.TrialCount, "trial"))
		}
		if m.Complexity != "" {
			parts = append(parts, "complexity "+m.Complexity)
		}
		if len(m.InputValues) > 0 {
			parts = append(parts, "inputs "+string(m.InputValues))
		}
	} else {
		if m.LowerIsBetter != nil && *m.LowerIsBetter {
			parts = append(parts, "lower is better")
		}
		if m.ScoreFloor != nil || m.ScoreCeiling != nil {
			parts = append(parts, fmt.Sprintf("range %s..%s", formatScore(m.ScoreFloor), formatScore(m.ScoreCeiling)))
		}
	}
	kind, ref := "judge", m.JudgeMetricID
	if !m.isJudge() {
		kind, ref = "computed", m.ComputedMetricKey
	}
	s := fmt.Sprintf("%s (%s %s)", m.label(), kind, ref)
	if len(parts) > 0 {
		s += ": " + strings.Join(parts, ", ")
	}
	return s
}

const rubricLong = `A rubric is a set of metrics a conversation is scored against. Each metric
is a judge metric (a model answers a question about the conversation) or a
computed metric (Partite measures a number, such as latency). The weights
decide how much each metric counts toward the rubric's score. Attach a rubric
to a mesh with partite eval monitor create.

Simple rubrics can be made with flags. For the other settings, use a file:

  name: support-quality
  metrics:
    - judge_metric_id: Helpfulness     # a judge metric's name or id
      weight: 2
      trial_count: 3                   # judge it 3 times and average
      complexity: HIGH                 # override the metric's complexity
      input_values: {policy: "..."}    # values for the metric's input fields
      tags:                       # match any name/value pair
        - {name: role, value: reviewer}
    - computed_metric_key: latency_ms  # a computed metric's key or name
      weight: 1
      lower_is_better: true
      score_floor: 0
      score_ceiling: 10000

For an existing metric, omit tags to keep its tags. Use [] or null to
clear them. Matching is exact and request-scoped judges ignore agent tags.`

func newRubric(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rubric",
		Aliases: []string{"rubrics"},
		Short:   "Manage rubrics, the sets of metrics a conversation is scored against",
		Long:    rubricLong,
	}
	cmd.AddCommand(
		rubricResource.ListCmd(app),
		rubricResource.GetCmd(app),
		rubricResource.ViewCmd(app),
		newRubricCreate(app),
		newRubricUpdate(app),
		rubricResource.ApplyCmd(app, "  partite eval rubric apply -f support-quality.yaml", func(ctx context.Context, spec Rubric, id string) (Rubric, error) {
			want, err := resolveRubricMetrics(ctx, app, spec.Metrics)
			if err != nil {
				return Rubric{}, err
			}
			if id == "" {
				return createRubric(ctx, app, spec.Name, spec.Description, want)
			}
			cur, err := rubricResource.Get(ctx, app, id)
			if err != nil {
				return Rubric{}, err
			}
			return saveRubric(ctx, app, id, rubricUpdate(spec.Name, spec.Description, cur.Metrics, want))
		}),
		rubricResource.DeleteCmd(app),
	)
	return cmd
}

func createRubric(ctx context.Context, app *cmdutil.App, name, description string, metrics []RubricMetric) (Rubric, error) {
	if len(metrics) == 0 {
		return Rubric{}, cmdutil.UsageError("a rubric needs at least one metric; pass --judge-metric or --computed-metric")
	}
	c, err := app.GQL()
	if err != nil {
		return Rubric{}, err
	}
	in := gql.NewRubric{Name: name, Description: cmdutil.OptString(description)}
	for _, m := range metrics {
		in.Metrics = append(in.Metrics, newRubricMetric(m))
	}
	resp, err := gql.EnterpriseCreateRubric(ctx, c, &in)
	if err != nil {
		return Rubric{}, err
	}
	return rubricFromAPI(resp.CreateRubric), nil
}

func saveRubric(ctx context.Context, app *cmdutil.App, id string, update gql.UpdateRubric) (Rubric, error) {
	c, err := app.GQL()
	if err != nil {
		return Rubric{}, err
	}
	resp, err := gql.EnterpriseModifyRubric(ctx, c, id, &update)
	if err != nil {
		return Rubric{}, err
	}
	return rubricFromAPI(resp.UpdateRubric), nil
}

// parseMetricFlags reads --judge-metric and --computed-metric values in
// REF or REF=WEIGHT form.
func parseMetricFlags(judge, computed []string) ([]RubricMetric, error) {
	var out []RubricMetric
	parse := func(flag, v string) (string, *float64, error) {
		ref, w, ok := strings.Cut(v, "=")
		if ref == "" {
			return "", nil, cmdutil.UsageError("--%s must be NAME or NAME=WEIGHT, got %q", flag, v)
		}
		if !ok {
			return ref, nil, nil
		}
		weight, err := strconv.ParseFloat(w, 64)
		if err != nil {
			return "", nil, cmdutil.UsageError("--%s %q: the weight must be a number", flag, v)
		}
		return ref, &weight, nil
	}
	for _, v := range judge {
		ref, w, err := parse("judge-metric", v)
		if err != nil {
			return nil, err
		}
		out = append(out, RubricMetric{JudgeMetricID: ref, Weight: w})
	}
	for _, v := range computed {
		ref, w, err := parse("computed-metric", v)
		if err != nil {
			return nil, err
		}
		out = append(out, RubricMetric{ComputedMetricKey: ref, Weight: w})
	}
	return out, nil
}

func newRubricCreate(app *cmdutil.App) *cobra.Command {
	var description, file string
	var judge, computed []string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a rubric",
		Long: `Create a rubric from flags or a file. --judge-metric and --computed-metric
take a name (or id, or key) and an optional weight after =. Metrics given as
flags are added to those in the file.`,
		Example: `  partite eval rubric create support-quality --judge-metric Helpfulness=2 --computed-metric latency_ms=1
  partite eval rubric create -f support-quality.yaml`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rubricResource.RequireEnterprise(app); err != nil {
				return err
			}
			var spec Rubric
			if file != "" {
				if err := app.ReadSpec(file, &spec); err != nil {
					return err
				}
			}
			if len(args) == 1 {
				spec.Name = args[0]
			}
			if spec.Name == "" {
				return cmdutil.UsageError("a name is required")
			}
			if cmd.Flags().Changed("description") {
				spec.Description = description
			}
			extra, err := parseMetricFlags(judge, computed)
			if err != nil {
				return err
			}
			want, err := resolveRubricMetrics(cmd.Context(), app, append(spec.Metrics, extra...))
			if err != nil {
				return err
			}
			r, err := createRubric(cmd.Context(), app, spec.Name, spec.Description, want)
			if err != nil {
				return err
			}
			app.Printer().Done("Created rubric %s (%s).", r.Name, r.ID)
			if app.Format() == output.Table {
				return nil
			}
			return rubricResource.Show(app, r)
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "description")
	cmd.Flags().StringArrayVar(&judge, "judge-metric", nil, "judge metric to score, as NAME or NAME=WEIGHT; repeat for more")
	cmd.Flags().StringArrayVar(&computed, "computed-metric", nil, "computed metric to score, as KEY or KEY=WEIGHT; repeat for more")
	cmd.Flags().StringVarP(&file, "file", "f", "", "read the rubric from a YAML or JSON file")
	return cmd
}

func newRubricUpdate(app *cmdutil.App) *cobra.Command {
	var name, description string
	var judge, computed, remove []string
	cmd := &cobra.Command{
		Use:   "update <name-or-id>",
		Short: "Rename a rubric or add, reweight and remove its metrics",
		Long: `Change a rubric. --judge-metric and --computed-metric add a metric, or set
the weight of one the rubric already has. --remove takes a judge metric's name
or id, or a computed metric's key or name. For other settings, use get -o yaml,
edit the file, and apply it.`,
		Example: `  partite eval rubric update support-quality --judge-metric Groundedness=1
  partite eval rubric update support-quality --remove latency_ms`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := rubricResource.Resolve(ctx, app, args[0])
			if err != nil {
				return err
			}
			cur, err := rubricResource.Get(ctx, app, id)
			if err != nil {
				return err
			}
			f := cmd.Flags()
			if !f.Changed("name") && !f.Changed("description") && len(judge) == 0 && len(computed) == 0 && len(remove) == 0 {
				return cmdutil.UsageError("nothing to change; pass --name, --description, --judge-metric, --computed-metric or --remove")
			}
			if !f.Changed("name") {
				name = cur.Name
			}
			if !f.Changed("description") {
				description = cur.Description
			}
			added, err := parseMetricFlags(judge, computed)
			if err != nil {
				return err
			}
			added, err = resolveRubricMetrics(ctx, app, added)
			if err != nil {
				return err
			}
			want, err := editRubricMetrics(cur.Metrics, added, remove)
			if err != nil {
				return err
			}
			if len(want) == 0 {
				return cmdutil.UsageError("a rubric needs at least one metric; to remove the rubric, use: partite eval rubric delete %s", args[0])
			}
			r, err := saveRubric(ctx, app, id, rubricUpdate(name, description, cur.Metrics, want))
			if err != nil {
				return err
			}
			app.Printer().Done("Updated rubric %s.", r.Name)
			if app.Format() == output.Table {
				return nil
			}
			return rubricResource.Show(app, r)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringArrayVar(&judge, "judge-metric", nil, "judge metric to add or reweight, as NAME or NAME=WEIGHT; repeat for more")
	cmd.Flags().StringArrayVar(&computed, "computed-metric", nil, "computed metric to add or reweight, as KEY or KEY=WEIGHT; repeat for more")
	cmd.Flags().StringArrayVar(&remove, "remove", nil, "metric to remove, by name, id or key; repeat for more")
	return cmd
}

// editRubricMetrics applies added and removed metrics to held. An added
// metric the rubric already has keeps its settings and takes the new weight
// when one is given. remove matches ids, keys and names.
func editRubricMetrics(held, added []RubricMetric, remove []string) ([]RubricMetric, error) {
	want := make([]RubricMetric, 0, len(held)+len(added))
	want = append(want, held...)
	for _, a := range added {
		found := false
		for i := range want {
			if want[i].key() == a.key() {
				found = true
				if a.Weight != nil {
					want[i].Weight = a.Weight
				}
			}
		}
		if !found {
			want = append(want, a)
		}
	}
	for _, ref := range remove {
		kept := want[:0]
		matched := false
		for _, m := range want {
			if m.JudgeMetricID == ref || m.ComputedMetricKey == ref || strings.EqualFold(m.MetricName, ref) {
				matched = true
				continue
			}
			kept = append(kept, m)
		}
		if !matched {
			return nil, cmdutil.UsageError("the rubric has no metric %q", ref)
		}
		want = kept
	}
	return want, nil
}
