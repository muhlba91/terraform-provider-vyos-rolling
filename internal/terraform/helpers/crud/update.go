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

	// For any list-like attribute present in the plan, ensure the
	// corresponding key on VyOS is deleted before re-applying the list
	// from the plan. This guarantees full replacement semantics for
	// lists like interface addresses, even if Terraform state does not
	// reflect all values currently configured on the device.
	for key, planValue := range planVyosData {
		resetListValueIfNeeded(ctx, client, resourcePath, key, stateVyosData[key], planValue)
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

	// If we don't have a list-like state value, fall back to deleting
	// the entire key to guarantee that any existing config is removed
	// before we apply the plan.
	stateSlice, okState := stringSliceFromAny(stateValue)
	if !okState {
		deletePath := append(slices.Clone(basePath), key)
		tools.Info(ctx, "resetListValueIfNeeded: no list-like state, deleting whole key", map[string]interface{}{
			"deletePath": deletePath,
			"key":        key,
		})
		client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, deletePath, nil))
		return
	}

	// Compute which elements exist on the device (state) but are not
	// present in the plan, and stage explicit deletes for each of those
	// elements. This ensures we remove stale entries (like old
	// interface addresses) while keeping the ones that are still
	// desired by the plan.
	toDelete := make([]string, 0)
	for _, existing := range stateSlice {
		if !slices.Contains(planSlice, existing) {
			toDelete = append(toDelete, existing)
		}
	}

	if len(toDelete) == 0 {
		tools.Trace(ctx, "resetListValueIfNeeded: no stale list elements to delete", map[string]interface{}{
			"key": key,
		})
		return
	}

	for _, value := range toDelete {
		deletePath := append(append(slices.Clone(basePath), key), value)
		tools.Info(ctx, "resetListValueIfNeeded: staging delete for stale list element", map[string]interface{}{
			"deletePath": deletePath,
			"key":        key,
			"value":      value,
		})
		client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, deletePath, nil))
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
