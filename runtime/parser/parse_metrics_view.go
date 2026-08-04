package parser

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime/pkg/duration"
	"github.com/rilldata/rill/runtime/pkg/rilltime"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/yaml.v3"

	// Load IANA time zone data
	_ "time/tzdata"
)

// maxMapDimensionDiscoverLimit bounds how many keys a map_column dimension may discover.
const maxMapDimensionDiscoverLimit = 500

// tableOptionSanitizeRegex matches characters that are unsafe in resource names derived from table names.
var tableOptionSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// variantMetricsViewName returns the resource name of the metrics view variant backed by the given table option.
func variantMetricsViewName(name, table string) string {
	return fmt.Sprintf("%s__%s", name, tableOptionSanitizeRegex.ReplaceAllString(table, "_"))
}

// MetricsViewYAML is the raw structure of a MetricsView resource defined in YAML
type MetricsViewYAML struct {
	commonYAML            `yaml:",inline"` // Not accessed here, only setting it so we can use KnownFields for YAML parsing
	Parent                string           `yaml:"parent"` // Parent metrics view, if any
	DisplayName           string           `yaml:"display_name"`
	Title                 string           `yaml:"title"` // Deprecated: use display_name
	Description           string           `yaml:"description"`
	AIInstructions        string           `yaml:"ai_instructions"`
	Model                 string           `yaml:"model"`
	Database              string           `yaml:"database"`
	DatabaseSchema        string           `yaml:"database_schema"`
	Table                 string           `yaml:"table"`
	TimeDimension         string           `yaml:"timeseries"`
	Watermark             string           `yaml:"watermark"`
	SmallestTimeGrain     string           `yaml:"smallest_time_grain"`
	FirstDayOfWeek        uint32           `yaml:"first_day_of_week"`
	FirstMonthOfYear      uint32           `yaml:"first_month_of_year"`
	MaxQueryTimeRange     string           `yaml:"max_query_time_range"`
	SkipInvalidDimensions bool             `yaml:"skip_invalid_dimensions"`
	SkipEmptyDimensions   bool             `yaml:"skip_empty_dimensions"`
	TableOptions          []string         `yaml:"table_options"`
	RowLinks              []*struct {
		Label string `yaml:"label"`
		URL   string `yaml:"url"`
	} `yaml:"row_links"`
	Dimensions []*struct {
		Name        string
		DisplayName string `yaml:"display_name"`
		Label       string // Deprecated: use display_name
		Description string
		Type        string
		Column      string
		Expression  string
		Property    string // For backwards compatibility
		Ignore      bool   `yaml:"ignore"` // Deprecated
		Unnest      bool
		URI         string
		Links       []*struct {
			Label   string `yaml:"label"`
			URL     string `yaml:"url"`
			Explore string `yaml:"explore"`
		} `yaml:"links"`
		MapColumn string `yaml:"map_column"`
		Columns   string `yaml:"columns"`
		Discover  *struct {
			Limit   uint32 `yaml:"limit"`
			Pattern string `yaml:"pattern"`
		} `yaml:"discover"`
		LookupTable             string `yaml:"lookup_table"`
		LookupKeyColumn         string `yaml:"lookup_key_column"`
		LookupValueColumn       string `yaml:"lookup_value_column"`
		LookupDefaultExpression string `yaml:"lookup_default_expression"`
		SmallestTimeGrain       string `yaml:"smallest_time_grain"`
		Tags                    []string
	}
	Measures []*struct {
		Name                string
		DisplayName         string `yaml:"display_name"`
		Label               string // Deprecated: use display_name
		Description         string
		Type                string
		Expression          string
		Window              *MetricsViewMeasureWindow
		Per                 MetricsViewFieldSelectorsYAML
		Requires            MetricsViewFieldSelectorsYAML
		FormatPreset        string         `yaml:"format_preset"`
		FormatD3            string         `yaml:"format_d3"`
		FormatD3Locale      map[string]any `yaml:"format_d3_locale"`
		Ignore              bool           `yaml:"ignore"` // Deprecated
		ValidPercentOfTotal bool           `yaml:"valid_percent_of_total"`
		TreatNullsAs        string         `yaml:"treat_nulls_as"`
		LowerIsBetter       bool           `yaml:"lower_is_better"`
		Thresholds          *MetricsViewMeasureThresholds
		Unit                string
		Kind                string
		Temporality         string
		Column              string
		Tags                []string
	}
	ParentDimensions *FieldSelectorYAML `yaml:"parent_dimensions"` // used when Parent is set
	ParentMeasures   *FieldSelectorYAML `yaml:"parent_measures"`   // used when Parent is set
	Annotations      []*struct {
		Name           string             `yaml:"name"`
		Model          string             `yaml:"model"`
		Database       string             `yaml:"database"`
		DatabaseSchema string             `yaml:"database_schema"`
		Table          string             `yaml:"table"`
		Connector      string             `yaml:"connector"`
		Measures       *FieldSelectorYAML `yaml:"measures"`
	} `yaml:"annotations"`
	Rollups []*struct {
		Model          string             `yaml:"model"`
		Database       string             `yaml:"database"`
		DatabaseSchema string             `yaml:"database_schema"`
		TimeGrain      string             `yaml:"time_grain"`
		TimeZone       string             `yaml:"time_zone"`
		Dimensions     *FieldSelectorYAML `yaml:"dimensions"`
		Measures       *FieldSelectorYAML `yaml:"measures"`
		DataTimeRange  string             `yaml:"data_time_range"`
	} `yaml:"rollups"`
	DataTimeRange   string `yaml:"data_time_range"`
	Security        *SecurityPolicyYAML
	QueryAttributes map[string]string `yaml:"query_attributes"`
	Cache           struct {
		Enabled       *bool  `yaml:"enabled"`
		KeySQL        string `yaml:"key_sql"`
		KeyTTL        string `yaml:"key_ttl"`
		TimestampsTTL string `yaml:"timestamps_ttl"` // fallback TTL for timestamp caching when MV-level cache is disabled; defaults to 5m
	} `yaml:"cache"`
	Explore *struct {
		ExploreDefinitionYAML `yaml:",inline"`
		Skip                  bool   `yaml:"skip"`
		Name                  string `yaml:"name"` // Name of the explore, defaults to the metrics view name
	} `yaml:"explore"`

	// DEPRECATED FIELDS
	DefaultTimeRange   string   `yaml:"default_time_range"`
	AvailableTimeZones []string `yaml:"available_time_zones"`
	DefaultTheme       string   `yaml:"default_theme"`
	DefaultDimensions  []string `yaml:"default_dimensions"`
	DefaultMeasures    []string `yaml:"default_measures"`
	DefaultComparison  struct {
		Mode      string `yaml:"mode"`
		Dimension string `yaml:"dimension"`
	} `yaml:"default_comparison"`
	AvailableTimeRanges []ExploreTimeRangeYAML `yaml:"available_time_ranges"`
}

type MetricsViewFieldSelectorYAML struct {
	Name       string
	TimeGrain  runtimev1.TimeGrain // Only for time dimensions
	Descending bool                // Only for sorting
}

func (f *MetricsViewFieldSelectorYAML) UnmarshalYAML(v *yaml.Node) error {
	if v == nil {
		return nil
	}

	switch v.Kind {
	case yaml.ScalarNode:
		f.Name = v.Value
	case yaml.MappingNode:
		// avoid infinite loop by using a separate struct
		tmp := &struct {
			Name      string
			TimeGrain string `yaml:"time_grain"`
		}{}
		err := v.Decode(tmp)
		if err != nil {
			return err
		}

		tg, err := parseTimeGrain(tmp.TimeGrain)
		if err != nil {
			return fmt.Errorf(`invalid "time_grain": %w`, err)
		}

		f.Name = tmp.Name
		f.TimeGrain = tg
	default:
		return fmt.Errorf("field reference should be either a string or an object")
	}

	return nil
}

type MetricsViewFieldSelectorsYAML []MetricsViewFieldSelectorYAML

func (f *MetricsViewFieldSelectorsYAML) UnmarshalYAML(v *yaml.Node) error {
	if v == nil {
		return nil
	}

	switch v.Kind {
	case yaml.ScalarNode:
		*f = []MetricsViewFieldSelectorYAML{{Name: v.Value}}
	case yaml.SequenceNode:
		res := make([]MetricsViewFieldSelectorYAML, len(v.Content))
		for i, n := range v.Content {
			var tmp MetricsViewFieldSelectorYAML
			err := n.Decode(&tmp)
			if err != nil {
				return err
			}
			res[i] = tmp
		}
		*f = res
	default:
		return fmt.Errorf("field references should be a name or a list")
	}

	return nil
}

