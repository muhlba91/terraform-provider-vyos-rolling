package resourcemodel

import (
	"context"
	"fmt"
	"sort"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
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
	if o == nil || len(o.TagFirewallZoneFrom) == 0 {
		return helpers.PlanAdjustment{}, nil
	}

	originalFrom := o.TagFirewallZoneFrom
	filtered := make(map[string]*FirewallZoneFrom, len(originalFrom))
	skipped := make([]string, 0)

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

	if len(skipped) == 0 {
		return helpers.PlanAdjustment{}, nil
	}

	sort.Strings(skipped)
	tools.Warn(ctx, "firewall zone references missing peers, temporarily skipping", map[string]interface{}{
		"zone":         o.SelfIdentifier.FirewallZone.ValueString(),
		"missing_from": skipped,
	})

	o.TagFirewallZoneFrom = filtered

	return helpers.PlanAdjustment{
		Restore: func() {
			o.TagFirewallZoneFrom = originalFrom
		},
	}, nil
}
