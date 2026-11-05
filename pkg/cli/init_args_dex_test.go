package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/renderer"
)

var _ = Describe("Dex client secret in generated args", func() {
	It("removes client secret values and keeps references", func() {
		cfg := &ArgsConfig{
			Kubernetes: renderer.KubernetesSection{
				Projects: []renderer.ProjectData{
					{
						Name: "prd",
						Components: []renderer.ComponentData{
							{
								Name: "2-argocd",
								Parameters: map[string]interface{}{
									"namespace":             "argocd",
									"DexGitHubClientSecret": "XXXXXXXX",
									"dex-config": map[string]interface{}{
										"connectors": []interface{}{
											map[string]interface{}{
												"type": "github",
												"config": map[string]interface{}{
													"clientID":     "client-id",
													"clientSecret": "XXXXXXXX",
												},
											},
											map[string]interface{}{
												"type": "github",
												"config": map[string]interface{}{
													"clientID":        "other-id",
													"clientSecretRef": "$my-secret:clientSecret",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		removed := stripDexClientSecrets(cfg)
		Expect(removed).To(Equal([]string{
			"prd/2-argocd.DexGitHubClientSecret",
			"prd/2-argocd.dex-config.connectors[].config.clientSecret",
		}))

		params := cfg.Kubernetes.Projects[0].Components[0].Parameters
		Expect(params).NotTo(HaveKey("DexGitHubClientSecret"))
		Expect(params).To(HaveKeyWithValue("namespace", "argocd"))
		connectors := params["dex-config"].(map[string]interface{})["connectors"].([]interface{})
		first := connectors[0].(map[string]interface{})["config"].(map[string]interface{})
		Expect(first).NotTo(HaveKey("clientSecret"))
		Expect(first).To(HaveKeyWithValue("clientID", "client-id"))
		second := connectors[1].(map[string]interface{})["config"].(map[string]interface{})
		Expect(second).To(HaveKeyWithValue("clientSecretRef", "$my-secret:clientSecret"))
	})

	It("reports nothing when there is no secret", func() {
		cfg := &ArgsConfig{}
		Expect(stripDexClientSecrets(cfg)).To(BeEmpty())
	})
})
