package eval

import (
	"encoding/json"
	"strings"
	"testing"

	"bytes"

	"github.com/partite-ai/partite-cli/internal/cmdutil"
	gql "github.com/partite-ai/partite-cli/internal/graphql"
	"github.com/partite-ai/partite-cli/internal/output"
)

func TestRubricAgentTagsPatch(t *testing.T) {
	for _, kind := range []string{"computed_metric_key", "judge_metric_id"} {
		for _, tc := range []struct{ name, tags, want string }{
			{"omitted", "", ""},
			{"empty", `,"tags":[]`, `"tags":[]`},
			{"null", `,"tags":null`, `"tags":null`},
			{"values", `,"tags":[{"name":"role","value":"reviewer"}]`, `"tags":[{"name":"role","value":"reviewer"}]`},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				var metric RubricMetric
				if err := cmdutil.DecodeSpec("metric.json", []byte(`{"`+kind+`":"example"`+tc.tags+`}`), &metric); err != nil {
					t.Fatal(err)
				}
				if _, err := checkRubricMetric(metric); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(updateRubricMetric("assignment", metric))
				if err != nil {
					t.Fatal(err)
				}
				if tc.want == "" && strings.Contains(string(data), `"tags"`) || tc.want != "" && !strings.Contains(string(data), tc.want) {
					t.Fatalf("update = %s, want %q", data, tc.want)
				}
			})
		}
	}
}

func TestRubricAgentTagsYAMLAndValidation(t *testing.T) {
	for _, spec := range []string{
		"computed_metric_key: latency\ntags:\n  - name: role\n    value: reviewer\n",
		`{"computed_metric_key":"latency","tags":[{"name":"role","value":"reviewer"}]}`,
	} {
		var metric RubricMetric
		if err := cmdutil.DecodeSpec("metric", []byte(spec), &metric); err != nil {
			t.Fatal(err)
		}
		if _, err := checkRubricMetric(metric); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(newRubricMetric(metric))
		if err != nil || !strings.Contains(string(data), `"tags":[{"name":"role","value":"reviewer"}]`) {
			t.Fatalf("create = %s, %v", data, err)
		}
	}
	for _, raw := range []string{`[null]`, `[{"name":"","value":"x"}]`, `[{"name":"x","value":""}]`, `"invalid"`, `[{"name":"x","value":"y","extra":"z"}]`} {
		var metric RubricMetric
		if err := json.Unmarshal([]byte(`{"computed_metric_key":"latency","tags":`+raw+`}`), &metric); err != nil {
			t.Fatal(err)
		}
		if _, err := checkRubricMetric(metric); err == nil {
			t.Fatalf("accepted invalid tags: %s", raw)
		}
	}
}

func TestRubricAgentTagsFlagUpdatePreservesTags(t *testing.T) {
	var held RubricMetric
	if err := json.Unmarshal([]byte(`{"computed_metric_key":"latency","tags":[{"name":"role","value":"reviewer"}]}`), &held); err != nil {
		t.Fatal(err)
	}
	weight := 2.0
	got, err := editRubricMetrics([]RubricMetric{held}, []RubricMetric{{ComputedMetricKey: "latency", Weight: &weight}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(updateRubricMetric("assignment", got[0]))
	if err != nil || !strings.Contains(string(data), `"tags":[{"name":"role","value":"reviewer"}]`) {
		t.Fatalf("update = %s, %v", data, err)
	}
}

func TestRubricAgentTagsGetYAMLRoundTrip(t *testing.T) {
	var api gql.EnterpriseRubric
	fixture := `{"id":"eurb_1","name":"test","createdAt":"now","updatedAt":"now","metrics":[{"__typename":"RubricJudgeMetric","judgeEntryID":"erum_1","judgeMetric":{"id":"eumt_1","name":"judge"},"trialCount":1,"tags":[{"name":"role","value":"reviewer"}]},{"__typename":"RubricComputedMetric","computedEntryID":"ersm_1","computedMetric":{"key":"latency","name":"Latency"},"tags":[]}]}`
	if err := json.Unmarshal([]byte(fixture), &api); err != nil {
		t.Fatal(err)
	}
	view := rubricFromAPI(api)
	var out bytes.Buffer
	printer := output.Printer{Out: &out, Format: output.YAML}
	if err := printer.Data(view); err != nil {
		t.Fatal(err)
	}
	var roundTrip Rubric
	if err := cmdutil.DecodeSpec("rubric.yaml", out.Bytes(), &roundTrip); err != nil {
		t.Fatalf("decode %s: %v", out.String(), err)
	}
	if len(roundTrip.Metrics) != 2 {
		t.Fatalf("metrics = %+v", roundTrip.Metrics)
	}
	for i, want := range []string{`[{"name":"role","value":"reviewer"}]`, `[]`} {
		if string(roundTrip.Metrics[i].Tags) != want {
			t.Fatalf("metric %d tags = %s, want %s", i, roundTrip.Metrics[i].Tags, want)
		}
		data, err := json.Marshal(updateRubricMetric("assignment", roundTrip.Metrics[i]))
		if err != nil || !strings.Contains(string(data), `"tags":`+want) {
			t.Fatalf("update = %s, %v", data, err)
		}
	}
}
