package helpers_test

import (
	"context"
	"testing"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers"
	ntpResourceModel "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/autogen/global/service/ntp/resourcemodel"
	qoscake "github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/resource/autogen/named/qos/policy-cake/resourcemodel"
	"github.com/go-test/deep"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestMarshalToVyosNamedResMergedIntoGlobal
// Verify that what would have been a named resource that has to be merged
// in under a global resource results in a map of config parameters and works as expected
func TestUnmarshalToVyosNamedResMergedIntoGlobal(t *testing.T) {
	ctx := context.Background()

	has := map[string]any{
		"server": map[string]any{
			"no.pool.ntp.org": map[string]any{
				"pool":   map[string]any{},
				"prefer": map[string]any{},
			},
		},
	}

	want := &ntpResourceModel.ServiceNtp{
		// ID: basetypes.NewStringNull(),
		// Timeouts:                    basetypes,
		// LeafServiceNtpInterface:     {},
		// LeafServiceNtpListenAddress: {},
		// LeafServiceNtpVrf:           {},
		// LeafServiceNtpLeapSecond:    {},
		TagServiceNtpServer: map[string]*ntpResourceModel.ServiceNtpServer{
			"no.pool.ntp.org": {
				LeafServiceNtpServerNoselect:   basetypes.NewBoolValue(false),
				LeafServiceNtpServerNts:        basetypes.NewBoolValue(false),
				LeafServiceNtpServerPool:       basetypes.NewBoolValue(true),
				LeafServiceNtpServerPrefer:     basetypes.NewBoolValue(true),
				LeafServiceNtpServerPtp:        basetypes.NewBoolValue(false),
				LeafServiceNtpServerInterleave: basetypes.NewBoolValue(false),
			},
		},
		// NodeServiceNtpAllowClient: nil,
		ExistsNodeServiceNtpPtp: false,
	}

	got := &ntpResourceModel.ServiceNtp{}

	err := helpers.UnmarshalVyos(ctx, has, got)
	if err != nil {
		t.Errorf("ERROR: %v", err)
	}

	// Diff results
	diff := deep.Equal(got, want)
	if diff != nil {
		t.Errorf("Want: %v", want)
		t.Errorf("Got: %v", got)
		t.Errorf("compare failed: %v", diff)
	}
}

func TestUnmarshalStringFromNestedTagNode(t *testing.T) {
	ctx := context.Background()

	data := map[string]any{
		"flow-isolation": map[string]any{
			"dual-src-host": map[string]any{
				"nat": map[string]any{},
			},
		},
		"flow-isolation-nat": map[string]any{},
	}

	got := &qoscake.QosPolicyCake{}
	err := helpers.UnmarshalVyos(ctx, data, got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.LeafQosPolicyCakeFlowIsolation.IsNull() {
		t.Fatalf("flow isolation remained null")
	}
	if got.LeafQosPolicyCakeFlowIsolation.ValueString() != "dual-src-host" {
		t.Fatalf("unexpected flow isolation value: %s", got.LeafQosPolicyCakeFlowIsolation.ValueString())
	}
	if got.LeafQosPolicyCakeFlowIsolationNat.IsNull() || !got.LeafQosPolicyCakeFlowIsolationNat.ValueBool() {
		t.Fatalf("flow isolation nat flag was not set")
	}
}
