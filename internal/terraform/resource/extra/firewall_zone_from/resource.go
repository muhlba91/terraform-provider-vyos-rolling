package firewall_zone_from

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/crud"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/provider/data"
)

func New() resource.Resource {
	return &r{model: &Model{}}
}

type r struct {
	providerData data.ProviderData
	model        *Model
}

var (
	_ resource.Resource                = &r{}
	_ resource.ResourceWithConfigure   = &r{}
	_ resource.ResourceWithImportState = &r{}
	_ helpers.VyosResource             = &r{}
)

func (x *r) GetClient() *client.Client {
	return x.providerData.Client
}

func (x *r) GetModel() helpers.VyosTopResourceDataModel {
	return x.model
}

func (x *r) GetProviderConfig() data.ProviderData {
	return x.providerData
}

func (x *r) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_zone_from"
}

func (x *r) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Firewall\n⯯\nZone-policy\n⯯\n**Zone from which to filter traffic**\n",
		Attributes:          x.model.ResourceSchemaAttributes(ctx),
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

func (x *r) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (x *r) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	crud.Create(ctx, x, req, resp)
}

func (x *r) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	crud.Read(ctx, x, req, resp)
}

func (x *r) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	crud.Update(ctx, x, req, resp)
}

func (x *r) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	crud.Delete(ctx, x, req, resp)
}