type MetricsViewMeasureWindow struct {
	Partition bool
	Order     []MetricsViewFieldSelectorYAML
	OrderTime bool // Preset for ordering by only the time dimension
	Frame     string
}

func (f *MetricsViewMeasureWindow) UnmarshalYAML(v *yaml.Node) error {
	if v == nil {
		return nil
	}

	switch v.Kind {
	case yaml.ScalarNode:
		switch strings.ToLower(v.Value) {
		case "time", "true":
			f.Partition = true
			f.OrderTime = true
		case "all":
			f.Partition = false
		default:
			return fmt.Errorf(`invalid window type %q`, v.Value)
		}
	case yaml.MappingNode:
		// Avoid infinite loop by using a separate struct
		tmp := &struct {
			Partition *bool
			Order     *MetricsViewFieldSelectorsYAML
			Frame     string
		}{}
		err := v.Decode(tmp)
		if err != nil {
			return err
		}

		// Let partition default to true
		f.Partition = true
		if tmp.Partition != nil {
			f.Partition = *tmp.Partition
		}

		if tmp.Order != nil {
			f.Order = *tmp.Order
		} else {
			// If order is not specified, default to ordering by time if it's a partitioned window
			f.OrderTime = f.Partition
		}

		f.Frame = tmp.Frame
	default:
		return fmt.Errorf("measure window should be either a string or an object")
	}

	return nil
}

// MetricsViewMeasureThresholds is the raw YAML structure of severity thresholds on a measure.
// It accepts either a plain list of steps (compared as "at or above"),
// or an object with "below: true" and "steps" for measures where low values are bad (e.g. free disk space).
type MetricsViewMeasureThresholds struct {
	Below bool
	Steps []MetricsViewMeasureThresholdStep
}

func (f *MetricsViewMeasureThresholds) UnmarshalYAML(v *yaml.Node) error {
	if v == nil {
		return nil
	}

	switch v.Kind {
	case yaml.SequenceNode:
		return v.Decode(&f.Steps)
	case yaml.MappingNode:
		// Avoid infinite loop by using a separate struct
		tmp := &struct {
			Below bool
			Steps []MetricsViewMeasureThresholdStep
		}{}
		err := v.Decode(tmp)
		if err != nil {
			return err
		}
		f.Below = tmp.Below
		f.Steps = tmp.Steps
	default:
		return fmt.Errorf("measure thresholds should be either a list of steps or an object with \"below\" and \"steps\"")
	}

	return nil
}

// MetricsViewMeasureThresholdStep is one step in a measure's thresholds.
// It accepts the compact form `warn: 10` / `critical: 60`, or the explicit form `{value: 10, level: warn}`.
type MetricsViewMeasureThresholdStep struct {
	Value float64
	Level string
}

func (f *MetricsViewMeasureThresholdStep) UnmarshalYAML(v *yaml.Node) error {
	if v == nil {
		return nil
	}
	if v.Kind != yaml.MappingNode {
		return fmt.Errorf("threshold step should be an object")
	}

	tmp := map[string]yaml.Node{}
	err := v.Decode(&tmp)
	if err != nil {
		return err
	}

	// Explicit form: {value: ..., level: ...}
	if levelNode, ok := tmp["level"]; ok {
		valueNode, ok := tmp["value"]
		if !ok {
			return fmt.Errorf(`threshold step with "level" must also set "value"`)
		}
		if err := levelNode.Decode(&f.Level); err != nil {
			return err
		}
		return valueNode.Decode(&f.Value)
	}

	// Compact form: {warn: ...} or {critical: ...}
	if len(tmp) != 1 {
		return fmt.Errorf(`threshold step should be a single "<level>: <value>" pair or an object with "value" and "level"`)
	}
	for level, valueNode := range tmp {
		f.Level = level
		if err := valueNode.Decode(&f.Value); err != nil {
			return err
		}
	}

	return nil
}

var validThresholdLevels = map[string]bool{"warn": true, "critical": true}

// safeSQLName quotes an identifier for use in generated SQL.
func safeSQLName(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}

var comparisonModesMap = map[string]runtimev1.ExploreComparisonMode{
	"":          runtimev1.ExploreComparisonMode_EXPLORE_COMPARISON_MODE_UNSPECIFIED,
	"none":      runtimev1.ExploreComparisonMode_EXPLORE_COMPARISON_MODE_NONE,
	"time":      runtimev1.ExploreComparisonMode_EXPLORE_COMPARISON_MODE_TIME,
	"dimension": runtimev1.ExploreComparisonMode_EXPLORE_COMPARISON_MODE_DIMENSION,
}

var validComparisonModes = []string{"none", "time", "dimension"}

const (
	nameIsMeasure   uint8 = 1
	nameIsDimension uint8 = 2
)

