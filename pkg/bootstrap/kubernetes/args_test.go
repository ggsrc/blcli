package kubernetes_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"blcli/pkg/bootstrap/kubernetes"
	"blcli/pkg/renderer"
)

var _ = Describe("Kubernetes Args", func() {
	Describe("ExtractComponentArgs", func() {
		It("should extract component-specific args from kubernetes.projects", func() {
			templateArgs := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "test-org",
				},
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "test-project",
							"components": []interface{}{
								map[string]interface{}{
									"name": "test-component",
									"parameters": map[string]interface{}{
										"namespace": "test-namespace",
										"version":   "1.0.0",
									},
								},
							},
						},
					},
				},
			}

			args := kubernetes.ExtractComponentArgs(templateArgs, "test-component")
			Expect(args).NotTo(BeNil())
			Expect(args["namespace"]).To(Equal("test-namespace"))
			Expect(args["version"]).To(Equal("1.0.0"))
		})

		It("should expose kebab-case parameters as original, PascalCase and lowerCamelCase keys", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "corp",
							"components": []interface{}{
								map[string]interface{}{
									"name": "4-grafana",
									"parameters": map[string]interface{}{
										"hosted-domain":    "corp.example",
										"root-url":         "https://grafana.corp.example",
										"remote-write-url": "http://vminsert:8480",
										"argocd-url":       "https://argocd.corp.example",
									},
								},
							},
						},
					},
				},
			}

			args := kubernetes.ExtractComponentArgs(templateArgs, "grafana")
			Expect(args).To(HaveKeyWithValue("hosted-domain", "corp.example"))
			Expect(args).To(HaveKeyWithValue("HostedDomain", "corp.example"))
			Expect(args).To(HaveKeyWithValue("hostedDomain", "corp.example"))
			Expect(args).To(HaveKeyWithValue("RootURL", "https://grafana.corp.example"))
			Expect(args).To(HaveKeyWithValue("rootUrl", "https://grafana.corp.example"))
			Expect(args).To(HaveKeyWithValue("remoteWriteUrl", "http://vminsert:8480"))
			Expect(args).To(HaveKeyWithValue("ArgoCDURL", "https://argocd.corp.example"))
			Expect(args).To(HaveKeyWithValue("argocdURL", "https://argocd.corp.example"))
		})

		It("should not borrow another project's parameters for a component the project does not list", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "prd",
							"components": []interface{}{
								map[string]interface{}{
									"name":       "0-external-secrets-operator",
									"parameters": map[string]interface{}{"namespace": "eso-prd"},
								},
							},
						},
						map[string]interface{}{
							"name": "corp",
							"components": []interface{}{
								map[string]interface{}{
									"name":       "0-external-secrets",
									"parameters": map[string]interface{}{"namespace": "external-secrets"},
								},
							},
						},
					},
				},
			}

			// corp pulls in external-secrets-operator only as a dependency.
			corpArgs := kubernetes.ExtractComponentArgs(kubernetes.GetProjectArgs(templateArgs, "corp"), "external-secrets-operator")
			Expect(corpArgs).NotTo(HaveKey("namespace"))

			prdArgs := kubernetes.ExtractComponentArgs(kubernetes.GetProjectArgs(templateArgs, "prd"), "external-secrets-operator")
			Expect(prdArgs).To(HaveKeyWithValue("namespace", "eso-prd"))
		})

		It("should keep an explicitly spelled key over an alias", func() {
			templateArgs := renderer.ArgsData{
				"components": []interface{}{
					map[string]interface{}{
						"name": "istio",
						"parameters": map[string]interface{}{
							"istio-version": "1.31.3",
							"IstioVersion":  "1.30.7",
						},
					},
				},
			}

			args := kubernetes.ExtractComponentArgs(templateArgs, "istio")
			Expect(args).To(HaveKeyWithValue("IstioVersion", "1.30.7"))
			Expect(args).To(HaveKeyWithValue("istio-version", "1.31.3"))
		})

		It("should return empty args if component not found", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{},
				},
			}

			args := kubernetes.ExtractComponentArgs(templateArgs, "non-existent")
			Expect(args).NotTo(BeNil())
			Expect(len(args)).To(BeNumerically("<=", 1)) // Only global might be present
		})
	})

	Describe("MergeComponentArgsWithDefaults", func() {
		It("should let a kebab-case arg replace a lowerCamelCase args.yaml default", func() {
			// default.yaml sets image-tag; args.yaml declares imageTag with default "";
			// the template reads .imageTag.
			templateArgs := renderer.ArgsData{
				"components": []interface{}{
					map[string]interface{}{
						"name": "0-external-secrets-operator",
						"parameters": map[string]interface{}{
							"image-tag":     "v2.0.1",
							"replica-count": 2,
						},
					},
				},
			}
			defaults := map[string]interface{}{
				"imageTag":     "",
				"replicaCount": 1,
				"namespace":    "external-secrets",
			}

			args := kubernetes.MergeComponentArgsWithDefaults(
				kubernetes.ExtractComponentArgs(templateArgs, "external-secrets-operator"), defaults)
			Expect(args).To(HaveKeyWithValue("imageTag", "v2.0.1"))
			Expect(args).To(HaveKeyWithValue("ImageTag", "v2.0.1"))
			Expect(args).To(HaveKeyWithValue("replicaCount", 2))
			Expect(args).To(HaveKeyWithValue("namespace", "external-secrets"))
			Expect(args).To(HaveKeyWithValue("Namespace", "external-secrets"))
		})

		It("should keep an explicit false over a true default", func() {
			args := kubernetes.MergeComponentArgsWithDefaults(
				renderer.ArgsData{"auth-enabled": false, "authEnabled": false},
				map[string]interface{}{"authEnabled": true},
			)
			Expect(args).To(HaveKeyWithValue("authEnabled", false))
			Expect(args).To(HaveKeyWithValue("AuthEnabled", false))
		})
	})

	Describe("GetAvailableInitComponents", func() {
		It("should extract available components from projects", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "test-project",
							"components": []interface{}{
								map[string]interface{}{
									"name": "test-component",
								},
								map[string]interface{}{
									"name": "another-component",
								},
							},
						},
					},
				},
			}

			// GetAvailableInitComponents is deprecated, use GetAvailableOptionalComponents instead
			available := kubernetes.GetAvailableOptionalComponents(templateArgs)
			Expect(available).NotTo(BeNil())
			Expect(available["test-component"]).To(BeTrue())
			Expect(available["another-component"]).To(BeTrue())
			Expect(available["non-existent"]).To(BeFalse())
		})

		It("should return empty map if no components", func() {
			templateArgs := renderer.ArgsData{}
			available := kubernetes.GetAvailableOptionalComponents(templateArgs)
			Expect(available).NotTo(BeNil())
			Expect(len(available)).To(Equal(0))
		})
	})

	Describe("GetAvailableOptionalComponents", func() {
		It("should extract available optional components from projects", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "test-project",
							"components": []interface{}{
								map[string]interface{}{
									"name": "optional-1",
								},
								map[string]interface{}{
									"name": "optional-2",
								},
							},
						},
					},
				},
			}

			available := kubernetes.GetAvailableOptionalComponents(templateArgs)
			Expect(available).NotTo(BeNil())
			Expect(available["optional-1"]).To(BeTrue())
			Expect(available["optional-2"]).To(BeTrue())
		})

		It("should return empty map if no optional components", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{},
				},
			}
			available := kubernetes.GetAvailableOptionalComponents(templateArgs)
			Expect(available).NotTo(BeNil())
			Expect(len(available)).To(Equal(0))
		})
	})

	Describe("GetKubernetesProjects", func() {
		It("should extract project names from kubernetes.projects", func() {
			templateArgs := renderer.ArgsData{
				"kubernetes": map[string]interface{}{
					"projects": []interface{}{
						map[string]interface{}{
							"name": "project-1",
						},
						map[string]interface{}{
							"name": "project-2",
						},
					},
				},
			}

			projects := kubernetes.GetKubernetesProjects(templateArgs)
			Expect(projects).To(ContainElements("project-1", "project-2"))
		})

		It("should return empty list if no projects", func() {
			templateArgs := renderer.ArgsData{}
			projects := kubernetes.GetKubernetesProjects(templateArgs)
			Expect(projects).NotTo(BeNil())
			Expect(len(projects)).To(Equal(0))
		})
	})

	Describe("GetProjectArgs", func() {
		It("should extract project-specific args", func() {
			templateArgs := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "test-org",
				},
				"kubernetes": map[string]interface{}{
					"global": map[string]interface{}{
						"K8sVersion": "1.28",
					},
					"projects": []interface{}{
						map[string]interface{}{
							"name": "test-project",
							"global": map[string]interface{}{
								"ProjectName": "test-project",
							},
							"components": []interface{}{
								map[string]interface{}{
									"name": "test-component",
									"parameters": map[string]interface{}{
										"namespace": "test-ns",
									},
								},
							},
						},
					},
				},
			}

			projectArgs := kubernetes.GetProjectArgs(templateArgs, "test-project")
			Expect(projectArgs).NotTo(BeNil())

			// Check global is merged
			if global, ok := projectArgs["global"].(map[string]interface{}); ok {
				Expect(global["GlobalName"]).To(Equal("test-org"))
				Expect(global["K8sVersion"]).To(Equal("1.28"))
				Expect(global["ProjectName"]).To(Equal("test-project"))
			}
		})

		It("should return base args if project not found", func() {
			templateArgs := renderer.ArgsData{
				"global": map[string]interface{}{
					"GlobalName": "test-org",
				},
			}

			projectArgs := kubernetes.GetProjectArgs(templateArgs, "non-existent")
			Expect(projectArgs).NotTo(BeNil())
			if global, ok := projectArgs["global"].(map[string]interface{}); ok {
				Expect(global["GlobalName"]).To(Equal("test-org"))
			}
		})
	})
})

// Helper function to create temporary test workspace
func createTestWorkspace() (string, error) {
	workspace := filepath.Join("workspace", "test-kubernetes")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("failed to create test workspace: %w", err)
	}
	return workspace, nil
}

// Helper function to cleanup test workspace
func cleanupTestWorkspace(workspace string) error {
	if workspace == "" {
		return nil
	}
	if filepath.Base(filepath.Dir(workspace)) == "workspace" {
		return os.RemoveAll(workspace)
	}
	return nil
}
