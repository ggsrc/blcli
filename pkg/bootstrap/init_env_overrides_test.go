package bootstrap_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap"
	"blcli/pkg/renderer"
)

var _ = Describe("Init env overrides", func() {
	It("should override terraform global values from env file", func() {
		workspace, err := setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())
		defer cleanupTestWorkspace(workspace)

		envPath := filepath.Join(workspace, "override.env")
		err = os.WriteFile(envPath, []byte(""+
			"BLCLI_TERRAFORM_ORGANIZATION_ID=999999999999\n"+
			"BLCLI_TERRAFORM_BILLING_ACCOUNT_ID=AA-BB-CC\n"), 0644)
		Expect(err).NotTo(HaveOccurred())

		base := renderer.ArgsData{
			renderer.FieldTerraform: map[string]interface{}{
				renderer.FieldGlobal: map[string]interface{}{
					"OrganizationID":   "123456789012",
					"BillingAccountID": "01ABCD-2EFGH3-4IJKL5",
				},
			},
		}

		merged, err := bootstrap.ApplyInitEnvOverrides(base, []string{envPath})
		Expect(err).NotTo(HaveOccurred())

		tf := merged[renderer.FieldTerraform].(map[string]interface{})
		global := tf[renderer.FieldGlobal].(map[string]interface{})
		Expect(global["OrganizationID"]).To(Equal("999999999999"))
		Expect(global["BillingAccountID"]).To(Equal("AA-BB-CC"))
	})

	It("should override argocd component parameters for all projects", func() {
		workspace, err := setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())
		defer cleanupTestWorkspace(workspace)

		envPath := filepath.Join(workspace, "argocd.env")
		err = os.WriteFile(envPath, []byte(""+
			"BLCLI_ARGOCD_GIT_REPOSITORY_URL=git@github.com:example/infra-gitops.git\n"+
			"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_ID=client-id\n"+
			"BLCLI_ARGOCD_DEX_GITHUB_CLIENT_SECRET=client-secret\n"+
			"BLCLI_ARGOCD_DEX_GITHUB_ORGS=org-a,org-b\n"), 0644)
		Expect(err).NotTo(HaveOccurred())

		base := renderer.ArgsData{
			"kubernetes": map[string]interface{}{
				renderer.FieldProjects: []interface{}{
					map[string]interface{}{
						renderer.FieldName: "prd",
						renderer.FieldComponents: []interface{}{
							map[string]interface{}{
								renderer.FieldName: "2-argocd",
								"parameters": map[string]interface{}{
									"namespace": "argocd",
								},
							},
						},
					},
					map[string]interface{}{
						renderer.FieldName: "stg",
						renderer.FieldComponents: []interface{}{
							map[string]interface{}{
								renderer.FieldName: "2-argocd",
								"parameters": map[string]interface{}{
									"namespace": "argocd",
								},
							},
						},
					},
				},
			},
		}

		merged, err := bootstrap.ApplyInitEnvOverrides(base, []string{envPath})
		Expect(err).NotTo(HaveOccurred())

		k8s := merged["kubernetes"].(map[string]interface{})
		projects := k8s[renderer.FieldProjects].([]interface{})
		Expect(projects).To(HaveLen(2))

		for _, project := range projects {
			projectMap := project.(map[string]interface{})
			components := projectMap[renderer.FieldComponents].([]interface{})
			component := components[0].(map[string]interface{})
			parameters := component["parameters"].(map[string]interface{})

			Expect(parameters["GitRepositoryURL"]).To(Equal("git@github.com:example/infra-gitops.git"))
			Expect(parameters["DexGitHubEnabled"]).To(Equal(true))
			Expect(parameters["DexGitHubClientID"]).To(Equal("client-id"))
			Expect(parameters["DexGitHubClientSecret"]).To(Equal("client-secret"))
			Expect(parameters["DexGitHubOrgs"]).To(Equal([]string{"org-a", "org-b"}))
			Expect(parameters["namespace"]).To(Equal("argocd"))
		}
	})

	It("should auto-discover .env next to the first args file", func() {
		workspace, err := setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())
		defer cleanupTestWorkspace(workspace)

		argsPath := filepath.Join(workspace, "args.yaml")
		envPath := filepath.Join(workspace, ".env")

		err = os.WriteFile(envPath, []byte("BLCLI_TERRAFORM_ORGANIZATION_ID=999999999999\n"), 0644)
		Expect(err).NotTo(HaveOccurred())

		resolved := bootstrap.ResolveInitEnvPaths([]string{argsPath}, nil)
		Expect(resolved).To(Equal([]string{envPath}))
	})

	It("should override gitops application repo per project and app", func() {
		workspace, err := setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())
		defer cleanupTestWorkspace(workspace)

		envPath := filepath.Join(workspace, "gitops.env")
		err = os.WriteFile(envPath, []byte(
			"BLCLI_GITOPS_STG_HELLO_WORLD_APPLICATION_REPO=https://github.com/acme/hello-world.git\n",
		), 0644)
		Expect(err).NotTo(HaveOccurred())

		base := renderer.ArgsData{
			"gitops": map[string]interface{}{
				"apps": []interface{}{
					map[string]interface{}{
						"name": "hello-world",
						"project": []interface{}{
							map[string]interface{}{
								"name": "stg",
								"parameters": map[string]interface{}{
									"ApplicationRepo": "https://github.com/your-org/hello-world.git",
								},
							},
						},
					},
				},
			},
		}

		merged, err := bootstrap.ApplyInitEnvOverrides(base, []string{envPath})
		Expect(err).NotTo(HaveOccurred())

		gitops := merged["gitops"].(map[string]interface{})
		apps := gitops["apps"].([]interface{})
		app := apps[0].(map[string]interface{})
		projects := app["project"].([]interface{})
		project := projects[0].(map[string]interface{})
		parameters := project["parameters"].(map[string]interface{})

		Expect(parameters["ApplicationRepo"]).To(Equal("https://github.com/acme/hello-world.git"))
	})

	It("should prioritize earlier env files over later ones", func() {
		workspace, err := setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())
		defer cleanupTestWorkspace(workspace)

		// later env (lower priority)
		laterEnvPath := filepath.Join(workspace, "later.env")
		err = os.WriteFile(laterEnvPath, []byte(
			"BLCLI_TERRAFORM_ORGANIZATION_ID=111111111111\n"+
				"BLCLI_TERRAFORM_BILLING_ACCOUNT_ID=11AAAA-111111-111111\n",
		), 0644)
		Expect(err).NotTo(HaveOccurred())

		// earlier env (higher priority)
		earlierEnvPath := filepath.Join(workspace, "earlier.env")
		err = os.WriteFile(earlierEnvPath, []byte(
			"BLCLI_TERRAFORM_ORGANIZATION_ID=222222222222\n"+
				"BLCLI_TERRAFORM_BILLING_ACCOUNT_ID=22BBBB-222222-222222\n",
		), 0644)
		Expect(err).NotTo(HaveOccurred())

		base := renderer.ArgsData{
			renderer.FieldTerraform: map[string]interface{}{
				renderer.FieldGlobal: map[string]interface{}{
					"OrganizationID":   "123456789012",
					"BillingAccountID": "01ABCD-2EFGH3-4IJKL5",
				},
			},
		}

		merged, err := bootstrap.ApplyInitEnvOverrides(base, []string{earlierEnvPath, laterEnvPath})
		Expect(err).NotTo(HaveOccurred())

		tf := merged[renderer.FieldTerraform].(map[string]interface{})
		global := tf[renderer.FieldGlobal].(map[string]interface{})
		Expect(global["OrganizationID"]).To(Equal("222222222222"))
		Expect(global["BillingAccountID"]).To(Equal("22BBBB-222222-222222"))
	})

	It("should override terraform globals for parsed YAML args", func() {
		workspace, err := setupTestWorkspace()
		Expect(err).NotTo(HaveOccurred())
		defer cleanupTestWorkspace(workspace)

		argsPath := filepath.Join(workspace, "args.yaml")
		err = os.WriteFile(argsPath, []byte(`
terraform:
  global:
    OrganizationID: "0"
    BillingAccountID: "01ABCD-2EFGH3-4IJKL5"
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		envPath := filepath.Join(workspace, "override.env")
		err = os.WriteFile(envPath, []byte(
			"BLCLI_TERRAFORM_ORGANIZATION_ID=888888888888\n"+
				"BLCLI_TERRAFORM_BILLING_ACCOUNT_ID=88YYYY-888888-888888\n",
		), 0644)
		Expect(err).NotTo(HaveOccurred())

		base, err := renderer.LoadArgs(argsPath)
		Expect(err).NotTo(HaveOccurred())

		merged, err := bootstrap.ApplyInitEnvOverrides(base, []string{envPath})
		Expect(err).NotTo(HaveOccurred())

		tf := merged[renderer.FieldTerraform].(map[string]interface{})
		global := tf[renderer.FieldGlobal].(map[string]interface{})
		Expect(global["OrganizationID"]).To(Equal("888888888888"))
		Expect(global["BillingAccountID"]).To(Equal("88YYYY-888888-888888"))
	})
})
