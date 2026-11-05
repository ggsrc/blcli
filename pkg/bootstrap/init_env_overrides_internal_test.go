package bootstrap

import (
	"reflect"
	"testing"
)

func TestIgnoredDexClientSecretEnvNames(t *testing.T) {
	env := map[string]string{
		"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET":              "a",
		"BLCLI_ARGOCD_PRD_DEX_GITHUB_CLIENT_SECRET":          "b",
		"BLCLI_ARGOCD_HELLO_WORLD_GITHUB_AUTH_CLIENT_SECRET": "c",
		"ARGOCD_GITHUB_AUTH_CLIENT_SECRET":                   "d",
		"BLCLI_ARGOCD_STG_DEX_GITHUB_CLIENT_SECRET":          "", // empty: not set
		"BLCLI_ARGOCD_PRD_DEX_GITHUB_CLIENT_ID":              "id",
		"DEX_GITHUB_CLIENT_SECRET":                           "install-time only, not an override",
	}
	got := ignoredDexClientSecretEnvNames(env)
	want := []string{
		"ARGOCD_GITHUB_AUTH_CLIENT_SECRET",
		"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET",
		"BLCLI_ARGOCD_HELLO_WORLD_GITHUB_AUTH_CLIENT_SECRET",
		"BLCLI_ARGOCD_PRD_DEX_GITHUB_CLIENT_SECRET",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ignoredDexClientSecretEnvNames() = %v, want %v", got, want)
	}
}
