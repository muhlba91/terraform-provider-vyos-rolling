package crud

import (
	"context"
	"math/big"
	"net/http"
	"os"
	"testing"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/provider/data"
	fw4res "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/autogen/named/firewall/ipv4-name/resourcemodel"
	polalres "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/autogen/named/policy/access-list/resourcemodel"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/tests/api"
	"github.com/go-test/deep"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
)

// TestCrudReadSuccess test CRUD helper: Read
func TestCrudReadSuccess(t *testing.T) {
	ctx := tflogtest.RootLogger(context.Background(), os.Stdout)

	// When Mock API Server
	address := "localhost:50012"
	uri := "/retrieve"
	apiKey := "test-key"
	srv := &http.Server{
		Addr: address,
	}

	eList := api.NewExchangeList()

	// Resource exists check API call
	eList.Add().Expect(
		"/retrieve",
		apiKey,
		`{"op":"exists","path":["firewall","ipv4","name","TestCrudReadSuccess"]}`,
	).Response(
		200,
		`{
			"success": true,
			"data": true,
			"error": null
		}`,
	)

	e1 := eList.Add()
	e1.Expect(
		uri,
		apiKey,
		`{"op":"showConfig","path":["firewall","ipv4","name","TestCrudReadSuccess"]}`,
	).Response(
		200,
		`{
				"success": true, "data": {
					"default-action": "reject",
					"default-log": {},
					"description": "Managed by terraform",
					"rule": {
						"1": {"action": "accept", "description": "Allow established connections", "protocol": "all", "state": "established"},
						"2": {"action": "accept", "description": "Allow related connections", "protocol": "all", "state": "related"},
						"3": {"action": "drop", "description": "Disallow invalid packets", "log": {}, "protocol": "all", "state": "invalid"},
						"1000": {"action": "accept", "description": "Allow http outgoing traffic", "destination": {"group": {"port-group": "Web"}}, "protocol": "tcp"}
					}
				},
				"error": null
			}`,
	)

	address = api.Server(srv, eList)

	// When API Client
	client := client.NewClient(ctx, "http://"+address, apiKey, "test-agent", true)

	// With Policy Access List
	model := &fw4res.FirewallIPvfourName{
		SelfIdentifier: &fw4res.FirewallIPvfourNameSelfIdentifier{FirewallIPvfourName: basetypes.NewStringValue("TestCrudReadSuccess")},
	}

	// Should
	modelShould := &fw4res.FirewallIPvfourName{
		SelfIdentifier: &fw4res.FirewallIPvfourNameSelfIdentifier{FirewallIPvfourName: basetypes.NewStringValue("TestCrudReadSuccess")},

		LeafFirewallIPvfourNameDefaultAction: basetypes.NewStringValue("reject"),
		LeafFirewallIPvfourNameDefaultLog:    basetypes.NewBoolValue(true),
		LeafFirewallIPvfourNameDescrIPtion:   basetypes.NewStringValue("Managed by terraform"),
		ExistsTagFirewallIPvfourNameRule:     true,
	}

	err := read(ctx, client, model)
	if err != nil {
		t.Errorf("read failed: %v", err)
	}

	deep.CompareUnexportedFields = true
	deep.LogErrors = true
	diff := deep.Equal(model, modelShould)
	if diff != nil {
		t.Errorf("compare failed: %v", diff)
	}

	// Validate API calls
	if len(eList.Unmatched()) > 0 {
		t.Logf("Total matched exchanges: %d", len(eList.Matched()))
		t.Errorf("Total unmatched exchanges: %d", len(eList.Unmatched()))
		t.Errorf("Next expected exchange match:\n%s", eList.Unmatched()[0].Sexpect())
		t.Errorf("Received request:\n%s", eList.Failed())
	}
}

