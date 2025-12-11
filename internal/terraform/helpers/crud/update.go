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
	// Refresh live state from VyOS to avoid drift where Terraform state
	// no longer reflects the actual device configuration (for example,
	// when previous provider versions failed to delete list elements
	// like interface addresses). This ensures list-diff logic below
	// sees the real current values on the router.
	if err := read(ctx, client, stateModel); err != nil {
		return fmt.Errorf("API read error before update: %s", err)
	}

	// Get existing and planned config
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
	for key, stateValue := range stateVyosData {
		planValue, exists := planVyosData[key]
		if !exists {
			deletePath := append(slices.Clone(resourcePath), key)
			client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, deletePath, nil))
			continue
		}

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
	stateList, ok := stringSliceFromAny(stateValue)
	if !ok {
		return
	}

	planList, ok := stringSliceFromAny(planValue)
	if !ok {
		return
	}

	// If the lists are identical, no reset is required.
	if slices.Equal(stateList, planList) {
		return
	}

	// Compute which values exist in state but not in plan; only those
	// should be deleted. This avoids unnecessarily deleting values that
	// are still desired while ensuring removed entries are cleaned up.
	planSet := make(map[string]struct{}, len(planList))
	for _, v := range planList {
		planSet[v] = struct{}{}
	}

	toDelete := make([]string, 0, len(stateList))
	for _, v := range stateList {
		if _, exists := planSet[v]; !exists {
			toDelete = append(toDelete, v)
		}
	}

	// Nothing to delete – differences may be only in ordering.
	if len(toDelete) == 0 {
		return
	}

	// Build a vyosData map so GenerateVyosOps produces per-element
	// operations like: [interfaces wireguard wgX address 172.16.x.x/30]
	vyosData := map[string]any{
		key: toDelete,
	}

	client.StageDelete(ctx, helpers.GenerateVyosOps(ctx, slices.Clone(basePath), vyosData))
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
