package reconcilers

import (
	"context"
	"errors"
	"fmt"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime"
	"github.com/rilldata/rill/runtime/metricsview/executor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	runtime.RegisterReconcilerInitializer(runtime.ResourceKindMetricsView, newMetricsViewReconciler)
}

type MetricsViewReconciler struct {
	C *runtime.Controller
}

func newMetricsViewReconciler(ctx context.Context, c *runtime.Controller) (runtime.Reconciler, error) {
	return &MetricsViewReconciler{C: c}, nil
}

func (r *MetricsViewReconciler) Close(ctx context.Context) error {
	return nil
}

func (r *MetricsViewReconciler) AssignSpec(from, to *runtimev1.Resource) error {
	a := from.GetMetricsView()
	b := to.GetMetricsView()
	if a == nil || b == nil {
		return fmt.Errorf("cannot assign spec from %T to %T", from.Resource, to.Resource)
	}
	b.Spec = a.Spec
	return nil
}

func (r *MetricsViewReconciler) AssignState(from, to *runtimev1.Resource) error {
	a := from.GetMetricsView()
	b := to.GetMetricsView()
	if a == nil || b == nil {
		return fmt.Errorf("cannot assign state from %T to %T", from.Resource, to.Resource)
	}
	b.State = a.State
	return nil
}

func (r *MetricsViewReconciler) ResetState(res *runtimev1.Resource) error {
	res.GetMetricsView().State = &runtimev1.MetricsViewState{}
	return nil
}

