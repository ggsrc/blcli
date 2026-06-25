package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Profile merge helpers", func() {
	It("should append overlay terraform components to an existing project", func() {
		base := map[string]interface{}{
			"projects": []interface{}{
				map[string]interface{}{
					"name": "app",
					"components": []interface{}{
						map[string]interface{}{
							"name":       "gke",
							"parameters": map[string]interface{}{"clusterName": "app-cluster"},
						},
					},
				},
			},
		}
		overlay := map[string]interface{}{
			"projects": []interface{}{
				map[string]interface{}{
					"name": "app",
					"components": []interface{}{
						map[string]interface{}{
							"name": "dns",
							"parameters": map[string]interface{}{
								"zones": []interface{}{},
							},
						},
						map[string]interface{}{
							"name": "outputs",
							"parameters": map[string]interface{}{
								"certificate_maps": []interface{}{
									map[string]interface{}{"map_name": "app"},
								},
							},
						},
					},
				},
			},
		}

		merged := mergeModuleDefaultData(base, overlay)
		projects, ok := merged["projects"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(projects).To(HaveLen(1))

		project := projects[0].(map[string]interface{})
		components := project["components"].([]interface{})
		Expect(components).To(HaveLen(3))

		outputs := components[2].(map[string]interface{})
		Expect(outputs["name"]).To(Equal("outputs"))
		params := outputs["parameters"].(map[string]interface{})
		Expect(params).To(HaveKey("certificate_maps"))
	})
})
