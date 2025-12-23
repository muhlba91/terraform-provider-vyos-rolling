//go:build manual_firewall_zone_override
// +build manual_firewall_zone_override

// Package resourcemodel keeps manual firewall zone overrides in a location the
// generator will not delete. Files here keep a build tag so Go tooling skips
// them; the Makefile copies the content into
// internal/terraform/resource/autogen and strips the build tag so production
// builds always include the overrides.
package resourcemodel
