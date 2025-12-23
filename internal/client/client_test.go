package client

import "testing"

// TODO test that certificate functionality works
//  milestone: 2
//  this is not going to be an automated test, and this file can be deleted later.
//  just test it out personally and document of how it works in the
//  provider config attr
//  - [ ] test invalid cert
//  - [ ] test skip check on invalid cert
//  - [ ] test valid cert
//  - [ ] test valid self signed cert in OS cert store

func TestBuildOperationsPrioritizesFirewallZoneMemberInterfaces(t *testing.T) {
	group := &bindingGroup{
		key: "firewall",
		resources: []*resourceBatch{
			{
				resourceID: "firewall zone WAN",
				bindingKey: "firewall",
				setOps: [][]string{
					{"firewall", "zone", "WAN", "member", "interface", "eth0"},
					{"firewall", "zone", "WAN", "default-action", "drop"},
				},
			},
			{
				resourceID: "firewall zone WAN from LOCAL",
				bindingKey: "firewall",
				setOps: [][]string{
					{"firewall", "zone", "WAN", "from", "LOCAL", "firewall", "name", "WAN-LOCAL"},
				},
			},
		},
	}

	ops := buildOperations([]*bindingGroup{group})
	wantMember := []string{"firewall", "zone", "WAN", "member", "interface", "eth0"}
	wantFrom := []string{"firewall", "zone", "WAN", "from", "LOCAL", "firewall", "name", "WAN-LOCAL"}
	memberIdx := findSetOpIndex(t, ops, wantMember)
	fromIdx := findSetOpIndex(t, ops, wantFrom)
	if memberIdx == -1 || fromIdx == -1 {
		t.Fatalf("missing expected operations: member=%d from=%d", memberIdx, fromIdx)
	}
	if memberIdx <= fromIdx {
		t.Fatalf("expected member interface ops after zone-from ops: member=%d from=%d", memberIdx, fromIdx)
	}
}

func TestIsFirewallZoneMemberInterface(t *testing.T) {
	testCases := []struct {
		name string
		path []string
		want bool
	}{
		{
			name: "member interface",
			path: []string{"firewall", "zone", "WAN", "member", "interface", "eth0"},
			want: true,
		},
		{
			name: "non firewall path",
			path: []string{"protocols", "static", "route"},
			want: false,
		},
		{
			name: "firewall but not member",
			path: []string{"firewall", "zone", "WAN", "from", "LOCAL"},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFirewallZoneMemberInterface(tc.path); got != tc.want {
				t.Fatalf("unexpected detection result: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func findSetOpIndex(t *testing.T, ops []map[string]interface{}, target []string) int {
	t.Helper()
	for idx, op := range ops {
		opType, _ := op["op"].(string)
		if opType != "set" {
			continue
		}
		path, ok := op["path"].([]string)
		if !ok {
			t.Fatalf("unexpected path type %T", op["path"])
		}
		if equalPath(path, target) {
			return idx
		}
	}
	return -1
}

func equalPath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