// TestCrudReadEmptyGlobalResource tests that an empty resource response from API is checked correctly
//
// curl -k --location --request POST "https://$VYOS_HOST/retrieve" --form key="$VYOS_KEY" --form data='{"op":"showConfig","path":["policy","access-list","2"]}'
//
//	Simulated resource:
//	resource "vyos_policy_access_list" "name" {
//		access_list_id = 42
//	}
func TestCrudReadEmptyGlobalResource(t *testing.T) {
	ctx := tflogtest.RootLogger(context.Background(), os.Stdout)

	// When Mock API Server
	address := "localhost:50013"
	apiKey := "test-key"
	srv := &http.Server{
		Addr: address,
	}

	eList := api.NewExchangeList()

	// Resource exists check API call
	eList.Add().Expect(
		"/retrieve",
		apiKey,
		`{"op":"exists","path":["policy","access-list","42"]}`,
	).Response(
		200,
		`{
			"success": true,
			"data": true,
			"error": null
		}`,
	)

	// Initial retrieve request
	e1 := eList.Add()
	e1.Expect(
		"/retrieve",
		apiKey,
		`{"op":"showConfig","path":["policy","access-list","42"]}`,
	).Response(
		400,
		`{"success": false, "error": "Configuration under specified path is empty\n", "data": null}`,
	)

	address = api.Server(srv, eList)

	// When API Client
	client := client.NewClient(ctx, "http://"+address, apiKey, "test-agent", true)

	// With Policy Access List
	model := &polalres.PolicyAccessList{
		SelfIdentifier: &polalres.PolicyAccessListSelfIdentifier{PolicyAccessList: basetypes.NewNumberValue(big.NewFloat(42))},
	}

	// Should
	modelShould := &polalres.PolicyAccessList{
		SelfIdentifier: &polalres.PolicyAccessListSelfIdentifier{PolicyAccessList: basetypes.NewNumberValue(big.NewFloat(42))},
	}

	// Execute test
	err := read(ctx, client, model)
	if err != nil {
		t.Errorf("read failed: %v", err)
	}

	deep.CompareUnexportedFields = true
	deep.LogErrors = true
	diff := deep.Equal(model, modelShould)
	if diff != nil {
		t.Errorf("compare failed: %v", diff)
	}

	// Validate API calls
	if len(eList.Unmatched()) > 0 {
		t.Logf("Total matched exchanges: %d", len(eList.Matched()))
		t.Errorf("Total unmatched exchanges: %d", len(eList.Unmatched()))
		t.Errorf("Next expected exchange match:\n%s", eList.Unmatched()[0].Sexpect())
		t.Errorf("Received request:\n%s", eList.Failed())
	}
}

type testReadResource struct {
	model        helpers.VyosTopResourceDataModel
	client       *client.Client
	providerData data.ProviderData
}

func (r *testReadResource) GetModel() helpers.VyosTopResourceDataModel {
	return r.model
}

func (r *testReadResource) GetClient() *client.Client {
	return r.client
}

func (r *testReadResource) GetProviderConfig() data.ProviderData {
	return r.providerData
}

func TestCrudReadRemoveMissingOnRefresh(t *testing.T) {
	ctx := tflogtest.RootLogger(context.Background(), os.Stdout)

	address := "localhost:50014"
	apiKey := "test-key"
	srv := &http.Server{
		Addr: address,
	}

	eList := api.NewExchangeList()

	eList.Add().Expect(
		"/retrieve",
		apiKey,
		`{"op":"exists","path":["firewall","ipv4","name","TestCrudReadRemoveMissingOnRefresh"]}`,
	).Response(
		200,
		`{"success": true, "data": false, "error": null}`,
	)

	for _, path := range []string{
		`["firewall","ipv4","name","TestCrudReadRemoveMissingOnRefresh"]`,
		`["firewall","ipv4","name"]`,
		`["firewall","ipv4"]`,
		`["firewall"]`,
	} {
		eList.Add().Expect(
			"/retrieve",
			apiKey,
			`{"op":"showConfig","path":`+path+`}`,
		).Response(
			400,
			`{"success": false, "error": "Configuration under specified path is empty\n", "data": null}`,
		)
	}

	address = api.Server(srv, eList)
	client := client.NewClient(ctx, "http://"+address, apiKey, "test-agent", true)
	providerData := data.NewProviderData(client)
	providerData.Config.CrudDefaultTimeouts = 1
	providerData.Config.ReadRemoveMissingOnRefresh = true

	model := &fw4res.FirewallIPvfourName{
		SelfIdentifier: &fw4res.FirewallIPvfourNameSelfIdentifier{
			FirewallIPvfourName: basetypes.NewStringValue("TestCrudReadRemoveMissingOnRefresh"),
		},
		Timeouts: timeouts.Value{
			Object: basetypes.NewObjectNull(map[string]attr.Type{
				"create": basetypes.StringType{},
			}),
		},
	}

	state := tfsdk.State{
		Schema: resourceschema.Schema{
			Attributes: model.ResourceSchemaAttributes(ctx),
		},
	}
	diags := state.Set(ctx, model)
	if diags.HasError() {
		t.Fatalf("failed to set state: %v", diags)
	}

	req := resource.ReadRequest{
		State: state,
	}
	resp := &resource.ReadResponse{
		State: state,
	}

	Read(ctx, &testReadResource{
		model:        model,
		client:       client,
		providerData: providerData,
	}, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("read failed: %v", resp.Diagnostics)
	}

	if !resp.State.Raw.IsNull() {
		t.Fatalf("expected state to be removed on refresh")
	}

	if len(eList.Unmatched()) > 0 {
		t.Logf("Total matched exchanges: %d", len(eList.Matched()))
		t.Errorf("Total unmatched exchanges: %d", len(eList.Unmatched()))
		t.Errorf("Next expected exchange match:\n%s", eList.Unmatched()[0].Sexpect())
		t.Errorf("Received request:\n%s", eList.Failed())
	}
}
