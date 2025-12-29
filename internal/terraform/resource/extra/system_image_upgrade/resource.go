package system_image_upgrade

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/provider/data"
)

func New() resource.Resource {
	return &r{}
}

type r struct {
	providerData data.ProviderData
}

var (
	_ resource.Resource              = &r{}
	_ resource.ResourceWithConfigure = &r{}
)

type model struct {
	ID types.String `tfsdk:"id"`

	TargetName types.String `tfsdk:"target_name"`
	ISOURL     types.String `tfsdk:"iso_url"`

	BackupConfigFile types.String `tfsdk:"backup_config_file"`
	BackupBefore     types.Bool   `tfsdk:"backup_before"`

	RebootAfter types.Bool `tfsdk:"reboot_after"`
	RebootPath  types.List `tfsdk:"reboot_path"`

	DeleteOnDestroy types.Bool `tfsdk:"delete_on_destroy"`

	Installed    types.Bool   `tfsdk:"installed"`
	ShowOutput   types.String `tfsdk:"show_output"`
	LastResponse types.String `tfsdk:"last_response"`
}

func (x *r) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_image_upgrade"
}

func (x *r) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Operational\n⯯\n**Install a pinned VyOS ISO via /image and optionally reboot**\n\nThis resource is intentionally imperative (it performs actions). Best practice is to run it in a separate apply (e.g. `-target`) because rebooting will interrupt other resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},

			"target_name": schema.StringAttribute{
				MarkdownDescription: "The expected image name/version string to check for in `show system image` output (e.g. `1.5-rolling-YYYYMMDDHHMM`).",
				Required:            true,
			},
			"iso_url": schema.StringAttribute{
				MarkdownDescription: "HTTP(S) URL of the ISO to install (passed to the VyOS `/image` endpoint).",
				Required:            true,
			},

			"backup_before": schema.BoolAttribute{
				MarkdownDescription: "Whether to call `/config-file` `save` before installing the image.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"backup_config_file": schema.StringAttribute{
				MarkdownDescription: "Optional file argument for `/config-file` save. If empty, VyOS saves to `/config/config.boot`. You may also try an `scp://...` URL if your VyOS build supports it.",
				Optional:            true,
			},

			"reboot_after": schema.BoolAttribute{
				MarkdownDescription: "Whether to call `/reboot` after the image install.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"reboot_path": schema.ListAttribute{
				MarkdownDescription: "Arguments passed as the `path` array to the `/reboot` endpoint. The docs use `[\"now\"]`.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default: listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{
					types.StringValue("now"),
				})),
			},

			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "If true, deletes `target_name` via `/image` when this resource is destroyed.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"installed": schema.BoolAttribute{
				MarkdownDescription: "Whether the target image is present according to `/show` `system image`.",
				Computed:            true,
			},
			"show_output": schema.StringAttribute{
				MarkdownDescription: "Raw output from `/show` `system image` (useful for debugging).",
				Computed:            true,
			},
			"last_response": schema.StringAttribute{
				MarkdownDescription: "Last response text from the API action (backup/install/delete).",
				Computed:            true,
			},
		},
	}
}

func (x *r) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerDataValue, ok := req.ProviderData.(data.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected data.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	x.providerData = providerDataValue
}

func (x *r) client() *client.Client {
	return x.providerData.Client
}

func (x *r) withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeoutMins := x.providerData.Config.CrudDefaultTimeouts
	if timeoutMins <= 0 {
		timeoutMins = 15
	}
	return context.WithTimeout(ctx, time.Duration(timeoutMins)*time.Minute)
}

func containsImage(showOutput, target string) bool {
	showOutput = strings.ToLower(showOutput)
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	return strings.Contains(showOutput, target)
}

