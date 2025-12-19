package data

import (
	"context"
	"regexp"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// CtxMutilator changes the context object and returns the new value
type CtxMutilator func(context.Context) context.Context

/*
NewProviderData sets defaults
*/
func NewProviderData(c *client.Client) ProviderData {
	return ProviderData{
		Client: c,
		Config: Config{
			CrudSkipCheckParentBeforeCreate: false,
			CrudSkipExistingResourceCheck:   false,
			CrudSkipCheckChildBeforeDelete:  false,
			ManualBindingOverrides:          map[string]string{},
		},
	}
}

// CtxMutilators is separated out for testing convenience
// only, do not use outside this package
func CtxMutilators(apiEndpoint, apiKey string) []CtxMutilator {
	// NOTE: apiEndpoint and apiKey may contain characters that are special in
	// regular expressions (for example `+`, `?`, `[` etc.). Passing them
	// directly to regexp.MustCompile can cause a panic if they form an
	// invalid pattern. To avoid crashing the provider during Configure,
	// we always quote them before compiling.

	maskedKey := regexp.QuoteMeta(apiKey)
	maskedEndpoint := regexp.QuoteMeta(apiEndpoint)

	return []CtxMutilator{
		func(ctx context.Context) context.Context {
			if maskedKey == "" {
				return ctx
			}
			return tflog.MaskLogRegexes(ctx, regexp.MustCompile(maskedKey))
		},
		func(ctx context.Context) context.Context {
			if maskedEndpoint == "" {
				return ctx
			}
			return tflog.MaskLogRegexes(ctx, regexp.MustCompile(maskedEndpoint))
		},
	}
}
