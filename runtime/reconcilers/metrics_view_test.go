package reconcilers_test

import (
	"testing"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime"
	"github.com/rilldata/rill/runtime/testruntime"
	"github.com/stretchr/testify/require"
)

func TestMetricsViewTimeCaseInsensitive(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS TiMe, 1 AS num`,
			"mv1.yaml": `
type: metrics_view
model: m1
timeseries: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv2.yaml": `
type: metrics_view
model: m1
timeseries: TiMe
dimensions:
- column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv3.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- name: time
  column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv4.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv5.yaml": `
type: metrics_view
model: m1
timeseries: TiMe
dimensions:
- name: time
  column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 5, 1, 2)

	r := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv1")
	require.Empty(t, r.Meta.ReconcileError)
	d := r.GetMetricsView().State.ValidSpec.Dimensions[0]
	require.Equal(t, runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, d.Type)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, d.DataType.Code)

	r = testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv2")
	require.Empty(t, r.Meta.ReconcileError)
	d = r.GetMetricsView().State.ValidSpec.Dimensions[0]
	require.Equal(t, runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, d.Type)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, d.DataType.Code)

	r = testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv3")
	require.Empty(t, r.Meta.ReconcileError)
	d = r.GetMetricsView().State.ValidSpec.Dimensions[0]
	require.Equal(t, runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, d.Type)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, d.DataType.Code)

	testruntime.RequireParseErrors(t, rt, id, map[string]string{
		"/mv4.yaml": "does not match the case of time dimension",
		"/mv5.yaml": "does not match the case of time dimension",
	})
}