func (r *MetricsViewReconciler) Reconcile(ctx context.Context, n *runtimev1.ResourceName) runtime.ReconcileResult {
	self, err := r.C.Get(ctx, n, true)
	if err != nil {
		return runtime.ReconcileResult{Err: err}
	}
	mv := self.GetMetricsView()
	if mv == nil {
		return runtime.ReconcileResult{Err: errors.New("not a metrics view")}
	}

	// Exit early for deletion
	if self.Meta.DeletedOn != nil {
		return runtime.ReconcileResult{}
	}

	// Get instance config
	cfg, err := r.C.Runtime.InstanceConfig(ctx, r.C.InstanceID)
	if err != nil {
		return runtime.ReconcileResult{Err: err}
	}

	// If the spec references a model, try resolving it to a table before validating it.
	// For backwards compatibility, the model may actually be a source or external table.
	// So if a model is not found, we optimistically use the model name as the table and proceed to validation
	var dataRefreshedOn *timestamppb.Timestamp
	if mv.Spec.Model != "" {
		res, err := r.C.Get(ctx, &runtimev1.ResourceName{Name: mv.Spec.Model, Kind: runtime.ResourceKindModel}, false)
		if err == nil && res.GetModel().State.ResultTable != "" {
			mv.Spec.Table = res.GetModel().State.ResultTable
			mv.Spec.Connector = res.GetModel().State.ResultConnector
			dataRefreshedOn = res.GetModel().State.RefreshedOn
		} else {
			mv.Spec.Table = mv.Spec.Model
		}
	}

	// Resolve rollup model names to table names (same pattern as main model resolution above)
	for _, rollup := range mv.Spec.Rollups {
		if rollup.Model != "" {
			res, err := r.C.Get(ctx, &runtimev1.ResourceName{Name: rollup.Model, Kind: runtime.ResourceKindModel}, false)
			if err == nil && res.GetModel().State.ResultTable != "" {
				rollup.Table = res.GetModel().State.ResultTable
			} else {
				rollup.Table = rollup.Model
			}
		}
	}

	refsForHasInternalRefCheck := self.Meta.Refs
	parentModel := ""
	parentTable := ""
	if mv.Spec.Parent != "" {
		parent, err := r.C.Get(ctx, &runtimev1.ResourceName{
			Name: mv.Spec.Parent,
			Kind: runtime.ResourceKindMetricsView,
		}, false)
		if err != nil {
			return runtime.ReconcileResult{Err: fmt.Errorf("failed to get parent metrics view %q: %w", mv.Spec.Parent, err)}
		}
		refsForHasInternalRefCheck = parent.Meta.Refs
		if parent.GetMetricsView().State.ValidSpec == nil {
			return runtime.ReconcileResult{Err: fmt.Errorf("parent metrics view %q deos not have a valid state", parent.Meta.Name.Name)}
		}
		parentModel = parent.GetMetricsView().State.ValidSpec.Model
		parentTable = parent.GetMetricsView().State.ValidSpec.Table
		if dataRefreshedOn == nil {
			dataRefreshedOn = parent.GetMetricsView().State.DataRefreshedOn
		}
	}

	// Find out if the metrics view has a ref to a source or model in the same project.
	hasInternalRef := false
	for _, ref := range refsForHasInternalRefCheck {
		// Check that the name matches the metrics view's table. This is to avoid false positive for annotation's model.
		if (ref.Name == mv.Spec.Table || ref.Name == mv.Spec.Model || ref.Name == parentTable || ref.Name == parentModel) &&
			(ref.Kind == runtime.ResourceKindSource || ref.Kind == runtime.ResourceKindModel) {
			hasInternalRef = true
		}
	}

	// NOTE: In other reconcilers, state like spec_hash and refreshed_on is used to avoid redundant reconciles.
	// We don't do that here because none of the operations below are particularly expensive.
	// So it doesn't really matter if they run a bit more often than necessary ¯\_(ツ)_/¯.

	// NOTE: Not checking refs for errors since they may still be valid even if they have errors. Instead, we just validate the metrics view against the table name.

	// Expand dimensions that declare a map_column by discovering the map's keys in the data.
	// The expanded spec is what gets validated and captured in the state; mv.Spec itself is left untouched.
	baseSpec, validateWarnings, validateErr := r.expandMapDimensions(ctx, mv.Spec)
	if baseSpec == nil {
		baseSpec = mv.Spec
	}

	// Validate the metrics view and update ValidSpec
	var validateResult *executor.ValidateMetricsViewResult
	if validateErr == nil {
		e, err := executor.New(ctx, r.C.Runtime, r.C.InstanceID, baseSpec, !hasInternalRef, runtime.ResolvedSecurityOpen, 0, nil)
		if err != nil {
			return runtime.ReconcileResult{Err: fmt.Errorf("failed to create metrics view executor: %w", err)}
		}
		validateResult, validateErr = e.ValidateAndNormalizeMetricsView(ctx)
		e.Close()
	}

	// The spec that will be captured in the state. May differ from mv.Spec if invalid dimensions are skipped below.
	validSpec := baseSpec
	if validateErr == nil && baseSpec.SkipInvalidDimensions && !validateResult.IsZero() {
		prunedSpec, warnings, pruneErr := r.pruneInvalidDimensions(ctx, baseSpec, validateResult, !hasInternalRef)
		if pruneErr == nil {
			validSpec = prunedSpec
			validateWarnings = append(validateWarnings, warnings...)
			validateResult = nil
		} else if !errors.Is(pruneErr, ctx.Err()) {
			validateErr = pruneErr
		}
	}
	if validateErr == nil && validateResult != nil {
		validateErr = validateResult.Error()
	}
	if validateErr == nil && validSpec.SkipEmptyDimensions {
		prunedSpec, warnings, pruneErr := r.pruneEmptyDimensions(ctx, validSpec, !hasInternalRef)
		if pruneErr == nil {
			validSpec = prunedSpec
			validateWarnings = append(validateWarnings, warnings...)
		} else if !errors.Is(pruneErr, ctx.Err()) {
			validateErr = pruneErr
		}
	}
	if ctx.Err() != nil { // May not be handled in all validation implementations
		return runtime.ReconcileResult{Err: ctx.Err()}
	}
	if validateErr != nil {
		// When not staging changes, clear the previously valid spec.
		// Otherwise, we keep serving the previously valid spec.
		if !cfg.StageChanges {
			mv.State.ValidSpec = nil
			mv.State.Streaming = false
			mv.State.DataRefreshedOn = nil
			err = r.C.UpdateState(ctx, self.Meta.Name, self)
			if err != nil {
				return runtime.ReconcileResult{Err: err}
			}
		}

		// Return the validation error
		return runtime.ReconcileResult{Err: validateErr}
	}

	// Capture the spec, which we now know to be valid.
	mv.State.ValidSpec = validSpec
	// If there's no internal ref, we assume the metrics view is based on an externally managed table and set the streaming state to true.
	mv.State.Streaming = !hasInternalRef
	// We copy the underlying model's refreshed_on timestamp to the metrics view state since dashboard users may not have access to the underlying model resource.
	mv.State.DataRefreshedOn = dataRefreshedOn
	// Update the state. Even if the validation result is unchanged, we always update the state to ensure the state version is incremented.
	err = r.C.UpdateState(ctx, self.Meta.Name, self)
	if err != nil {
		return runtime.ReconcileResult{Err: err}
	}

	return runtime.ReconcileResult{Warnings: validateWarnings}
}

// expandMapDimensions replaces dimensions that declare a map_column with concrete dimensions
// for the keys discovered in the data, returning a new spec and leaving the input spec untouched.
// It returns a nil spec if the input spec has no map dimensions.
func (r *MetricsViewReconciler) expandMapDimensions(ctx context.Context, spec *runtimev1.MetricsViewSpec) (*runtimev1.MetricsViewSpec, []string, error) {
	hasMap := false
	for _, d := range spec.Dimensions {
		if d.MapColumn != "" {
			hasMap = true
			break
		}
	}
	if !hasMap {
		return nil, nil, nil
	}

	// The streaming flag doesn't affect key discovery, so it's simply set to false here.
	e, err := executor.New(ctx, r.C.Runtime, r.C.InstanceID, spec, false, runtime.ResolvedSecurityOpen, 0, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create metrics view executor: %w", err)
	}
	defer e.Close()
	return e.ExpandMapDimensions(ctx)
}

