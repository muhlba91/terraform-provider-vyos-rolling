package resourcemodel

import (
	"context"
	"fmt"
	"sort"
	"time"

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

	zonePath := append([]string(nil), o.GetVyosPath()...)
	identifierCopy := cloneFirewallZoneIdentifier(o.SelfIdentifier)
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

	missing := append([]string(nil), skipped...)
	o.TagFirewallZoneFrom = filtered

	return helpers.PlanAdjustment{
		Restore: func() {
			o.TagFirewallZoneFrom = originalFrom
		},
		PostApply: func(postCtx context.Context) error {
			return reapplyFirewallZoneReferences(postCtx, c, zonePath, identifierCopy, originalFrom, missing)
		},
	}, nil
}

const firewallZoneReferencePollInterval = 1 * time.Second

func cloneFirewallZoneIdentifier(src *FirewallZoneSelfIdentifier) *FirewallZoneSelfIdentifier {
	if src == nil {
		return nil
	}
	clone := *src
	return &clone
}

func reapplyFirewallZoneReferences(
	ctx context.Context,
	c *client.Client,
	zonePath []string,
	identifier *FirewallZoneSelfIdentifier,
	desired map[string]*FirewallZoneFrom,
	missing []string,
) error {
	if len(missing) == 0 {
		return nil
	}

	if identifier == nil {
		tools.Warn(ctx, "firewall zone identifier missing, cannot reattach skipped references")
		return nil
	}

	zoneName := identifier.FirewallZone.ValueString()
	tools.Info(ctx, "waiting for referenced firewall zones before reattaching from blocks", map[string]interface{}{
		"zone":         zoneName,
		"missing_from": missing,
	})

	pending := append([]string(nil), missing...)
	var err error
	if pending, err = missingFirewallZoneReferences(ctx, c, pending); err != nil {
		return err
	}

	if len(pending) > 0 {
		ticker := time.NewTicker(firewallZoneReferencePollInterval)
		defer ticker.Stop()

		for len(pending) > 0 {
			select {
			case <-ctx.Done():
				tools.Warn(ctx, "context ended before referenced firewall zones existed; leaving from references detached", map[string]interface{}{
					"zone":         zoneName,
					"missing_from": pending,
				})
				return nil
			case <-ticker.C:
				var err error
				pending, err = missingFirewallZoneReferences(ctx, c, pending)
				if err != nil {
					return err
				}
			}
		}
	}

	payload := &FirewallZone{
		SelfIdentifier:      identifier,
		TagFirewallZoneFrom: desired,
	}
	vyosData, err := helpers.MarshalVyos(ctx, payload)
	if err != nil {
		return fmt.Errorf("marshalling firewall zone for reattach: %w", err)
	}

	path := zonePath
	if len(path) == 0 {
		path = payload.GetVyosPath()
	}

	ops := helpers.GenerateVyosOps(ctx, path, vyosData)
	c.StageSet(ctx, path, ops)
	response, err := c.CommitChanges(ctx, path)
	if err != nil {
		return fmt.Errorf("unable to reattach firewall zone references: %w", err)
	}
	if response != nil {
		tools.Warn(ctx, "reapplyFirewallZoneReferences received non-nil response", map[string]interface{}{"response": response})
	}

	tools.Info(ctx, "firewall zone references reattached", map[string]interface{}{
		"zone": zoneName,
	})
	return nil
}

func missingFirewallZoneReferences(ctx context.Context, c *client.Client, names []string) ([]string, error) {
	remaining := make([]string, 0, len(names))
	for _, name := range names {
		exists, err := c.Has(ctx, []string{"firewall", "zone", name})
		if err != nil {
			return nil, fmt.Errorf("checking referenced zone %q during post-apply: %w", name, err)
		}
		if !exists {
			remaining = append(remaining, name)
		}
	}
	return remaining, nil
}
