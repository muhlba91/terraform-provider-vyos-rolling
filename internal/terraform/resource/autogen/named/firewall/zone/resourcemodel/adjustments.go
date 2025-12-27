package resourcemodel

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/client/clienterrors"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/tools"
)

var _ helpers.CreatePlanAdjuster = (*FirewallZone)(nil)
var _ helpers.UpdatePlanAdjuster = (*FirewallZone)(nil)

// AdjustCreatePlan removes temporary references to zones that are not yet present on
// the target router so VyOS does not reject the commit. Skipped entries are
// restored after create so a follow up apply can reattach them once peers exist.
func (o *FirewallZone) AdjustCreatePlan(ctx context.Context, c *client.Client) (helpers.PlanAdjustment, error) {
	return o.adjustMissingFromReferences(ctx, c)
}

// AdjustUpdatePlan mirrors the create behavior to cover replacement scenarios
// where referencing zones may have been removed between applies.
func (o *FirewallZone) AdjustUpdatePlan(ctx context.Context, c *client.Client, _ helpers.VyosTopResourceDataModel) (helpers.PlanAdjustment, error) {
	return o.adjustMissingFromReferences(ctx, c)
}

func (o *FirewallZone) adjustMissingFromReferences(ctx context.Context, c *client.Client) (helpers.PlanAdjustment, error) {
	if o == nil {
		return helpers.PlanAdjustment{}, nil
	}

	zoneName := ""
	if o.SelfIdentifier != nil {
		zoneName = o.SelfIdentifier.FirewallZone.ValueString()
	}

	originalFrom := o.TagFirewallZoneFrom
	filtered := make(map[string]*FirewallZoneFrom, len(originalFrom))
	skipped := make([]string, 0)

	if len(originalFrom) > 0 {
		for fromName, fromCfg := range originalFrom {
			exists, err := c.Has(ctx, []string{"firewall", "zone", fromName})
			if err != nil {
				return helpers.PlanAdjustment{}, fmt.Errorf("checking referenced zone %q: %w", fromName, err)
			}
			if !exists {
				skipped = append(skipped, fromName)
				continue
			}
			filtered[fromName] = fromCfg
		}

		if len(skipped) > 0 {
			sort.Strings(skipped)
			tools.Warn(ctx, "firewall zone references missing peers, temporarily skipping", map[string]interface{}{
				"zone":         zoneName,
				"missing_from": skipped,
			})

			if len(filtered) == 0 {
				// Clear the entire block to avoid emitting `set firewall zone <name> from`
				// without nested values, which VyOS rejects.
				o.TagFirewallZoneFrom = nil
			} else {
				o.TagFirewallZoneFrom = filtered
			}
		}
	}

	originalDefaultAction := types.String{}
	relaxedDefaultAction := false
	relaxedBecauseFromMissingOnDevice := false

	// Safety: creating/updating a zone with default-action=drop before any
	// `from` rules exist can lock out management or even router-originated traffic
	// (e.g. WAN missing `from local` prevents pings/DNS).
	if !o.LeafFirewallZoneDefaultAction.IsNull() && !o.LeafFirewallZoneDefaultAction.IsUnknown() && o.LeafFirewallZoneDefaultAction.ValueString() == "drop" {
		fromExists := false
		if zoneName != "" {
			// NOTE: `exists` returns true for empty config blocks; we need to know
			// whether there are any *actual* zone-from entries.
			data, err := c.Get(ctx, []string{"firewall", "zone", zoneName, "from"})
			switch {
			case err == nil:
				fromExists = configNonEmpty(data)
			case errorsIsNotFound(err):
				fromExists = false
			default:
				return helpers.PlanAdjustment{}, fmt.Errorf("checking firewall zone %q from entries: %w", zoneName, err)
			}
		}

		if !fromExists {
			relaxedDefaultAction = true
			relaxedBecauseFromMissingOnDevice = true
			originalDefaultAction = cloneStringValue(o.LeafFirewallZoneDefaultAction)
			// VyOS does not accept "accept" here (valid values are drop/reject).
			// To avoid lockout, temporarily omit default-action so the zone behaves
			// permissively until zone-from rules exist.
			o.LeafFirewallZoneDefaultAction = types.StringNull()
			tools.Warn(ctx, "firewall zone default action temporarily relaxed (no from rules yet)", map[string]interface{}{
				"zone":   zoneName,
				"action": originalDefaultAction.ValueString(),
			})
		}
	}

	if len(skipped) == 0 && !relaxedDefaultAction {
		return helpers.PlanAdjustment{}, nil
	}

	skippedCopy := append([]string(nil), skipped...)

	return helpers.PlanAdjustment{
		Restore: func() {
			// Restore only the temporary `from` filtering so state matches config.
			// If we relaxed default-action due to missing `from` rules on-device,
			// keep the relaxed value so Terraform state reflects what was applied.
			o.TagFirewallZoneFrom = originalFrom
			if relaxedDefaultAction && !relaxedBecauseFromMissingOnDevice {
				o.LeafFirewallZoneDefaultAction = cloneStringValue(originalDefaultAction)
			}
		},
		PostApply: func(ctx context.Context) error {
			if relaxedBecauseFromMissingOnDevice {
				// Defer restoring default-action=drop until a later apply, after
				// zone-from rules exist (managed by separate resources).
				return nil
			}
			return o.reapplyMissingFromReferences(ctx, c, skippedCopy, originalFrom, relaxedDefaultAction, originalDefaultAction)
		},
	}, nil
}

func errorsIsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nf clienterrors.NotFoundError
	return errors.As(err, &nf)
}

func configNonEmpty(data any) bool {
	if data == nil {
		return false
	}
	switch v := data.(type) {
	case map[string]interface{}:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	default:
		// Be conservative: unknown shapes might still represent a non-empty config,
		// but for lockout-safety we treat it as empty unless clearly non-empty.
		return false
	}
}

func (o *FirewallZone) reapplyMissingFromReferences(ctx context.Context, c *client.Client, skipped []string, original map[string]*FirewallZoneFrom, relaxedDefaultAction bool, originalDefaultAction types.String) error {
	needFromRestore := len(skipped) > 0
	needDefaultRestore := relaxedDefaultAction && !originalDefaultAction.IsNull() && !originalDefaultAction.IsUnknown()

	if !needFromRestore && !needDefaultRestore {
		return nil
	}

	var restorable map[string]*FirewallZoneFrom
	if needFromRestore {
		restorable = make(map[string]*FirewallZoneFrom, len(skipped))
		for _, name := range skipped {
			cfg, ok := original[name]
			if !ok || cfg == nil {
				continue
			}
			restorable[name] = cloneFirewallZoneFrom(cfg)
		}

		if len(restorable) == 0 {
			needFromRestore = false
		}
	}

	var zoneNames []string
	if needFromRestore {
		zoneNames = sortedMapKeys(restorable)
		if err := waitForReferencedFirewallZones(ctx, c, zoneNames); err != nil {
			return err
		}
	}

	patch := &firewallZoneRestorePatch{}
	if needFromRestore {
		patch.TagFirewallZoneFrom = restorable
	}
	if needDefaultRestore {
		patch.LeafFirewallZoneDefaultAction = cloneStringValue(originalDefaultAction)
	}

	vyosData, err := helpers.MarshalVyos(ctx, patch)
	if err != nil {
		return fmt.Errorf("marshal firewall zone restoration patch: %w", err)
	}

	resourcePath := o.GetVyosPath()
	c.StageSet(ctx, resourcePath, helpers.GenerateVyosOps(ctx, resourcePath, vyosData))
	resp, err := c.CommitChanges(ctx, resourcePath)
	if err != nil {
		return fmt.Errorf("commit firewall zone restoration: %w", err)
	}
	if resp != nil {
		tools.Warn(ctx, "post-apply firewall zone restoration returned non-nil response", map[string]interface{}{"response": resp})
	}

	if needFromRestore {
		tools.Info(ctx, "firewall zone references restored post-apply", map[string]interface{}{
			"zone":           o.SelfIdentifier.FirewallZone.ValueString(),
			"restored_from":  zoneNames,
			"restorable_cnt": len(restorable),
			"default_action": originalDefaultAction.ValueString(),
		})
	} else if needDefaultRestore {
		tools.Info(ctx, "firewall zone default action restored post-apply", map[string]interface{}{
			"zone":           o.SelfIdentifier.FirewallZone.ValueString(),
			"default_action": originalDefaultAction.ValueString(),
		})
	}

	return nil
}

func waitForReferencedFirewallZones(ctx context.Context, c *client.Client, zoneNames []string) error {
	if len(zoneNames) == 0 {
		return nil
	}

	pending := make(map[string]struct{}, len(zoneNames))
	for _, name := range zoneNames {
		pending[name] = struct{}{}
	}

	delay := 250 * time.Millisecond
	const maxDelay = 2 * time.Second

	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for referenced firewall zones: %w", ctx.Err())
		default:
		}

		for name := range pending {
			exists, err := c.Has(ctx, []string{"firewall", "zone", name})
			if err != nil {
				return fmt.Errorf("checking referenced firewall zone %q: %w", name, err)
			}
			if exists {
				delete(pending, name)
			}
		}

		if len(pending) == 0 {
			break
		}

		tools.Debug(ctx, "referenced firewall zones still provisioning, backing off", map[string]interface{}{
			"pending_zones": sortedMapKeys(pending),
			"sleep":         delay,
		})

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("waiting for referenced firewall zones: %w", ctx.Err())
		case <-timer.C:
		}

		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	return nil
}

type firewallZoneRestorePatch struct {
	LeafFirewallZoneDefaultAction types.String                 `tfsdk:"default_action" vyos:"default-action,omitempty"`
	TagFirewallZoneFrom           map[string]*FirewallZoneFrom `tfsdk:"from" vyos:"from,omitempty"`
}

func (firewallZoneRestorePatch) ResourceSchemaAttributes(ctx context.Context) map[string]schema.Attribute {
	return map[string]schema.Attribute{}
}

func sortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func cloneFirewallZoneFrom(src *FirewallZoneFrom) *FirewallZoneFrom {
	if src == nil {
		return nil
	}

	clone := &FirewallZoneFrom{}
	if fw := src.NodeFirewallZoneFromFirewall; fw != nil {
		clone.NodeFirewallZoneFromFirewall = &FirewallZoneFromFirewall{
			LeafFirewallZoneFromFirewallName:       cloneStringValue(fw.LeafFirewallZoneFromFirewallName),
			LeafFirewallZoneFromFirewallIPvsixName: cloneStringValue(fw.LeafFirewallZoneFromFirewallIPvsixName),
		}
	}

	return clone
}

func cloneStringValue(val types.String) types.String {
	switch {
	case val.IsNull():
		return types.StringNull()
	case val.IsUnknown():
		return types.StringUnknown()
	default:
		return types.StringValue(val.ValueString())
	}
}