// parseMetricsView parses a metrics view definition and adds the resulting resource to p.Resources.
func (p *Parser) parseMetricsView(node *Node) error {
	// Parse YAML
	tmp := &MetricsViewYAML{}
	err := p.decodeNodeYAML(node, true, tmp)
	if err != nil {
		return err
	}

	// Backwards compatibility
	if tmp.Title != "" && tmp.DisplayName == "" {
		tmp.DisplayName = tmp.Title
	}

	if len(tmp.TableOptions) > 0 {
		if tmp.Table == "" {
			return errors.New(`"table_options" requires "table" to be set to the default table`)
		}
		for _, t := range tmp.TableOptions {
			if t == "" {
				return errors.New(`"table_options" entries must be non-empty table names`)
			}
		}
	}

	if tmp.Table != "" && tmp.Model != "" {
		return fmt.Errorf(`cannot set both the "model" field and the "table" field`)
	}
	if tmp.Table == "" && tmp.Model == "" && tmp.Parent == "" {
		return fmt.Errorf(`must set a value for either the "model", "table" or "parent" field`)
	}

	smallestTimeGrain, err := parseTimeGrain(tmp.SmallestTimeGrain)
	if err != nil {
		return fmt.Errorf(`invalid "smallest_time_grain": %w`, err)
	}
	if smallestTimeGrain != runtimev1.TimeGrain_TIME_GRAIN_UNSPECIFIED && smallestTimeGrain < runtimev1.TimeGrain_TIME_GRAIN_SECOND {
		return errors.New(`"smallest_time_grain" must be at least "second"`)
	}

	if tmp.DefaultTimeRange != "" {
		_, err := rilltime.Parse(tmp.DefaultTimeRange, rilltime.ParseOptions{})
		if err != nil {
			return fmt.Errorf(`invalid "default_time_range": %w`, err)
		}
	}

	if tmp.DataTimeRange != "" {
		if err := validateDataTimeRange(tmp.DataTimeRange); err != nil {
			return fmt.Errorf(`invalid "data_time_range": %w`, err)
		}
	}

	for _, tz := range tmp.AvailableTimeZones {
		_, err := time.LoadLocation(tz)
		if err != nil {
			return err
		}
	}

	if err := validateQueryAttributes(tmp.QueryAttributes); err != nil {
		return fmt.Errorf("invalid query_attributes: %w", err)
	}

	if tmp.Parent != "" {
		if len(tmp.Dimensions) > 0 || len(tmp.Measures) > 0 {
			return fmt.Errorf("cannot define dimensions or measures in a derived metrics view, use parent_dimensions and parent_measures to select from parent %q", tmp.Parent)
		}
		if tmp.Database != "" || tmp.DatabaseSchema != "" || tmp.Table != "" || tmp.Model != "" {
			return fmt.Errorf("cannot set data source in a derived metrics view (parent %q)", tmp.Parent)
		}
		if tmp.Cache.Enabled != nil || tmp.Cache.KeySQL != "" || tmp.Cache.KeyTTL != "" {
			return fmt.Errorf("cannot set cache in a derived metrics view (parent %q)", tmp.Parent)
		}
		// disallow deprecated fields in derived metrics views
		if tmp.DefaultTimeRange != "" || tmp.DefaultTheme != "" || len(tmp.DefaultDimensions) > 0 || len(tmp.DefaultMeasures) > 0 || tmp.DefaultComparison.Mode != "" || tmp.DefaultComparison.Dimension != "" {
			return fmt.Errorf("cannot set defaults in derived metrics view (parent %q), defaults can be set under explore key", tmp.Parent)
		}
		if len(tmp.AvailableTimeRanges) > 0 || len(tmp.AvailableTimeZones) > 0 {
			return fmt.Errorf("cannot set available time ranges or time zones in derived metrics view (parent %q), use explore key", tmp.Parent)
		}

		node.Refs = append(node.Refs, ResourceName{Kind: ResourceKindMetricsView, Name: tmp.Parent})
	} else if tmp.ParentDimensions != nil || tmp.ParentMeasures != nil {
		return fmt.Errorf("parent_dimensions and parent_measures can only be set in derived metrics views, use dimensions and measures instead")
	}

	names := make(map[string]uint8)
	names[strings.ToLower(tmp.TimeDimension)] = nameIsDimension
	timeDimSeenInDimList := false

	dimensions := make([]*runtimev1.MetricsViewSpec_Dimension, 0, len(tmp.Dimensions))
	for i, dim := range tmp.Dimensions {
		if dim == nil || dim.Ignore {
			continue
		}

		// Backwards compatibility
		if dim.Property != "" && dim.Column == "" {
			dim.Column = dim.Property
		}

		// Backwards compatibility
		if dim.Name == "" {
			switch {
			case dim.Column != "":
				dim.Name = dim.Column
			case dim.MapColumn != "":
				dim.Name = dim.MapColumn
			case dim.Columns != "":
				dim.Name = fmt.Sprintf("columns_%d", i)
			default:
				dim.Name = fmt.Sprintf("dimension_%d", i)
			}
		}

		// Backwards compatibility
		if dim.Label != "" && dim.DisplayName == "" {
			dim.DisplayName = dim.Label
		}

		// When display name is not provided, we derive a human-friendly one from the dimension name
		if dim.DisplayName == "" {
			dim.DisplayName = ToDisplayName(dim.Name)
		}

		// The "column", "expression", "map_column" and "columns" properties are mutually exclusive
		if dim.Columns != "" && dim.Columns != "*" {
			return fmt.Errorf(`invalid columns %q for dimension %q: only "*" is supported (use discover.pattern to filter)`, dim.Columns, dim.Name)
		}
		if dim.MapColumn != "" || dim.Columns != "" {
			if dim.MapColumn != "" && dim.Columns != "" {
				return fmt.Errorf("map_column and columns cannot be combined for dimension: %q", dim.Name)
			}
			if dim.Column != "" || dim.Expression != "" {
				return fmt.Errorf("map_column or columns cannot be combined with column or expression for dimension: %q", dim.Name)
			}
			if dim.Unnest || dim.URI != "" || dim.LookupTable != "" {
				return fmt.Errorf("map_column or columns cannot be combined with unnest, uri or lookup fields for dimension: %q", dim.Name)
			}
			if dim.Discover != nil && dim.Discover.Pattern != "" {
				if _, err := regexp.Compile(dim.Discover.Pattern); err != nil {
					return fmt.Errorf("invalid discover pattern for dimension %q: %w", dim.Name, err)
				}
			}
			if dim.Discover != nil && dim.Discover.Limit > maxMapDimensionDiscoverLimit {
				return fmt.Errorf("discover limit for dimension %q may not exceed %d", dim.Name, maxMapDimensionDiscoverLimit)
			}
		} else if dim.Discover != nil {
			return fmt.Errorf("discover can only be set for map_column or columns dimensions: %q", dim.Name)
		} else if (dim.Column == "" && dim.Expression == "") || (dim.Column != "" && dim.Expression != "") {
			return fmt.Errorf("exactly one of column or expression should be set for dimension: %q", dim.Name)
		}

		// Validate the lookup table fields
		if dim.LookupTable != "" || dim.LookupKeyColumn != "" || dim.LookupValueColumn != "" {
			if dim.LookupTable == "" || dim.LookupKeyColumn == "" || dim.LookupValueColumn == "" {
				return fmt.Errorf("all lookup fields should be defined (lookup_table, lookup_key_column and lookup_value_column should be defined")
			}
			if strings.Contains(dim.Expression, "dictGet") {
				return fmt.Errorf("dictGet expression and lookup fields cannot be used together")
			}
			if dim.Unnest {
				return fmt.Errorf("unnest cannot be used with lookup fields")
			}
		}

		// Validate the dimension name is unique
		lower := strings.ToLower(dim.Name)
		if _, ok := names[lower]; ok {
			// allow time dimension to be defined in the dimensions list once
			if strings.EqualFold(lower, tmp.TimeDimension) {
				if timeDimSeenInDimList {
					return fmt.Errorf("time dimension %q defined multiple times", tmp.TimeDimension)
				} else if dim.Name != tmp.TimeDimension {
					return fmt.Errorf("dimension name %q does not match the case of time dimension %q", dim.Name, tmp.TimeDimension)
				}
				timeDimSeenInDimList = true
			} else {
				return fmt.Errorf("found duplicate dimension or measure name %q", dim.Name)
			}
		}
		names[lower] = nameIsDimension

		smallestTimeGrain, err := parseTimeGrain(dim.SmallestTimeGrain)
		if err != nil {
			return fmt.Errorf(`invalid "smallest_time_grain" for dimension %q: %w`, dim.Name, err)
		}
		if smallestTimeGrain != runtimev1.TimeGrain_TIME_GRAIN_UNSPECIFIED && smallestTimeGrain < runtimev1.TimeGrain_TIME_GRAIN_SECOND {
			return fmt.Errorf(`invalid "smallest_time_grain" for dimension %q: must be at least "second"`, dim.Name)
		}

		var typ runtimev1.MetricsViewSpec_DimensionType
		switch strings.ToLower(dim.Type) {
		case "":
			// Leave unspecified as default
		case "geo":
			typ = runtimev1.MetricsViewSpec_DIMENSION_TYPE_GEOSPATIAL
		case "time":
			typ = runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME
		case "categorical":
			typ = runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL
		default:
			return fmt.Errorf(`invalid dimension type %q (allowed values: geo, time, categorical)`, dim.Type)
		}

		// Dimension is valid, add to the list
		d := &runtimev1.MetricsViewSpec_Dimension{
			Name:                    dim.Name,
			DisplayName:             dim.DisplayName,
			Description:             dim.Description,
			Column:                  dim.Column,
			Expression:              dim.Expression,
			Type:                    typ,
			Unnest:                  dim.Unnest,
			Uri:                     dim.URI,
			MapColumn:               dim.MapColumn,
			AllColumns:              dim.Columns == "*",
			LookupTable:             dim.LookupTable,
			LookupKeyColumn:         dim.LookupKeyColumn,
			LookupValueColumn:       dim.LookupValueColumn,
			LookupDefaultExpression: dim.LookupDefaultExpression,
			SmallestTimeGrain:       smallestTimeGrain,
			Tags:                    dim.Tags,
		}
		if dim.Discover != nil {
			d.DiscoverLimit = dim.Discover.Limit
			d.DiscoverPattern = dim.Discover.Pattern
		}
		for _, link := range dim.Links {
			if link == nil {
				continue
			}
			if (link.URL == "") == (link.Explore == "") {
				return fmt.Errorf(`invalid link for dimension %q: exactly one of "url" and "explore" must be set`, dim.Name)
			}
			label := link.Label
			if link.URL != "" {
				if err := validateDimensionLink(link.URL); err != nil {
					return fmt.Errorf(`invalid link for dimension %q: %w`, dim.Name, err)
				}
				if label == "" {
					if u, err := url.Parse(link.URL); err == nil {
						label = u.Host
					}
				}
			} else if label == "" {
				label = link.Explore
			}
			d.Links = append(d.Links, &runtimev1.MetricsViewSpec_Dimension_ValueLink{
				Label:   label,
				Url:     link.URL,
				Explore: link.Explore,
			})
		}
		dimensions = append(dimensions, d)
	}

	for _, dimension := range tmp.DefaultDimensions {
		if v, ok := names[strings.ToLower(dimension)]; !ok || v != nameIsDimension {
			return fmt.Errorf(`dimension %q referenced in "default_dimensions" not found`, dimension)
		}
	}

	measures := make([]*runtimev1.MetricsViewSpec_Measure, 0, len(tmp.Measures))
	// Columns of cumulative counter measures, which need reset-safe per-series delta normalization in a generated model.
	counterColumns := make(map[string]bool)
	for i, measure := range tmp.Measures {
		if measure == nil || measure.Ignore {
			continue
		}

		// Backwards compatibility
		if measure.Name == "" {
			measure.Name = fmt.Sprintf("measure_%d", i)
		}

		// Backwards compatibility
		if measure.Label != "" && measure.DisplayName == "" {
			measure.DisplayName = measure.Label
		}

		if measure.DisplayName == "" {
			measure.DisplayName = ToDisplayName(measure.Name)
		}

		lower := strings.ToLower(measure.Name)
		if _, ok := names[lower]; ok {
			return fmt.Errorf("found duplicate dimension or measure name %q", measure.Name)
		}
		names[lower] = nameIsMeasure

		if measure.FormatPreset != "" && measure.FormatD3 != "" {
			return fmt.Errorf(`cannot set both "format_preset" and "format_d3" for a measure`)
		}

		var formatD3Locale *structpb.Struct
		if measure.FormatD3Locale != nil {
			if measure.FormatD3 == "" {
				return fmt.Errorf(`"format_d3_locale" can only be set if "format_d3" is set`)
			}

			formatD3Locale, err = structpb.NewStruct(measure.FormatD3Locale)
			if err != nil {
				return fmt.Errorf(`invalid "format_d3_locale": %w`, err)
			}
		}

		var perDimensions []*runtimev1.MetricsViewSpec_DimensionSelector
		for _, per := range measure.Per {
			typ, ok := names[strings.ToLower(per.Name)]
			if !ok || typ != nameIsDimension {
				return fmt.Errorf(`per dimension %q not found`, per.Name)
			}
			perDimensions = append(perDimensions, &runtimev1.MetricsViewSpec_DimensionSelector{
				Name:      per.Name,
				TimeGrain: per.TimeGrain,
			})
		}

		var requiredDimensions []*runtimev1.MetricsViewSpec_DimensionSelector
		var referencedMeasures []string
		for _, ref := range measure.Requires {
			typ, ok := names[strings.ToLower(ref.Name)]

			// All dimensions have already been parsed, so we know for sure if it's a dimension
			if ok && typ == nameIsDimension {
				requiredDimensions = append(requiredDimensions, &runtimev1.MetricsViewSpec_DimensionSelector{
					Name:      ref.Name,
					TimeGrain: ref.TimeGrain,
				})
				continue
			}

			// If not a dimension, we assume it's a measure and validate after the loop (when all measures have been seen)
			referencedMeasures = append(referencedMeasures, ref.Name)
		}

		var window *runtimev1.MetricsViewSpec_MeasureWindow
		if measure.Window != nil {
			// Build order list
			var order []*runtimev1.MetricsViewSpec_DimensionSelector
			if measure.Window.OrderTime && tmp.TimeDimension != "" {
				order = append(order, &runtimev1.MetricsViewSpec_DimensionSelector{
					Name: tmp.TimeDimension,
				})
			}
			for _, o := range measure.Window.Order {
				typ, ok := names[strings.ToLower(o.Name)]
				if !ok || typ != nameIsDimension {
					return fmt.Errorf(`order dimension %q not found`, o.Name)
				}

				order = append(order, &runtimev1.MetricsViewSpec_DimensionSelector{
					Name:      o.Name,
					TimeGrain: o.TimeGrain,
					Desc:      o.Descending,
				})
			}

			// Add items in order list to requiredDimensions
			for _, o := range order {
				found := false
				for _, rd := range requiredDimensions {
					if strings.EqualFold(rd.Name, o.Name) {
						found = true
						break
					}
				}
				if !found {
					requiredDimensions = append(requiredDimensions, &runtimev1.MetricsViewSpec_DimensionSelector{
						Name:      o.Name,
						TimeGrain: o.TimeGrain,
					})
				}
			}

			// Build window
			window = &runtimev1.MetricsViewSpec_MeasureWindow{
				Partition:       measure.Window.Partition,
				OrderBy:         order,
				FrameExpression: measure.Window.Frame,
			}
		}

		var typ runtimev1.MetricsViewSpec_MeasureType
		switch strings.ToLower(measure.Type) {
		case "":
			typ = runtimev1.MetricsViewSpec_MEASURE_TYPE_SIMPLE
			if len(referencedMeasures) > 0 || len(perDimensions) > 0 {
				typ = runtimev1.MetricsViewSpec_MEASURE_TYPE_DERIVED
			}
		case "simple":
			typ = runtimev1.MetricsViewSpec_MEASURE_TYPE_SIMPLE
			if len(referencedMeasures) > 0 || len(perDimensions) > 0 {
				return fmt.Errorf(`measure type "simple" cannot have "per" or "requires" fields`)
			}
		case "derived":
			typ = runtimev1.MetricsViewSpec_MEASURE_TYPE_DERIVED
		case "time_comparison":
			typ = runtimev1.MetricsViewSpec_MEASURE_TYPE_TIME_COMPARISON
		default:
			return fmt.Errorf(`invalid measure type %q (allowed values: simple, derived, time_comparison)`, measure.Type)
		}

		if measure.Unit != "" && measure.Unit != "per_second" {
			return fmt.Errorf(`measure %q: invalid unit %q (allowed values: per_second)`, measure.Name, measure.Unit)
		}

		// Kind-based measures: validate the declaration and derive a default expression from the column.
		// For cumulative counters, the columns are collected and normalized in a generated model (see below).
		switch measure.Kind {
		case "":
			if measure.Temporality != "" {
				return fmt.Errorf(`measure %q: "temporality" can only be set when "kind" is "counter"`, measure.Name)
			}
			if measure.Column != "" {
				return fmt.Errorf(`measure %q: "column" can only be set when "kind" is set`, measure.Name)
			}
		case "gauge":
			if measure.Temporality != "" {
				return fmt.Errorf(`measure %q: "temporality" can only be set when "kind" is "counter"`, measure.Name)
			}
			if measure.Column == "" && measure.Expression == "" {
				return fmt.Errorf(`measure %q: gauge measures require "column" or "expression"`, measure.Name)
			}
			// Gauges re-aggregate with avg by default.
			if measure.Expression == "" {
				measure.Expression = fmt.Sprintf("avg(%s)", safeSQLName(measure.Column))
			}
		case "counter":
			switch measure.Temporality {
			case "":
				measure.Temporality = "cumulative" // The Prometheus default
			case "delta", "cumulative":
				// Valid
			default:
				return fmt.Errorf(`measure %q: invalid temporality %q (allowed values: delta, cumulative)`, measure.Name, measure.Temporality)
			}
			if measure.Column == "" {
				return fmt.Errorf(`measure %q: counter measures require "column" (the cumulative or delta value column)`, measure.Name)
			}
			// Counters re-aggregate with sum; for cumulative counters this is correct because
			// the generated model rewrites the column to reset-safe per-series increases.
			if measure.Expression == "" {
				measure.Expression = fmt.Sprintf("sum(%s)", safeSQLName(measure.Column))
			}
			if measure.Temporality == "cumulative" {
				counterColumns[measure.Column] = true
			}
		default:
			return fmt.Errorf(`measure %q: invalid kind %q (allowed values: gauge, counter)`, measure.Name, measure.Kind)
		}

		var thresholds *runtimev1.MetricsViewSpec_MeasureThresholds
		if measure.Thresholds != nil {
			if len(measure.Thresholds.Steps) == 0 {
				return fmt.Errorf(`measure %q: "thresholds" must have at least one step`, measure.Name)
			}
			thresholds = &runtimev1.MetricsViewSpec_MeasureThresholds{Below: measure.Thresholds.Below}
			for i, step := range measure.Thresholds.Steps {
				if !validThresholdLevels[step.Level] {
					return fmt.Errorf(`measure %q: invalid threshold level %q (allowed values: warn, critical)`, measure.Name, step.Level)
				}
				// Steps must escalate: each step's value must be further in the comparison direction than the previous one.
				if i > 0 {
					prev := measure.Thresholds.Steps[i-1].Value
					if measure.Thresholds.Below && step.Value >= prev {
						return fmt.Errorf(`measure %q: threshold step values must be decreasing when "below" is set`, measure.Name)
					}
					if !measure.Thresholds.Below && step.Value <= prev {
						return fmt.Errorf(`measure %q: threshold step values must be increasing`, measure.Name)
					}
				}
				thresholds.Steps = append(thresholds.Steps, &runtimev1.MetricsViewSpec_MeasureThresholds_Step{
					Value: step.Value,
					Level: step.Level,
				})
			}
		}

		measures = append(measures, &runtimev1.MetricsViewSpec_Measure{
			Name:                measure.Name,
			DisplayName:         measure.DisplayName,
			Description:         measure.Description,
			Expression:          measure.Expression,
			Type:                typ,
			Window:              window,
			PerDimensions:       perDimensions,
			RequiredDimensions:  requiredDimensions,
			ReferencedMeasures:  referencedMeasures,
			FormatPreset:        measure.FormatPreset,
			FormatD3:            measure.FormatD3,
			FormatD3Locale:      formatD3Locale,
			ValidPercentOfTotal: measure.ValidPercentOfTotal,
			TreatNullsAs:        measure.TreatNullsAs,
			LowerIsBetter:       measure.LowerIsBetter,
			Thresholds:          thresholds,
			Unit:                measure.Unit,
			Kind:                measure.Kind,
			Temporality:         measure.Temporality,
			Column:              measure.Column,
			Tags:                measure.Tags,
		})
	}
	if len(measures) == 0 && tmp.Parent == "" {
		return fmt.Errorf("must define at least one measure")
	}

	// Validate referenced measures now that all measures have been seen
	for _, m := range measures {
		for _, ref := range m.ReferencedMeasures {
			if typ, ok := names[strings.ToLower(ref)]; !ok || typ != nameIsMeasure {
				return fmt.Errorf(`referenced measure %q not found`, ref)
			}
		}
	}

	for _, measure := range tmp.DefaultMeasures {
		if v, ok := names[strings.ToLower(measure)]; !ok || v != nameIsMeasure {
			return fmt.Errorf(`measure %q referenced in "default_dimensions" not found`, measure)
		}
	}

	// 0 is default and type is uint32
	if tmp.FirstDayOfWeek > 7 {
		return fmt.Errorf("invalid first day of week %d, must be between 1 and 7", tmp.FirstDayOfWeek)
	}

	// 0 is default and type is uint32
	if tmp.FirstMonthOfYear > 12 {
		return fmt.Errorf("invalid first month of year %d, must be between 1 and 12", tmp.FirstMonthOfYear)
	}

	if tmp.MaxQueryTimeRange != "" {
		if strings.HasPrefix(tmp.MaxQueryTimeRange, "rill-") {
			return fmt.Errorf(`invalid "max_query_time_range" %q: only fixed ISO 8601 day-or-larger durations are allowed`, tmp.MaxQueryTimeRange)
		}
		d, err := duration.ParseISO8601(tmp.MaxQueryTimeRange)
		if err != nil {
			return fmt.Errorf(`invalid "max_query_time_range": %w`, err)
		}
		sd, ok := d.(duration.StandardDuration)
		if !ok || sd.Hour != 0 || sd.Minute != 0 || sd.Second != 0 {
			return fmt.Errorf(`invalid "max_query_time_range" %q: sub-day granularity is not supported, use a duration like P1D, P30D, P3M, or P1Y`, tmp.MaxQueryTimeRange)
		}
	}

	if tmp.Cache.TimestampsTTL != "" {
		if _, err := time.ParseDuration(tmp.Cache.TimestampsTTL); err != nil {
			return fmt.Errorf(`invalid "cache.timestamps_ttl": %w`, err)
		}
	}

	tmp.DefaultComparison.Mode = strings.ToLower(tmp.DefaultComparison.Mode)
	if _, ok := comparisonModesMap[tmp.DefaultComparison.Mode]; !ok {
		return fmt.Errorf("invalid mode: %q. allowed values: %s", tmp.DefaultComparison.Mode, strings.Join(validComparisonModes, ","))
	}
	if tmp.DefaultComparison.Dimension != "" {
		if v, ok := names[strings.ToLower(tmp.DefaultComparison.Dimension)]; !ok && v != nameIsDimension {
			return fmt.Errorf("default comparison dimension %q doesn't exist", tmp.DefaultComparison.Dimension)
		}
	}

	if tmp.AvailableTimeRanges != nil {
		for _, r := range tmp.AvailableTimeRanges {
			_, err := rilltime.Parse(r.Range, rilltime.ParseOptions{})
			if err != nil {
				return fmt.Errorf("invalid range in available_time_ranges: %w", err)
			}

			for _, o := range r.ComparisonTimeRanges {
				err = rilltime.ParseCompatibility(o.Range, o.Offset)
				if err != nil {
					return err
				}
			}
		}
	}

	// Gather all lookup table names
	var lookupTableNames map[string]bool
	for _, dim := range tmp.Dimensions {
		if dim != nil && dim.LookupTable != "" {
			if lookupTableNames == nil {
				lookupTableNames = make(map[string]bool)
			}
			lookupTableNames[dim.LookupTable] = true
		}
	}

	securityRules, err := tmp.Security.Proto()
	if err != nil {
		return err
	}

	if tmp.Model != "" {
		// Not setting Kind because for backwards compatibility, it may actually be a source or an external table.
		node.Refs = append(node.Refs, ResourceName{Name: tmp.Model})
	}
	if tmp.Table != "" {
		// By convention, if the table name matches a source or model name we add a DAG link.
		// We may want to remove this at some point, but the cases where it would not be desired are very rare.
		// Not setting Kind so that inference kicks in.
		node.Refs = append(node.Refs, ResourceName{Name: tmp.Table})
	}
	for _, t := range tmp.TableOptions {
		if t != tmp.Table {
			node.Refs = append(node.Refs, ResourceName{Name: t})
		}
	}

	// Attempt to link the lookup tables in the DAG in case they are models.
	// If they are not models, the upstream logic for refs will filter them out.
	for lookupTable := range lookupTableNames {
		// see if the lookup table name is qualified with a dot, and if so we take the part after the last dot as the name for DAG linking
		if idx := strings.LastIndex(lookupTable, "."); idx >= 0 && idx < len(lookupTable)-1 {
			lookupTable = lookupTable[idx+1:]
		}
		// Not setting Kind so that inference kicks in.
		node.Refs = append(node.Refs, ResourceName{Name: lookupTable})
	}

	if tmp.DefaultTheme != "" {
		node.Refs = append(node.Refs, ResourceName{Kind: ResourceKindTheme, Name: tmp.DefaultTheme})
	}

	// Add annotations as refs to the end of the metrics view.
	for _, annotation := range tmp.Annotations {
		if annotation == nil {
			continue
		}

		if annotation.Table != "" && annotation.Model != "" {
			return fmt.Errorf(`cannot set both the "model" field and the "table" field for annotation`)
		}
		if annotation.Table == "" && annotation.Model == "" {
			return fmt.Errorf(`must set a value for either the "model" field or the "table" field for annotation`)
		}
		if annotation.Name == "" {
			if annotation.Model != "" {
				annotation.Name = annotation.Model
			} else {
				annotation.Name = annotation.Table
			}
		}

		if annotation.Model != "" {
			// Not setting Kind because for backwards compatibility, it may actually be a source or an external table.
			node.Refs = append(node.Refs, ResourceName{Name: annotation.Model})
		} else if annotation.Table != "" {
			// By convention, if the table name matches a source or model name we add a DAG link.
			// We may want to remove this at some point, but the cases where it would not be desired are very rare.
			// Not setting Kind so that inference kicks in.
			node.Refs = append(node.Refs, ResourceName{Name: annotation.Table})
		}
	}

	// Validate and add rollup tables as refs
	if len(tmp.Rollups) > 0 && tmp.TimeDimension == "" {
		return fmt.Errorf(`rollups require a "timeseries" to be defined`)
	}
	var rollups []*runtimev1.MetricsViewSpec_Rollup
	for i, rollup := range tmp.Rollups {
		if rollup == nil {
			return fmt.Errorf(`rollup[%d]: empty rollup configuration`, i)
		}
		if rollup.Model == "" {
			return fmt.Errorf(`rollup[%d]: "model" is required`, i)
		}
		if rollup.TimeGrain == "" {
			return fmt.Errorf(`rollup[%d]: "time_grain" is required`, i)
		}
		tg, err := parseTimeGrain(rollup.TimeGrain)
		if err != nil {
			return fmt.Errorf(`rollup[%d]: invalid "time_grain": %w`, i, err)
		}
		if rollup.TimeZone != "" {
			if _, err := time.LoadLocation(rollup.TimeZone); err != nil {
				return fmt.Errorf(`rollup[%d]: invalid "time_zone" %q: %w`, i, rollup.TimeZone, err)
			}
		}
		if rollup.DataTimeRange != "" {
			if err := validateDataTimeRange(rollup.DataTimeRange); err != nil {
				return fmt.Errorf(`rollup[%d]: invalid "data_time_range": %w`, i, err)
			}
		}
		// Validate and resolve dimensions
		var dims []string
		var dimsSelector *runtimev1.FieldSelector
		if resolved, ok := rollup.Dimensions.TryResolve(); ok {
			for _, dimName := range resolved {
				nameType, ok := names[strings.ToLower(dimName)]
				if !ok || nameType != nameIsDimension {
					return fmt.Errorf(`rollup[%d]: dimension %q does not exist in the metrics view`, i, dimName)
				}
			}
			dims = resolved
		} else {
			dimsSelector = rollup.Dimensions.Proto()
		}
		// Validate and resolve measures
		var measures []string
		var measSelector *runtimev1.FieldSelector
		if resolved, ok := rollup.Measures.TryResolve(); ok {
			for _, mName := range resolved {
				nameType, ok := names[strings.ToLower(mName)]
				if !ok || nameType != nameIsMeasure {
					return fmt.Errorf(`rollup[%d]: measure %q does not exist in the metrics view`, i, mName)
				}
			}
			measures = resolved
		} else {
			measSelector = rollup.Measures.Proto()
		}

		rollups = append(rollups, &runtimev1.MetricsViewSpec_Rollup{
			Database:           rollup.Database,
			DatabaseSchema:     rollup.DatabaseSchema,
			Model:              rollup.Model,
			TimeGrain:          tg,
			TimeZone:           rollup.TimeZone,
			Dimensions:         dims,
			DimensionsSelector: dimsSelector,
			Measures:           measures,
			MeasuresSelector:   measSelector,
			DataTimeRange:      rollup.DataTimeRange,
		})
		node.Refs = append(node.Refs, ResourceName{Name: rollup.Model})
	}

	securityRefs, err := inferRefsFromSecurityRules(securityRules)
	if err != nil {
		return err
	}
	node.Refs = append(node.Refs, securityRefs...)

	node.Refs = append(node.Refs, ResourceName{Kind: ResourceKindConnector, Name: node.Connector})

	var cacheTTLDuration time.Duration
	if tmp.Cache.KeyTTL != "" {
		cacheTTLDuration, err = time.ParseDuration(tmp.Cache.KeyTTL)
		if err != nil {
			return fmt.Errorf(`invalid "cache.key_ttl": %w`, err)
		}
	}

	// Normalize and validate table option variants before any resource is inserted, since errors are not allowed afterwards.
	var tableOptionTables []string
	tableOptionNames := make(map[string]string) // table -> variant metrics view name
	if len(tmp.TableOptions) > 0 {
		seenTables := map[string]bool{tmp.Table: true}
		seenNames := map[string]bool{strings.ToLower(node.Name): true}
		for _, t := range tmp.TableOptions {
			if seenTables[t] {
				continue
			}
			seenTables[t] = true
			variantName := variantMetricsViewName(node.Name, t)
			if seenNames[strings.ToLower(variantName)] {
				return fmt.Errorf("table option %q produces a duplicate metrics view name %q", t, variantName)
			}
			seenNames[strings.ToLower(variantName)] = true
			if _, ok := p.Resources[ResourceName{Kind: ResourceKindMetricsView, Name: variantName}.Normalized()]; ok {
				return fmt.Errorf("table option %q conflicts with an existing resource named %q", t, variantName)
			}
			tableOptionTables = append(tableOptionTables, t)
			tableOptionNames[t] = variantName
		}
	}

	// Cumulative counter measures require a generated normalization model: validate the preconditions
	// and prepare its SQL before any resources are inserted.
	var counterModelName, counterModelSQL string
	if len(counterColumns) > 0 {
		if tmp.TimeDimension == "" {
			return errors.New(`cumulative counter measures require a "timeseries" time dimension (per-series deltas are ordered by it)`)
		}
		if tmp.Parent != "" {
			return errors.New(`cumulative counter measures are not supported on derived metrics views (remove "parent" or normalize in the parent)`)
		}
		if len(tmp.TableOptions) > 0 {
			return errors.New(`cumulative counter measures cannot be combined with "table_options"`)
		}
		var source string
		switch {
		case tmp.Model != "":
			source = safeSQLName(tmp.Model)
		case tmp.Table != "":
			source = safeSQLName(tmp.Table)
		default:
			return errors.New(`cumulative counter measures require a "model" or "table" source`)
		}

		// The series key is the full set of declared dimension columns (excluding the time dimension).
		var partitionCols []string
		for _, dim := range dimensions {
			if dim.Column == tmp.TimeDimension {
				continue
			}
			if dim.Column == "" {
				return fmt.Errorf(`cumulative counter measures require all dimensions to be plain columns, but dimension %q is not (the generated normalization model partitions by the dimension columns)`, dim.Name)
			}
			partitionCols = append(partitionCols, safeSQLName(dim.Column))
		}

		// Reset-safe per-series delta per counter column: when the counter goes backwards
		// (process restart), the new value is the increase.
		cols := make([]string, 0, len(counterColumns))
		for c := range counterColumns {
			cols = append(cols, c)
		}
		slices.Sort(cols)
		var replaces []string
		for _, c := range cols {
			q := safeSQLName(c)
			replaces = append(replaces, fmt.Sprintf(
				"CASE WHEN %[1]s - lag(%[1]s) OVER w_rill_counters < 0 THEN %[1]s ELSE coalesce(%[1]s - lag(%[1]s) OVER w_rill_counters, 0) END AS %[1]s",
				q,
			))
		}
		partitionClause := ""
		if len(partitionCols) > 0 {
			partitionClause = fmt.Sprintf("PARTITION BY %s ", strings.Join(partitionCols, ", "))
		}
		counterModelSQL = fmt.Sprintf(
			"-- Generated by Rill from `kind: counter` measures on metrics view %q. Do not edit.\nSELECT * REPLACE (\n  %s\n)\nFROM %s\nWINDOW w_rill_counters AS (%sORDER BY %s)",
			node.Name,
			strings.Join(replaces, ",\n  "),
			source,
			partitionClause,
			safeSQLName(tmp.TimeDimension),
		)

		counterModelName = node.Name + "__normalized"
		if _, ok := p.Resources[ResourceName{Kind: ResourceKindModel, Name: counterModelName}.Normalized()]; ok {
			return fmt.Errorf("generated counter normalization model %q conflicts with an existing resource", counterModelName)
		}
		node.Refs = append(node.Refs, ResourceName{Kind: ResourceKindModel, Name: counterModelName})
	}

	// validate and insert inline explore, if true and no error is returned from the method then an explore resource is created so no error should be returned after this point
	skipExplore, exploreRes, err := p.parseAndInsertInlineExplore(tmp, node.Name, node.Paths, node.Tags)
	if err != nil {
		return fmt.Errorf("failed to parse inline explore: %w", err)
	}

	// insert metrics view resource immediately after parsing the inline explore as it inserts the explore resource so we should not return an error now
	r, err := p.insertResource(ResourceKindMetricsView, node.Name, node.Paths, node.Tags, node.Refs...)
	if err != nil {
		// If we fail to insert the metrics view, we must delete the inline explore if it was created.
		if exploreRes != nil {
			panic(fmt.Sprintf("failed to insert metrics view %q, but inline explore was created: %s", node.Name, exploreRes.Name))
		}
		return err
	}
	// NOTE: After calling insertResource, an error must not be returned. Any validation should be done before calling it.
	spec := r.MetricsViewSpec

	// Insert one hidden variant metrics view per additional table option. Their specs are populated at the end,
	// once the primary spec is fully built. Collisions were checked above, so insertion must not fail.
	tableOptionResources := make(map[string]*Resource, len(tableOptionTables))
	for _, t := range tableOptionTables {
		vr, err := p.insertResource(ResourceKindMetricsView, tableOptionNames[t], node.Paths, node.Tags, node.Refs...)
		if err != nil {
			panic(fmt.Sprintf("failed to insert table option variant %q for metrics view %q: %s", tableOptionNames[t], node.Name, err))
		}
		tableOptionResources[t] = vr
	}

	// Insert the generated counter normalization model and point the metrics view at it.
	// The collision was checked before insertResource, so insertion must not fail.
	if counterModelName != "" {
		var modelRefs []ResourceName
		if tmp.Model != "" {
			// Not setting Kind so that inference kicks in (it may be a source or an external table).
			modelRefs = append(modelRefs, ResourceName{Name: tmp.Model})
		}
		mr, err := p.insertResource(ResourceKindModel, counterModelName, node.Paths, node.Tags, modelRefs...)
		if err != nil {
			panic(fmt.Sprintf("failed to insert counter normalization model %q for metrics view %q: %s", counterModelName, node.Name, err))
		}
		inputProps, err := structpb.NewStruct(map[string]any{"sql": counterModelSQL})
		if err != nil {
			panic(fmt.Sprintf("failed to serialize counter normalization SQL for metrics view %q: %s", node.Name, err))
		}
		outputProps, err := structpb.NewStruct(map[string]any{"materialize": true})
		if err != nil {
			panic(fmt.Sprintf("failed to serialize counter normalization output properties for metrics view %q: %s", node.Name, err))
		}
		mr.ModelSpec.InputConnector = node.Connector
		mr.ModelSpec.InputProperties = inputProps
		mr.ModelSpec.OutputConnector = node.Connector
		mr.ModelSpec.OutputProperties = outputProps
		mr.ModelSpec.ChangeMode = runtimev1.ModelChangeMode_MODEL_CHANGE_MODE_RESET

		// The metrics view now reads from the normalized model.
		tmp.Model = counterModelName
		tmp.Table = ""
	}

	spec.Parent = tmp.Parent
	spec.Connector = node.Connector
	spec.Database = tmp.Database
	spec.DatabaseSchema = tmp.DatabaseSchema
	spec.Table = tmp.Table
	spec.Model = tmp.Model
	spec.DisplayName = tmp.DisplayName
	if spec.DisplayName == "" {
		spec.DisplayName = ToDisplayName(node.Name)
	}
	spec.Description = tmp.Description
	spec.AiInstructions = tmp.AIInstructions
	spec.TimeDimension = tmp.TimeDimension
	spec.WatermarkExpression = tmp.Watermark
	spec.DataTimeRange = tmp.DataTimeRange
	spec.SmallestTimeGrain = smallestTimeGrain
	spec.FirstDayOfWeek = tmp.FirstDayOfWeek
	spec.FirstMonthOfYear = tmp.FirstMonthOfYear
	spec.MaxQueryTimeRange = tmp.MaxQueryTimeRange
	spec.SkipInvalidDimensions = tmp.SkipInvalidDimensions
	spec.SkipEmptyDimensions = tmp.SkipEmptyDimensions
	if tmp.Cache.TimestampsTTL != "" {
		d, _ := time.ParseDuration(tmp.Cache.TimestampsTTL) // already validated above
		spec.CacheTimestampsTtlSeconds = int64(d.Seconds())
	}

	spec.Dimensions = dimensions
	spec.Measures = measures

	// if time dimension is not defined in the dimensions list but is defined in the `timeseries` key, we prepend it to the dimensions list here
	if !timeDimSeenInDimList && tmp.TimeDimension != "" {
		spec.Dimensions = append([]*runtimev1.MetricsViewSpec_Dimension{
			{
				Name:        tmp.TimeDimension,
				Column:      tmp.TimeDimension,
				DisplayName: ToDisplayName(tmp.TimeDimension),
			},
		}, spec.Dimensions...)
	}

	for _, annotation := range tmp.Annotations {
		if annotation == nil {
			continue
		}
		var annotationMeasuresSelector *runtimev1.FieldSelector
		annotationMeasures, ok := annotation.Measures.TryResolve()
		if !ok {
			annotationMeasuresSelector = annotation.Measures.Proto()
		}

		spec.Annotations = append(spec.Annotations, &runtimev1.MetricsViewSpec_Annotation{
			Name:             annotation.Name,
			Model:            annotation.Model,
			Database:         annotation.Database,
			DatabaseSchema:   annotation.DatabaseSchema,
			Table:            annotation.Table,
			Connector:        annotation.Connector,
			Measures:         annotationMeasures,
			MeasuresSelector: annotationMeasuresSelector,
		})
	}

	spec.Rollups = rollups

	// Parse the dimensions and measures selectors
	if tmp.Parent != "" {
		spec.ParentDimensions = tmp.ParentDimensions.Proto()
		spec.ParentMeasures = tmp.ParentMeasures.Proto()
	}

	spec.SecurityRules = securityRules
	spec.CacheEnabled = tmp.Cache.Enabled
	spec.CacheKeySql = tmp.Cache.KeySQL
	spec.CacheKeyTtlSeconds = int64(cacheTTLDuration.Seconds())
	spec.QueryAttributes = tmp.QueryAttributes

	for _, link := range tmp.RowLinks {
		if link == nil {
			continue
		}
		if err := validateRowLink(link.URL); err != nil {
			return fmt.Errorf(`invalid row link: %w`, err)
		}
		label := link.Label
		if label == "" {
			if u, err := url.Parse(rowLinkPlaceholderRegex.ReplaceAllString(link.URL, "x")); err == nil {
				label = u.Host
			}
		}
		spec.RowLinks = append(spec.RowLinks, &runtimev1.MetricsViewSpec_RowLink{
			Label: label,
			Url:   link.URL,
		})
	}

	// Populate the table option variants as copies of the now fully-built primary spec with a different table,
	// and record the option-to-variant mapping on the primary spec for the frontend's table selector.
	if len(tableOptionTables) > 0 {
		spec.TableOptions = []*runtimev1.MetricsViewSpec_TableOption{{Table: spec.Table, MetricsView: node.Name}}
		for _, t := range tableOptionTables {
			vspec := proto.Clone(spec).(*runtimev1.MetricsViewSpec)
			vspec.Table = t
			vspec.TableOptions = nil
			vspec.TableOptionOf = node.Name
			tableOptionResources[t].MetricsViewSpec = vspec
			spec.TableOptions = append(spec.TableOptions, &runtimev1.MetricsViewSpec_TableOption{Table: t, MetricsView: tableOptionNames[t]})
		}
	}

	// When version is greater than 0 or inline explore is defined or skip explore set to true, we skip creating a default explore resource. Application should set version to 0 now to enable automatic explore emission.
	if node.Version > 0 || skipExplore {
		return nil
	}

	refs := []ResourceName{{Kind: ResourceKindMetricsView, Name: node.Name}}
	if tmp.DefaultTheme != "" {
		refs = append(refs, ResourceName{Kind: ResourceKindTheme, Name: tmp.DefaultTheme})
	}
	e, err := p.insertResource(ResourceKindExplore, node.Name, node.Paths, node.Tags, refs...)
	if err != nil {
		// We mustn't error because we have already emitted one resource.
		// Since this probably means an explore has been defined separately, we can just ignore this error.
		return nil
	}

	e.ExploreSpec.DisplayName = spec.DisplayName
	e.ExploreSpec.Description = spec.Description
	e.ExploreSpec.MetricsView = node.Name
	for _, dim := range spec.Dimensions {
		e.ExploreSpec.Dimensions = append(e.ExploreSpec.Dimensions, dim.Name)
	}
	for _, m := range spec.Measures {
		e.ExploreSpec.Measures = append(e.ExploreSpec.Measures, m.Name)
	}
	if tmp.Parent != "" {
		e.ExploreSpec.DimensionsSelector = &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}}
		e.ExploreSpec.MeasuresSelector = &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}}
	}
	e.ExploreSpec.Theme = tmp.DefaultTheme
	for _, tr := range tmp.AvailableTimeRanges {
		res := &runtimev1.ExploreTimeRange{Range: tr.Range}
		for _, ctr := range tr.ComparisonTimeRanges {
			res.ComparisonTimeRanges = append(res.ComparisonTimeRanges, &runtimev1.ExploreComparisonTimeRange{
				Offset: ctr.Offset,
				Range:  ctr.Range,
			})
		}
		e.ExploreSpec.TimeRanges = append(e.ExploreSpec.TimeRanges, res)
	}
	e.ExploreSpec.TimeZones = tmp.AvailableTimeZones

	var presetDimensionsSelector, presetMeasuresSelector *runtimev1.FieldSelector
	if len(tmp.DefaultDimensions) == 0 {
		presetDimensionsSelector = &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}}
	}
	if len(tmp.DefaultMeasures) == 0 {
		presetMeasuresSelector = &runtimev1.FieldSelector{Selector: &runtimev1.FieldSelector_All{All: true}}
	}
	var tr *string
	if tmp.DefaultTimeRange != "" {
		tr = &tmp.DefaultTimeRange
	}
	var compareDim *string
	if tmp.DefaultComparison.Dimension != "" {
		compareDim = &tmp.DefaultComparison.Dimension
	}
	e.ExploreSpec.DefaultPreset = &runtimev1.ExplorePreset{
		Dimensions:          tmp.DefaultDimensions,
		DimensionsSelector:  presetDimensionsSelector,
		Measures:            tmp.DefaultMeasures,
		MeasuresSelector:    presetMeasuresSelector,
		TimeRange:           tr,
		ComparisonMode:      comparisonModesMap[tmp.DefaultComparison.Mode],
		ComparisonDimension: compareDim,
	}
	// Backwards compatibility: explore parser will default to true so also emit true on the emitted explore spec
	e.ExploreSpec.AllowCustomTimeRange = true
	e.ExploreSpec.DefinedInMetricsView = true

	return nil
}

