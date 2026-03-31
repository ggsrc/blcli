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