// pruneInvalidDimensions attempts to produce a valid spec by removing dimensions that fail validation.
// It returns an error if validation fails for any reason other than invalid dimensions.
// Since validation may short-circuit on the first errors it encounters,
// it re-validates after each round of pruning until the spec is valid (bounded to avoid endless loops).
func (r *MetricsViewReconciler) pruneInvalidDimensions(ctx context.Context, spec *runtimev1.MetricsViewSpec, res *executor.ValidateMetricsViewResult, streaming bool) (*runtimev1.MetricsViewSpec, []string, error) {
	pruned := proto.Clone(spec).(*runtimev1.MetricsViewSpec)
	var warnings []string
	for range 5 {
		// Errors other than dimension errors cannot be resolved by pruning.
		if res.TimeDimensionErr != nil || len(res.MeasureErrs) > 0 || len(res.OtherErrs) > 0 || len(res.DimensionErrs) == 0 {
			return nil, nil, res.Error()
		}

		remove := make(map[int]bool, len(res.DimensionErrs))
		for _, ie := range res.DimensionErrs {
			if ie.Idx >= len(pruned.Dimensions) {
				return nil, nil, res.Error()
			}
			remove[ie.Idx] = true
			warnings = append(warnings, fmt.Sprintf("skipped dimension %q: %s", pruned.Dimensions[ie.Idx].Name, ie.Err.Error()))
		}
		dims := make([]*runtimev1.MetricsViewSpec_Dimension, 0, len(pruned.Dimensions)-len(remove))
		for i, d := range pruned.Dimensions {
			if !remove[i] {
				dims = append(dims, d)
			}
		}
		pruned.Dimensions = dims

		e, err := executor.New(ctx, r.C.Runtime, r.C.InstanceID, pruned, streaming, runtime.ResolvedSecurityOpen, 0, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create metrics view executor: %w", err)
		}
		res, err = e.ValidateAndNormalizeMetricsView(ctx)
		e.Close()
		if err != nil {
			return nil, nil, err
		}
		if res.IsZero() {
			return pruned, warnings, nil
		}
	}
	return nil, nil, res.Error()
}

// pruneEmptyDimensions removes dimensions whose values are NULL in every row of the underlying table.
// It supports schemaless data sources where the table schema is a union of all fields ever ingested,
// so a field not present in the current data shows up as an all-NULL column.
// The input spec must already be valid and normalized.
// The pruned spec is deliberately not re-validated:
// removing a dimension cannot break the remaining dimensions and measures,
// and re-validation would reject specs whose security rules or rollups reference a hidden dimension,
// which is expected for schemaless sources where the dimension comes back once its field carries data again.
func (r *MetricsViewReconciler) pruneEmptyDimensions(ctx context.Context, spec *runtimev1.MetricsViewSpec, streaming bool) (*runtimev1.MetricsViewSpec, []string, error) {
	e, err := executor.New(ctx, r.C.Runtime, r.C.InstanceID, spec, streaming, runtime.ResolvedSecurityOpen, 0, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create metrics view executor: %w", err)
	}
	empty, err := e.EmptyDimensions(ctx)
	e.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find empty dimensions: %w", err)
	}
	if len(empty) == 0 {
		return spec, nil, nil
	}

	remove := make(map[int]bool, len(empty))
	warnings := make([]string, 0, len(empty))
	for _, idx := range empty {
		remove[idx] = true
		warnings = append(warnings, fmt.Sprintf("hid empty dimension %q: all values are NULL", spec.Dimensions[idx].Name))
	}
	pruned := proto.Clone(spec).(*runtimev1.MetricsViewSpec)
	dims := make([]*runtimev1.MetricsViewSpec_Dimension, 0, len(pruned.Dimensions)-len(remove))
	for i, d := range pruned.Dimensions {
		if !remove[i] {
			dims = append(dims, d)
		}
	}
	pruned.Dimensions = dims
	return pruned, warnings, nil
}

func (r *MetricsViewReconciler) ResolveTransitiveAccess(ctx context.Context, claims *runtime.SecurityClaims, res *runtimev1.Resource) ([]*runtimev1.SecurityRule, error) {
	if res.GetMetricsView() == nil {
		return nil, fmt.Errorf("not a metrics view resource")
	}
	return []*runtimev1.SecurityRule{{Rule: runtime.SelfAllowRuleAccess(res)}}, nil
}