// parseAndInsertInlineExplore parses and validates the inline explore definition in a metrics view YAML. It returns true if automatic explore emission should be skipped, false otherwise.
func (p *Parser) parseAndInsertInlineExplore(tmp *MetricsViewYAML, mvName string, mvPaths, mvTags []string) (bool, *Resource, error) {
	if tmp.Explore == nil {
		return false, nil, nil
	}
	if tmp.Explore.Skip {
		return true, nil, nil
	}

	if tmp.DefaultTimeRange != "" || len(tmp.AvailableTimeZones) > 0 || tmp.DefaultTheme != "" || len(tmp.DefaultDimensions) > 0 || len(tmp.DefaultMeasures) > 0 || tmp.DefaultComparison.Mode != "" || tmp.DefaultComparison.Dimension != "" || len(tmp.AvailableTimeRanges) > 0 {
		return false, nil, fmt.Errorf("setting defaults or available time zones or ranges under metrics view is deprecated, set them under explore key")
	}

	// Parse and validate the fields shared with standalone explore files
	def, err := p.parseExploreDefinition(&tmp.Explore.ExploreDefinitionYAML)
	if err != nil {
		return false, nil, err
	}

	refs := []ResourceName{{Kind: ResourceKindMetricsView, Name: mvName}}
	if def.themeName != "" && def.themeSpec == nil {
		refs = append(refs, ResourceName{Kind: ResourceKindTheme, Name: def.themeName})
	}

	// before inserting inline explore, dry run inserting the parent metrics view resource to ensure that the explore can be inserted
	err = p.insertDryRun(ResourceKindMetricsView, mvName)
	if err != nil {
		return false, nil, fmt.Errorf("failed to dry run inserting metrics view %q: %w", mvName, err)
	}

	name := mvName
	if tmp.Explore.Name != "" {
		name = tmp.Explore.Name
	}
	// Track explore
	r, err := p.insertResource(ResourceKindExplore, name, mvPaths, mvTags, refs...)
	if err != nil {
		return false, nil, err
	}
	// NOTE: After calling insertResource, an error must not be returned. Any validation should be done before calling it.
	def.applyToSpec(r.ExploreSpec, &tmp.Explore.ExploreDefinitionYAML)
	if r.ExploreSpec.DisplayName == "" {
		r.ExploreSpec.DisplayName = ToDisplayName(name)
	}
	r.ExploreSpec.MetricsView = mvName
	r.ExploreSpec.DefinedInMetricsView = true

	return true, r, nil
}

