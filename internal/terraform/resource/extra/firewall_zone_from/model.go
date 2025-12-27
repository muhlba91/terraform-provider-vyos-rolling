package firewall_zone_from

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	zone_resourcemodel "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/autogen/named/firewall/zone/resourcemodel"
)

var _ helpers.VyosTopResourceDataModel = &Model{}

type Identifier struct {
	From types.String `tfsdk:"from" vyos:"-,self-id"`
	Zone types.String `tfsdk:"zone" vyos:"-,self-id"`
}

// Model implements the top-level resource model for firewall zone "from" entries.
// VyOS path: firewall zone <zone> from <from>
type Model struct {
	ID       types.String   `tfsdk:"id" vyos:"-,tfsdk-id"`
	Timeouts timeouts.Value `tfsdk:"timeouts" vyos:"-,timeout"`

	SelfIdentifier *Identifier `tfsdk:"identifier" vyos:"-,self-id"`

	NodeFirewall *zone_resourcemodel.FirewallZoneFromFirewall `tfsdk:"firewall" vyos:"firewall,omitempty"`
}

func (o *Model) SetID(id []string) {
	o.ID = basetypes.NewStringValue(strings.Join(id, "__"))
}

func (o *Model) GetTimeouts() timeouts.Value {
	return o.Timeouts
}

func (o *Model) IsGlobalResource() bool {
	return false
}

func (o *Model) GetVyosParentPath() []string {
	return []string{"firewall"}
}

func (o *Model) GetVyosNamedParentPath() []string {
	if o.SelfIdentifier == nil {
		return []string{"firewall", "zone"}
	}
	return []string{"firewall", "zone", o.SelfIdentifier.Zone.ValueString()}
}

func (o *Model) GetVyosPath() []string {
	if o.ID.ValueString() != "" {
		return strings.Split(o.ID.ValueString(), "__")
	}

	if o.SelfIdentifier == nil {
		return append(o.GetVyosParentPath(), "zone")
	}

	return []string{
		"firewall",
		"zone",
		o.SelfIdentifier.Zone.ValueString(),
		"from",
		o.SelfIdentifier.From.ValueString(),
	}
}

func (o Model) ResourceSchemaAttributes(ctx context.Context) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Resource ID, full vyos path to the resource with each field separated by dunder (`__`).",
		},
		"identifier": schema.SingleNestedAttribute{
			Required: true,
			Attributes: map[string]schema.Attribute{
				"zone": schema.StringAttribute{
					Required: true,
					MarkdownDescription: `Zone-policy

    |  Format  |  Description  |
    |----------|---------------|
    |  txt     |  Zone name    |
`,
					Description: `Zone-policy

    |  Format  |  Description  |
    |----------|---------------|
    |  txt     |  Zone name    |
`,
					PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
					Validators: []validator.String{
						stringvalidator.All(
							helpers.StringNot(
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^.*__.*$`),
									"double underscores in zone, conflicts with the internal resource id",
								),
							),
							stringvalidator.RegexMatches(
								regexp.MustCompile(`^[.:a-zA-Z0-9-_/]+$`),
								"illegal character in  zone, value must match: ^[.:a-zA-Z0-9-_/]+$",
							),
						),
					},
				},
				"from": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Zone from which to filter traffic\n",
					Description:         "Zone from which to filter traffic\n",
					PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					Validators: []validator.String{
						stringvalidator.All(
							helpers.StringNot(
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^.*__.*$`),
									"double underscores in from, conflicts with the internal resource id",
								),
							),
							stringvalidator.RegexMatches(
								regexp.MustCompile(`^[.:a-zA-Z0-9-_/]+$`),
								"illegal character in from, value must match: ^[.:a-zA-Z0-9-_/]+$",
							),
						),
					},
				},
			},
		},
		"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true}),
		"firewall": schema.SingleNestedAttribute{
			Optional:            true,
			Attributes:          zone_resourcemodel.FirewallZoneFromFirewall{}.ResourceSchemaAttributes(ctx),
			MarkdownDescription: "Firewall options\n\n",
			Description:         "Firewall options\n\n",
		},
	}
}
