package crud

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/tools"
)

// Update method to define the logic which updates the resource and sets the updated Terraform state on success.
func Update(ctx context.Context, r helpers.VyosResource, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tools.Debug(ctx, "Update Resource")
	ctx = r.GetProviderConfig().CtxMutilatorRun(ctx)

	tools.Trace(ctx, "Fetching data model")
	// Current State as resource model
	stateModel := r.GetModel()

	// Plan as resource model
	//  ! since GetModel returns a ptr we must create a new struct with a ptr to not mix up state and plan
	//		This is only a problem here in Update() as other CRUD functions only need to keep track of 1 state/model
	modelType := reflect.TypeOf(stateModel)
	pointerType := modelType.Elem()
	reflectedValue := reflect.New(pointerType)
	planModel := reflectedValue.Interface().(helpers.VyosTopResourceDataModel)

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, stateModel)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, planModel)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Setup timeout
	createTimeout, diags := stateModel.GetTimeouts().Update(ctx, time.Duration(r.GetProviderConfig().Config.CrudDefaultTimeouts)*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Execute
	err := update(ctx, r.GetClient(), stateModel, planModel)
	if err != nil {
		resp.Diagnostics.AddError("API Config error", err.Error())
		return
	}

	// Save data to Terraform state
	tools.Debug(ctx, "resource created, saving state")
	helpers.UnknownToNull(ctx, planModel)
	resp.Diagnostics.Append(resp.State.Set(ctx, planModel)...)
}

// update re-configures all resource parameters
// model must be a ptr
// this function is separated out to keep the terraform provider
// logic and API logic separate so we can test the API logic easier
func update(ctx context.Context, client client.Client, stateModel, planModel helpers.VyosTopResourceDataModel) error {
	// Get existing and planned config based on Terraform state and plan
	stateVyosData, err := helpers.MarshalVyos(ctx, stateModel)
	if err != nil {
		return fmt.Errorf("API marshalling error: %s", err)
	}

	planVyosData, err := helpers.MarshalVyos(ctx, planModel)
	if err != nil {
		return fmt.Errorf("API marshalling error: %s", err)
	}

	// Delete fields that are in state but not in plan
	resourcePath := stateModel.GetVyosPath()
	for key := range stateVyosData {
		if _, exists := planVyosData[key]; !exists {
			deletePath := append(slices.Clone(resourcePath), key)
			client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, deletePath, nil))
		}
	}

	// For any list-like attribute present in the plan, ensure that
	// any values which are currently configured on VyOS but not
	// present in the plan are explicitly deleted. This provides
	// full replacement semantics for lists like interface addresses
	// or allowed-ips.
	for key, planValue := range planVyosData {
		stateValue, _ := stateVyosData[key]
		resetListValueIfNeeded(ctx, client, resourcePath, key, stateValue, planValue)
	}

	// Set the new config
	vyosOpsPlan := helpers.GenerateVyosOps(ctx, planModel.GetVyosPath(), planVyosData)
	tools.Trace(ctx, "Compiling vyos plan operations", map[string]interface{}{"vyosOpsPlan": vyosOpsPlan})

	client.StageSet(ctx, vyosOpsPlan)

	// Commit changes to api
	response, err := client.CommitChanges(ctx)
	if err != nil {
		return fmt.Errorf("client error: unable to update '%s', got error: %s", planModel.GetVyosPath(), err)
	}
	if response != nil {
		tools.Warn(ctx, "Got non-nil response from API", map[string]interface{}{"response": response})
	}

	// Add ID to the resource model as plan fetching do not include it
	planModel.SetID(planModel.GetVyosPath())

	return nil
}

func resetListValueIfNeeded(ctx context.Context, client client.Client, basePath []string, key string, stateValue, planValue any) {
	// Detect whether this attribute is list-like in the plan. If not,
	// there is nothing to do here.
	planSlice, okPlan := stringSliceFromAny(planValue)
	if !okPlan {
		tools.Trace(ctx, "resetListValueIfNeeded: attribute is not list-like in plan, skipping", map[string]interface{}{
			"key": key,
		})
		return
	}

	// Convert current state value (from live VyOS) into a slice of
	// strings if possible. If this is not list-like in state, there
	// is nothing to delete.
	stateSlice, okState := stringSliceFromAny(stateValue)
	if !okState {
		tools.Trace(ctx, "resetListValueIfNeeded: attribute not list-like in state, nothing to delete", map[string]interface{}{
			"key": key,
		})
		return
	}

	// Build a lookup set for values that should remain according to
	// the plan.
	planSet := make(map[string]struct{}, len(planSlice))
	for _, v := range planSlice {
		planSet[v] = struct{}{}
	}

	// For any value that exists in state but not in the plan, stage
	// an explicit delete for that particular list element.
	for _, current := range stateSlice {
		if _, keep := planSet[current]; keep {
			continue
		}

		deletePath := append(slices.Clone(basePath), key, current)
		tools.Info(ctx, "resetListValueIfNeeded: deleting stale list element", map[string]interface{}{
			"deletePath": deletePath,
			"key":        key,
			"value":      current,
		})
		client.StageDelete(ctx, [][]string{deletePath})
	}
}

func stringSliceFromAny(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []interface{}:
		res := make([]string, 0, len(v))
		for _, item := range v {
			res = append(res, fmt.Sprint(item))
		}
		return res, true
	default:
		return nil, false
	}
}