// parseTimeGrain parses a YAML time grain string
func parseTimeGrain(s string) (runtimev1.TimeGrain, error) {
	switch strings.ToLower(s) {
	case "":
		return runtimev1.TimeGrain_TIME_GRAIN_UNSPECIFIED, nil
	case "ms", "millisecond":
		return runtimev1.TimeGrain_TIME_GRAIN_MILLISECOND, nil
	case "s", "second":
		return runtimev1.TimeGrain_TIME_GRAIN_SECOND, nil
	case "min", "minute":
		return runtimev1.TimeGrain_TIME_GRAIN_MINUTE, nil
	case "h", "hour":
		return runtimev1.TimeGrain_TIME_GRAIN_HOUR, nil
	case "d", "day":
		return runtimev1.TimeGrain_TIME_GRAIN_DAY, nil
	case "w", "week":
		return runtimev1.TimeGrain_TIME_GRAIN_WEEK, nil
	case "month":
		return runtimev1.TimeGrain_TIME_GRAIN_MONTH, nil
	case "q", "quarter":
		return runtimev1.TimeGrain_TIME_GRAIN_QUARTER, nil
	case "y", "year":
		return runtimev1.TimeGrain_TIME_GRAIN_YEAR, nil
	default:
		return runtimev1.TimeGrain_TIME_GRAIN_UNSPECIFIED, fmt.Errorf("invalid time grain %q", s)
	}
}