func TestMetricsViewTimeTypes(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql": `SELECT '2024-01-01'::DATE AS date, '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'foo' AS str, 1 AS num`,
			"mv_none.yaml": `
type: metrics_view
model: m1
dimensions:
- column: time
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_time.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: time
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_date.yaml": `
type: metrics_view
model: m1
timeseries: date
dimensions:
- column: time
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_time_legacy.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_date_legacy.yaml": `
type: metrics_view
model: m1
timeseries: date
dimensions:
- column: time
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 7, 0, 0)

	// Expectations
	cases := []struct {
		metricsView string
		dimension   string
		typ         runtimev1.MetricsViewSpec_DimensionType
		dataTyp     runtimev1.Type_Code
	}{
		{"mv_none", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_none", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL, runtimev1.Type_CODE_DATE},
		{"mv_time", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_time", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL, runtimev1.Type_CODE_DATE},
		{"mv_date", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_date", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_DATE},
		{"mv_time_legacy", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_time_legacy", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL, runtimev1.Type_CODE_DATE},
		{"mv_date_legacy", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_date_legacy", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_DATE},
	}
	for _, c := range cases {
		t.Run(c.metricsView+"_"+c.dimension, func(t *testing.T) {
			ctrl, err := rt.Controller(t.Context(), id)
			require.NoError(t, err)
			mv, err := ctrl.Get(t.Context(), &runtimev1.ResourceName{Kind: runtime.ResourceKindMetricsView, Name: c.metricsView}, false)
			require.NoError(t, err)
			validSpec := mv.GetMetricsView().State.ValidSpec
			require.NotNil(t, validSpec)

			var found bool
			for _, d := range validSpec.Dimensions {
				if d.Name == c.dimension {
					found = true
					require.Equal(t, c.typ, d.Type)
					require.Equal(t, c.dataTyp, d.DataType.Code)
				}
			}
			require.True(t, found, "dimension %s not found in metrics view %s", c.dimension, c.metricsView)
		})
	}
}

func TestMetricsViewSkipInvalidDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, 1 AS num`,
			// Without the flag, a dimension referencing a missing column fails the whole metrics view.
			"mv_strict.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: service
- column: status_code
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			// With the flag, invalid dimensions are excluded from the valid spec with a warning.
			"mv_skip.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_invalid_dimensions: true
dimensions:
- column: service
- column: status_code
- column: log_level
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 4, 1, 0)

	strict := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_strict")
	require.Nil(t, strict.GetMetricsView().State.ValidSpec)
	require.Contains(t, strict.Meta.ReconcileError, "status_code")

	skip := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_skip")
	require.Empty(t, skip.Meta.ReconcileError)
	validSpec := skip.GetMetricsView().State.ValidSpec
	require.NotNil(t, validSpec)
	var dims []string
	for _, d := range validSpec.Dimensions {
		dims = append(dims, d.Name)
	}
	require.Equal(t, []string{"time", "service"}, dims)
	// The full spec still contains all declared dimensions.
	require.Len(t, skip.GetMetricsView().Spec.Dimensions, 4)
	// The skipped dimensions are surfaced as reconcile warnings.
	require.Len(t, skip.Meta.ReconcileWarnings, 2)
	require.Contains(t, skip.Meta.ReconcileWarnings[0], "status_code")
	require.Contains(t, skip.Meta.ReconcileWarnings[1], "log_level")
}

func TestMetricsViewSkipEmptyDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql":      `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, CAST(NULL AS VARCHAR) AS status_code, 1 AS num`,
			"m_empty.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, 1 AS num WHERE 1=0`,
			// With the flag, dimensions whose values are all NULL are hidden from the valid spec with a warning.
			"mv_hide.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_empty_dimensions: true
dimensions:
- column: service
- column: status_code
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			// Without the flag, all-NULL dimensions are kept.
			"mv_keep.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: service
- column: status_code
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			// On a table with no rows, emptiness carries no signal, so nothing is hidden.
			"mv_empty.yaml": `
type: metrics_view
model: m_empty
timeseries: time
skip_empty_dimensions: true
dimensions:
- column: service
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 6, 0, 0)

	dimNames := func(spec *runtimev1.MetricsViewSpec) []string {
		var names []string
		for _, d := range spec.Dimensions {
			names = append(names, d.Name)
		}
		return names
	}

	hide := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_hide")
	require.NotNil(t, hide.GetMetricsView().State.ValidSpec)
	require.Equal(t, []string{"time", "service"}, dimNames(hide.GetMetricsView().State.ValidSpec))
	// The full spec still contains all declared dimensions.
	require.Equal(t, []string{"time", "service", "status_code"}, dimNames(hide.GetMetricsView().Spec))
	// The hidden dimension is surfaced as a reconcile warning.
	require.Len(t, hide.Meta.ReconcileWarnings, 1)
	require.Contains(t, hide.Meta.ReconcileWarnings[0], "status_code")

	keep := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_keep")
	require.Equal(t, []string{"time", "service", "status_code"}, dimNames(keep.GetMetricsView().State.ValidSpec))
	require.Empty(t, keep.Meta.ReconcileWarnings)

	empty := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_empty")
	require.Equal(t, []string{"time", "service"}, dimNames(empty.GetMetricsView().State.ValidSpec))
	require.Empty(t, empty.Meta.ReconcileWarnings)

	// Once the column carries data, the dimension comes back automatically.
	testruntime.PutFiles(t, rt, id, map[string]string{
		"m1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, '200' AS status_code, 1 AS num`,
	})
	testruntime.ReconcileParserAndWait(t, rt, id)
	testruntime.RequireReconcileState(t, rt, id, 6, 0, 0)
	hide = testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_hide")
	require.Equal(t, []string{"time", "service", "status_code"}, dimNames(hide.GetMetricsView().State.ValidSpec))
	require.Empty(t, hide.Meta.ReconcileWarnings)
}

func TestMetricsViewMapDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			// Structured-log-style data: fixed columns plus a map of dynamic attributes.
			// legacy_field is all-NULL, as when a field stops being ingested.
			"m1.sql": `
SELECT * FROM (VALUES
  ('2024-01-01T00:00:00Z'::TIMESTAMP, MAP {'http.method': 'GET', 'level': 'info'}, 'svc-a', NULL::VARCHAR, 1),
  ('2024-01-02T00:00:00Z'::TIMESTAMP, MAP {'http.method': 'POST', 'k8s.pod': 'p1'}, 'svc-b', NULL::VARCHAR, 2),
  ('2024-01-03T00:00:00Z'::TIMESTAMP, MAP {'http.method': 'GET'}, 'svc-a', NULL::VARCHAR, 3)
) t("time", attrs, service, legacy_field, num)`,
			"mv_map.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_empty_dimensions: true
dimensions:
- column: service
- column: legacy_field
- map_column: attrs
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_pattern.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- map_column: attrs
  discover:
    pattern: '^http\.'
    limit: 10
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 4, 0, 0)

	// mv_map: map keys expanded into dimensions, all-NULL legacy_field hidden.
	mvMap := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_map")
	require.Empty(t, mvMap.Meta.ReconcileError)
	validSpec := mvMap.GetMetricsView().State.ValidSpec
	require.NotNil(t, validSpec)
	var names []string
	for _, d := range validSpec.Dimensions {
		names = append(names, d.Name)
	}
	// Discovered keys are ordered by frequency, then alphabetically.
	require.Equal(t, []string{"time", "service", "http_method", "k8s_pod", "level"}, names)
	require.Equal(t, "http.method", validSpec.Dimensions[2].DisplayName)
	require.Equal(t, `"attrs"['http.method']`, validSpec.Dimensions[2].Expression)
	// The declared spec still contains the raw map_column dimension.
	require.Len(t, mvMap.GetMetricsView().Spec.Dimensions, 4)
	require.Equal(t, "attrs", mvMap.GetMetricsView().Spec.Dimensions[3].MapColumn)
	// The all-NULL dimension is surfaced as a warning.
	require.Len(t, mvMap.Meta.ReconcileWarnings, 1)
	require.Contains(t, mvMap.Meta.ReconcileWarnings[0], "legacy_field")

	// mv_pattern: only keys matching the pattern are expanded.
	mvPattern := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_pattern")
	require.Empty(t, mvPattern.Meta.ReconcileError)
	names = nil
	for _, d := range mvPattern.GetMetricsView().State.ValidSpec.Dimensions {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"time", "http_method"}, names)
}
