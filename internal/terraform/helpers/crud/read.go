package crud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/client/clienterrors"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	cruderrors "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/crud/cruderror"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/tools"
)

// Read method to define the logic which refreshes the Terraform state for the resource.
func Read(ctx context.Context, r helpers.VyosResource, req resource.ReadRequest, resp *resource.ReadResponse) {
	tools.Debug(ctx, "Read Resource")
	ctx = r.GetProviderConfig().CtxMutilatorRun(ctx)

	tools.Trace(ctx, "Fetching data model")
	stateModel := r.GetModel()

	// Read Terraform prior state data into the model
	tools.Debug(ctx, "Fetching state data")
	diags := req.State.Get(ctx, stateModel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tools.Trace(ctx, "Fetched state data", map[string]interface{}{"state-model": stateModel})

	// Setup timeout
	createTimeout, diags := stateModel.GetTimeouts().Read(ctx, time.Duration(r.GetProviderConfig().Config.CrudDefaultTimeouts)*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Fetch live state from Vyos
	err := read(ctx, r.GetClient(), stateModel)

	var rnfErr cruderrors.ResourceNotFoundError
	if errors.As(err, &rnfErr) {
		// `exists` can be a false-negative in some environments; before removing the
		// resource from state, double-check via `showConfig`.
		path := stateModel.GetVyosPath()
		response, getErr := r.GetClient().Get(ctx, path)
		var nfErr clienterrors.NotFoundError
		if errors.As(getErr, &nfErr) {
			// Some VyOS endpoints can return an empty response for the exact resource
			// path, while still returning the resource nested under a higher-level
			// parent path. Try to recover by fetching a parent subtree and traversing
			// down to the resource.
			tryRecoverFrom := func(base []string, remaining []string) (bool, error) {
				parentResp, parentErr := r.GetClient().Get(ctx, base)
				if errors.As(parentErr, &nfErr) {
					return false, nil
				}
				if parentErr != nil {
					return false, parentErr
				}
				cur, ok := parentResp.(map[string]any)
				if !ok {
					return false, nil
				}

				// Some API responses include one or more of the queried base segments
				// as wrapper keys (e.g. {"firewall": {"zone": {...}}} when querying
				// ["firewall"] or ["firewall","zone"]). Unwrap these wrappers before
				// attempting to traverse the remaining path.
				for _, seg := range base {
					val, ok := cur[seg]
					if !ok {
						continue
					}
					next, ok := val.(map[string]any)
					if !ok {
						continue
					}
					cur = next
				}

				for i, seg := range remaining {
					val, ok := cur[seg]
					if !ok {
						return false, nil
					}
					if i == len(remaining)-1 {
						leafMap, ok := val.(map[string]any)
						if !ok {
							return false, nil
						}
						if unmarshalErr := helpers.UnmarshalVyos(ctx, leafMap, stateModel); unmarshalErr != nil {
							return false, unmarshalErr
						}
						return true, nil
					}
					next, ok := val.(map[string]any)
					if !ok {
						return false, nil
					}
					cur = next
				}
				return false, nil
			}

			// Try a few increasingly higher-level parents.
			for cut := len(path) - 1; cut >= 1 && cut >= len(path)-3; cut-- {
				base := path[:cut]
				remaining := path[cut:]
				recovered, recErr := tryRecoverFrom(base, remaining)
				if recErr != nil {
					resp.Diagnostics.AddError("unable to retrieve config", recErr.Error())
					return
				}
				if recovered {
					err = nil
					break
				}
			}
			if err != nil {
				if r.GetProviderConfig().Config.ReadRemoveMissingOnRefresh {
					resp.Diagnostics.AddWarning(
						"resource missing during refresh",
						fmt.Sprintf("Removing from state because the VyOS API did not return configuration for path %q", strings.Join(path, " ")),
					)
					resp.State.RemoveResource(ctx)
					return
				}
				// Be conservative: some VyOS endpoints can return false negatives for
				// `exists` and even `showConfig` on the exact path and nearby parents.
				// Removing from state would force a recreate on the next plan/apply,
				// which can be dangerous for networking constructs.
				resp.Diagnostics.AddWarning(
					"unable to confirm resource presence during refresh",
					fmt.Sprintf("Keeping prior state because the VyOS API did not return configuration for path %q", strings.Join(path, " ")),
				)
				helpers.UnknownToNull(ctx, stateModel)
				resp.Diagnostics.Append(resp.State.Set(ctx, stateModel)...)
				return
			}
		} else {
			if getErr != nil {
				resp.Diagnostics.AddError("unable to retrieve config", getErr.Error())
				return
			}
			if responseAssrt, ok := response.(map[string]any); ok {
				unmarshalErr := helpers.UnmarshalVyos(ctx, responseAssrt, stateModel)
				if unmarshalErr != nil {
					resp.Diagnostics.AddError("unable to retrieve config", unmarshalErr.Error())
					return
				}
				err = nil
			} else {
				resp.Diagnostics.AddError("unable to retrieve config", fmt.Errorf("unknown api response: %#v", response).Error())
				return
			}
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("unable to retrieve config", err.Error())
		return
	}

	// Save data to Terraform state
	tools.Debug(ctx, "resource created, saving state")
	helpers.UnknownToNull(ctx, stateModel)
	resp.Diagnostics.Append(resp.State.Set(ctx, stateModel)...)
}

// read populates resource model
// model must be a ptr
// this function is separated out to keep the terraform provider
// logic and API logic separate so we can test the API logic easier
func read(ctx context.Context, c *client.Client, model helpers.VyosTopResourceDataModel) error {
	tools.Debug(ctx, "Fetching API data", map[string]interface{}{"vyos-path": model.GetVyosPath()})

	// Check if we exists, if so this means we are an empty resource
	ret, hasErr := c.Has(ctx, model.GetVyosPath())
	if hasErr != nil {
		return fmt.Errorf("[%s] resource lookup: %w", model.GetVyosNamedParentPath(), hasErr)
	}

	if !ret {
		return cruderrors.WrapIntoResourceNotFoundError(model, hasErr)
	}

	response, getErr := c.Get(ctx, model.GetVyosPath())
	// Error after successful client.Has call should mean empty resource
	var nfErr clienterrors.NotFoundError
	if errors.As(getErr, &nfErr) {
		err := helpers.UnmarshalVyos(ctx, make(map[string]any), model)
		if err != nil {
			return fmt.Errorf("empty unmarshal response: %w", err)
		}
		return nil
	}
	if getErr != nil {
		return getErr
	}

	// Populate full model config
	if responseAssrt, ok := response.(map[string]any); ok {
		err := helpers.UnmarshalVyos(ctx, responseAssrt, model)
		if err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	} else {
		return fmt.Errorf("unknown api response: %#v", response)
	}

	return nil
}