var validationTemplateData = TemplateData{
	Environment: "dev",
	User: map[string]interface{}{
		"name":   "dummy",
		"email":  "mock@example.org",
		"domain": "example.org",
		"groups": []interface{}{"all"},
		"admin":  false,
	},
	Resolve: func(ref ResourceName) (string, error) {
		return ref.Name, nil
	},
}

// parseNamesYAML parses a []string or a '*' denoting "all names" from a YAML node.
func parseNamesYAML(n yaml.Node) (names []string, all bool, err error) {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value == "*" {
			all = true
			return
		}
		err = fmt.Errorf("unexpected scalar %q", n.Value)
	case yaml.SequenceNode:
		names = make([]string, len(n.Content))
		for i, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				err = fmt.Errorf("unexpected non-string list entry on line %d", c.Line)
				return
			}
			names[i] = c.Value
		}
	default:
		err = fmt.Errorf("invalid field names %v", n)
	}
	return
}

// inferRefsFromSecurityRules infers resource references from security rules.
func inferRefsFromSecurityRules(rules []*runtimev1.SecurityRule) ([]ResourceName, error) {
	var refs []ResourceName
	for _, r := range rules {
		// RowFilter rules are the only rules that can reference external data (since they execute inside the OLAP instead of in the in-memory expression engine).
		if r == nil {
			continue
		}
		rowFilter := r.GetRowFilter()
		if rowFilter == nil {
			continue
		}

		meta, err := AnalyzeTemplate(rowFilter.Sql)
		if err != nil {
			return nil, fmt.Errorf(`invalid 'sql' in row_filter security rule: %w`, err)
		}

		refs = append(refs, meta.Refs...)
	}
	// No need to deduplicate because that's done upstream when the resource is inserted.
	return refs, nil
}

