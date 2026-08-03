package parser

import (
	"context"
	"testing"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/stretchr/testify/require"
)

func TestExploreFieldSelector(t *testing.T) {
	files := map[string]string{
		// rill.yaml
		`rill.yaml`: ``,
		// explore e1
		`explores/e1.yaml`: `
type: explore
metrics_view: mv1
`,
		// explore e2
		`explores/e2.yaml`: `
type: explore
metrics_view: mv1
dimensions: '*'
measures:
  exclude: '*'
`,
		// explore e3
		`explores/e3.yaml`: `
type: explore
metrics_view: mv1
dimensions: foo
measures:
  exclude: bar
`,
		// explore e4
		`explores/e4.yaml`: `
type: explore
metrics_view: mv1
dimensions: [bar, baz]
measures:
  exclude: [foo, qux]
`,
		// explore e5
		`explores/e5.yaml`: `
type: explore
metrics_view: mv1
dimensions:
  regex: 'foo.*'
measures:
  exclude:
    regex: 'bar.*'
`,
	}

	resources := []*Resource{
		// explore e1
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e1"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e1.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E1",
				MetricsView:          "mv1",
				DimensionsSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				MeasuresSelector:     &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				AllowCustomTimeRange: true,
			},
		},
		// explore e2
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e2"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e2.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E2",
				MetricsView:          "mv1",
				DimensionsSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				MeasuresSelector:     &runtimev1.FieldSelector{Invert: true, Selector: &runtimev1.FieldSelector_All{All: true}},
				AllowCustomTimeRange: true,
			},
		},
		// explore e3
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e3"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e3.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E3",
				MetricsView:          "mv1",
				Dimensions:           []string{"foo"},
				MeasuresSelector:     &runtimev1.FieldSelector{Invert: true, Selector: &runtimev1.FieldSelector_Fields{Fields: &runtimev1.StringListValue{Values: []string{"bar"}}}},
				AllowCustomTimeRange: true,
			},
		},
		// explore e4
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e4"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e4.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E4",
				MetricsView:          "mv1",
				Dimensions:           []string{"bar", "baz"},
				MeasuresSelector:     &runtimev1.FieldSelector{Invert: true, Selector: &runtimev1.FieldSelector_Fields{Fields: &runtimev1.StringListValue{Values: []string{"foo", "qux"}}}},
				AllowCustomTimeRange: true,
			},
		},
		// explore e5
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e5"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e5.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E5",
				MetricsView:          "mv1",
				DimensionsSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_Regex{Regex: "foo.*"}},
				MeasuresSelector:     &runtimev1.FieldSelector{Invert: true, Selector: &runtimev1.FieldSelector_Regex{Regex: "bar.*"}},
				AllowCustomTimeRange: true,
			},
		},
	}

	ctx := context.Background()
	repo := makeRepo(t, files)
	p, err := Parse(ctx, repo, "", "", "duckdb", true)
	require.NoError(t, err)
	requireResourcesAndErrors(t, p, resources, nil)
}

func TestExploreRefreshIntervals(t *testing.T) {
	files := map[string]string{
		`rill.yaml`: ``,
		`explores/e1.yaml`: `
type: explore
metrics_view: mv1
refresh_intervals: ['30s', '5m', '1h', '1d']
defaults:
  refresh_interval: 5m
`,
		`explores/e2.yaml`: `
type: explore
metrics_view: mv1
defaults:
  refresh_interval: off
`,
	}

	refreshInterval1 := "5m"
	refreshInterval2 := "off"
	resources := []*Resource{
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e1"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e1.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E1",
				MetricsView:          "mv1",
				DimensionsSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				MeasuresSelector:     &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				RefreshIntervals: []string{"30s", "5m", "1h", "1d"},
				DefaultPreset: &runtimev1.ExplorePreset{
					DimensionsSelector: &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
					MeasuresSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
					RefreshInterval:    &refreshInterval1,
					ComparisonMode:     runtimev1.ExploreComparisonMode_EXPLORE_COMPARISON_MODE_NONE,
				},
				AllowCustomTimeRange: true,
			},
		},
		{
			Name:  ResourceName{Kind: ResourceKindExplore, Name: "e2"},
			Refs:  []ResourceName{{Kind: ResourceKindMetricsView, Name: "mv1"}},
			Paths: []string{"/explores/e2.yaml"},
			ExploreSpec: &runtimev1.ExploreSpec{
				DisplayName:          "E2",
				MetricsView:          "mv1",
				DimensionsSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				MeasuresSelector:     &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
				DefaultPreset: &runtimev1.ExplorePreset{
					DimensionsSelector: &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
					MeasuresSelector:   &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}},
					RefreshInterval:    &refreshInterval2,
					ComparisonMode:     runtimev1.ExploreComparisonMode_EXPLORE_COMPARISON_MODE_NONE,
				},
				AllowCustomTimeRange: true,
			},
		},
	}

	ctx := context.Background()
	repo := makeRepo(t, files)
	p, err := Parse(ctx, repo, "", "", "duckdb", true)
	require.NoError(t, err)
	requireResourcesAndErrors(t, p, resources, nil)

	// Invalid: bad duration
	repo = makeRepo(t, map[string]string{
		`rill.yaml`: ``,
		`explores/e1.yaml`: `
type: explore
metrics_view: mv1
refresh_intervals: ['5x']
`,
	})
	p, err = Parse(ctx, repo, "", "", "duckdb", true)
	require.NoError(t, err)
	require.Len(t, p.Errors, 1)
	require.Contains(t, p.Errors[0].Message, "invalid refresh interval")

	// Invalid: special values in refresh_intervals
	repo = makeRepo(t, map[string]string{
		`rill.yaml`: ``,
		`explores/e1.yaml`: `
type: explore
metrics_view: mv1
refresh_intervals: ['off', '5m']
`,
	})
	p, err = Parse(ctx, repo, "", "", "duckdb", true)
	require.NoError(t, err)
	require.Len(t, p.Errors, 1)
	require.Contains(t, p.Errors[0].Message, "always selectable")
}

func TestExploreLogsView(t *testing.T) {
	files := map[string]string{
		`rill.yaml`: ``,
		`explores/e1.yaml`: `
type: explore
metrics_view: mv1
logs_view: true
logs_view_columns: ['ts', 'body']
`,
	}

	ctx := context.Background()
	repo := makeRepo(t, files)
	p, err := Parse(ctx, repo, "", "", "duckdb", true)
	require.NoError(t, err)
	require.Empty(t, p.Errors)

	e := p.Resources[ResourceName{Kind: ResourceKindExplore, Name: "e1"}.Normalized()]
	require.NotNil(t, e)
	require.True(t, e.ExploreSpec.LogsView)
	require.Equal(t, []string{"ts", "body"}, e.ExploreSpec.LogsViewColumns)
}