func (x *r) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = x.providerData.CtxMutilatorRun(ctx)
	ctx, cancel := x.withDefaultTimeout(ctx)
	defer cancel()

	out, err := x.client().Show(ctx, []string{"system", "image"})
	if err != nil {
		resp.Diagnostics.AddError("VyOS API error", err.Error())
		return
	}

	state.ShowOutput = types.StringValue(out)
	state.Installed = types.BoolValue(containsImage(out, state.TargetName.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (x *r) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = x.providerData.CtxMutilatorRun(ctx)
	ctx, cancel := x.withDefaultTimeout(ctx)
	defer cancel()

	target := plan.TargetName.ValueString()
	isoURL := plan.ISOURL.ValueString()
	if strings.TrimSpace(target) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("target_name"), "Missing target_name", "target_name is required")
		return
	}
	if strings.TrimSpace(isoURL) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("iso_url"), "Missing iso_url", "iso_url is required")
		return
	}

	out, err := x.client().Show(ctx, []string{"system", "image"})
	if err != nil {
		resp.Diagnostics.AddError("VyOS API error", err.Error())
		return
	}

	installed := containsImage(out, target)
	lastResp := ""

	if !installed {
		if plan.BackupBefore.ValueBool() {
			backupResp, err := x.client().ConfigFileSave(ctx, plan.BackupConfigFile.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("VyOS API error (config backup)", err.Error())
				return
			}
			lastResp = backupResp
		}

		installResp, err := x.client().ImageAdd(ctx, isoURL)
		if err != nil {
			resp.Diagnostics.AddError("VyOS API error (image install)", err.Error())
			return
		}
		lastResp = installResp

		out, err = x.client().Show(ctx, []string{"system", "image"})
		if err != nil {
			resp.Diagnostics.AddError("VyOS API error", err.Error())
			return
		}
		installed = containsImage(out, target)
	}

	if plan.RebootAfter.ValueBool() {
		var rebootPath []string
		if !plan.RebootPath.IsNull() && !plan.RebootPath.IsUnknown() {
			resp.Diagnostics.Append(plan.RebootPath.ElementsAs(ctx, &rebootPath, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		if len(rebootPath) == 0 {
			rebootPath = []string{"now"}
		}
		if err := x.client().Reboot(ctx, rebootPath); err != nil {
			resp.Diagnostics.AddError("VyOS API error (reboot)", err.Error())
			return
		}
		lastResp = strings.TrimSpace(lastResp + "\nReboot initiated")
	}

	plan.ID = types.StringValue(target)
	plan.ShowOutput = types.StringValue(out)
	plan.Installed = types.BoolValue(installed)
	plan.LastResponse = types.StringValue(lastResp)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (x *r) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = x.providerData.CtxMutilatorRun(ctx)
	ctx, cancel := x.withDefaultTimeout(ctx)
	defer cancel()

	target := plan.TargetName.ValueString()
	isoURL := plan.ISOURL.ValueString()
	if strings.TrimSpace(target) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("target_name"), "Missing target_name", "target_name is required")
		return
	}
	if strings.TrimSpace(isoURL) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("iso_url"), "Missing iso_url", "iso_url is required")
		return
	}

	out, err := x.client().Show(ctx, []string{"system", "image"})
	if err != nil {
		resp.Diagnostics.AddError("VyOS API error", err.Error())
		return
	}

	installed := containsImage(out, target)
	lastResp := ""

	if !installed {
		if plan.BackupBefore.ValueBool() {
			backupResp, err := x.client().ConfigFileSave(ctx, plan.BackupConfigFile.ValueString())
			if err != nil {
				resp.Diagnostics.AddError("VyOS API error (config backup)", err.Error())
				return
			}
			lastResp = backupResp
		}

		installResp, err := x.client().ImageAdd(ctx, isoURL)
		if err != nil {
			resp.Diagnostics.AddError("VyOS API error (image install)", err.Error())
			return
		}
		lastResp = installResp

		out, err = x.client().Show(ctx, []string{"system", "image"})
		if err != nil {
			resp.Diagnostics.AddError("VyOS API error", err.Error())
			return
		}
		installed = containsImage(out, target)
	}

	if plan.RebootAfter.ValueBool() {
		var rebootPath []string
		if !plan.RebootPath.IsNull() && !plan.RebootPath.IsUnknown() {
			resp.Diagnostics.Append(plan.RebootPath.ElementsAs(ctx, &rebootPath, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		if len(rebootPath) == 0 {
			rebootPath = []string{"now"}
		}
		if err := x.client().Reboot(ctx, rebootPath); err != nil {
			resp.Diagnostics.AddError("VyOS API error (reboot)", err.Error())
			return
		}
		lastResp = strings.TrimSpace(lastResp + "\nReboot initiated")
	}

	plan.ID = types.StringValue(target)
	plan.ShowOutput = types.StringValue(out)
	plan.Installed = types.BoolValue(installed)
	plan.LastResponse = types.StringValue(lastResp)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (x *r) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.DeleteOnDestroy.ValueBool() {
		return
	}

	ctx = x.providerData.CtxMutilatorRun(ctx)
	ctx, cancel := x.withDefaultTimeout(ctx)
	defer cancel()

	name := state.TargetName.ValueString()
	if strings.TrimSpace(name) == "" {
		return
	}

	_, err := x.client().ImageDelete(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("VyOS API error (image delete)", err.Error())
		return
	}
}