// validateDataTimeRange parses a data_time_range expression and rejects it if it resolves to an
// unbounded lower bound (e.g. "inf" or "earliest to now"). A zero start is the system-wide assumption
// for "no time data present" (see valOrNullTime in the server and the metrics_time_range resolver),
// so a declared range must have a concrete start; otherwise omit data_time_range to probe the table.
func validateDataTimeRange(expr string) error {
	rt, err := rilltime.Parse(expr, rilltime.ParseOptions{})
	if err != nil {
		return err
	}
	// Evaluate against the same synthetic anchors used when resolving declared ranges.
	now := time.Now()
	start, _, _ := rt.Eval(rilltime.EvalOptions{Now: now, MinTime: time.Time{}, MaxTime: now, Watermark: now})
	if start.IsZero() {
		return errors.New("must have a bounded start; \"inf\" and \"earliest\" resolve to an unbounded lower bound which the system treats as no data (use a concrete range like \"-90d to now\", or omit data_time_range to detect bounds from the table)")
	}
	return nil
}

// validateDimensionLink validates a dimension value link URL template.
// The template may contain "{{ value }}" placeholders, which the UI replaces with the
// URL-encoded dimension value; the URL must be absolute with an http or https scheme.
func validateDimensionLink(rawURL string) error {
	if rawURL == "" {
		return errors.New(`"url" is required`)
	}
	// Substitute placeholders with a dummy value so templates parse as URLs.
	probe := dimensionLinkPlaceholderRegex.ReplaceAllString(rawURL, "x")
	u, err := url.Parse(probe)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid url %q: must be an absolute http(s) URL", rawURL)
	}
	return nil
}

var dimensionLinkPlaceholderRegex = regexp.MustCompile(`\{\{\s*value\s*\}\}`)

// validateRowLink validates a row link URL template.
// The template may contain "{{ <column> }}" placeholders, which the UI replaces with the
// URL-encoded value of that column in the clicked row.
func validateRowLink(rawURL string) error {
	if rawURL == "" {
		return errors.New(`"url" is required`)
	}
	probe := rowLinkPlaceholderRegex.ReplaceAllString(rawURL, "x")
	u, err := url.Parse(probe)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid url %q: must be an absolute http(s) URL", rawURL)
	}
	return nil
}

var rowLinkPlaceholderRegex = regexp.MustCompile(`\{\{\s*[^}]+\s*\}\}`)

// validateQueryAttributes validates query attribute keys
func validateQueryAttributes(attrs map[string]string) error {
	for key := range attrs {
		if !queryAttributeKeyRegex.MatchString(key) {
			return fmt.Errorf("query attribute key %q contains invalid characters (must be alphanumeric with underscores, hyphens, or dots only)", key)
		}
	}
	return nil
}

var queryAttributeKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
