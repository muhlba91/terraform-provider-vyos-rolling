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

	// Delete fields that are in state but not in plan, including nested
	// attributes. This ensures that when a leaf attribute (for example,
	// an address-mask under a destination block) is removed from the
	// Terraform configuration, we issue the appropriate delete operation
	// to VyOS even if the parent block still exists.
	resourcePath := stateModel.GetVyosPath()
	deleteStateNotInPlan(ctx, &client, resourcePath, stateVyosData, planVyosData)

	// For any list-like attribute present in the plan, ensure the
	// corresponding key on VyOS is deleted before re-applying the list
	// from the plan. This guarantees full replacement semantics for
	// lists like interface addresses and firewall zone members regardless
	// of what is currently configured on the device. We do this
	// recursively so that nested list attributes (for example,
	// member.interface on a firewall zone) are also handled.
	resetListValuesRecursive(ctx, &client, resourcePath, planVyosData)

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

func resetListValueIfNeeded(ctx context.Context, client *client.Client, basePath []string, key string, planValue any) {
	// Detect whether this attribute is list-like in the plan. If not,
	// there is nothing to do here.
	if _, okPlan := stringSliceFromAny(planValue); !okPlan {
		tools.Trace(ctx, "resetListValueIfNeeded: attribute is not list-like in plan, skipping", map[string]interface{}{
			"key": key,
		})
		return
	}

	// Always delete the entire key before applying the new list
	// values from the plan. This guarantees that any existing values
	// on VyOS (including ones Terraform state never saw) are removed.
	deletePath := append(slices.Clone(basePath), key)
	tools.Info(ctx, "resetListValueIfNeeded: deleting whole list-valued key before reset", map[string]interface{}{
		"deletePath": deletePath,
		"key":        key,
	})
	client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, deletePath, nil))
}

// deleteStateNotInPlan walks the marshalled VyOS representation of the
// current (state) and desired (plan) configuration and stages delete
// operations for any keys that exist in state but not in plan. It
// operates recursively so that nested attributes like
// destination.address-mask are also removed when they are no longer
// present in the plan.
func deleteStateNotInPlan(ctx context.Context, client *client.Client, basePath []string, stateMap, planMap map[string]any) {
	for key, stateVal := range stateMap {
		planVal, exists := planMap[key]
		if !exists {
			deletePath := append(slices.Clone(basePath), key)
			tools.Info(ctx, "deleteStateNotInPlan: deleting key missing from plan", map[string]interface{}{
				"deletePath": deletePath,
				"key":        key,
			})
			client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, deletePath, nil))
			continue
		}

		// If both values are nested maps, recurse into them so that we
		// can detect keys that were removed deeper in the structure.
		stateSub, okState := stateVal.(map[string]any)
		planSub, okPlan := planVal.(map[string]any)
		if okState && okPlan {
			deleteStateNotInPlan(ctx, client, append(slices.Clone(basePath), key), stateSub, planSub)
		}
	}
}

// resetListValuesRecursive traverses the planned VyOS data and applies
// list reset semantics to any list-like attributes it finds. It ensures
// we delete the full key (for example, member.interface) before
// re-applying all values from the plan.
func resetListValuesRecursive(ctx context.Context, client *client.Client, basePath []string, planMap map[string]any) {
	for key, planVal := range planMap {
		if _, ok := stringSliceFromAny(planVal); ok {
			resetListValueIfNeeded(ctx, client, basePath, key, planVal)
			continue
		}

		if subMap, ok := planVal.(map[string]any); ok {
			resetListValuesRecursive(ctx, client, append(slices.Clone(basePath), key), subMap)
		}
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
