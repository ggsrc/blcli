package terraform

import (
	"fmt"
	"strings"

	"blcli/pkg/renderer"
)

// ProjectComponentNode represents a (project, component) pair for the DAG.
type ProjectComponentNode struct {
	Project   string
	Component string
}

// String returns "project/component" for marker storage.
func (n ProjectComponentNode) String() string {
	return n.Project + "/" + n.Component
}

// ParseProjectComponentNode parses "project/component" back to a node.
func ParseProjectComponentNode(s string) (ProjectComponentNode, bool) {
	idx := strings.Index(s, "/")
	if idx <= 0 || idx >= len(s)-1 {
		return ProjectComponentNode{}, false
	}
	return ProjectComponentNode{Project: s[:idx], Component: s[idx+1:]}, true
}

// BuildProjectDependencyPlan builds DAG from terraform.projects[].components[].depends_on,
// returns topological order, which (project, component) should be rendered to subdir,
// and the dependency layer of each subdir component (1-based; layer 1 = depends only on root components).
// Apply uses layers to promote and apply in rounds: after each layer is promoted, only affected projects are applied.
func BuildProjectDependencyPlan(templateArgs renderer.ArgsData, projectNames []string) (dependencyOrder []string, subdirComponents map[string][]string, subdirComponentLayers map[string]int, err error) {
	subdirComponents = make(map[string][]string)
	subdirComponentLayers = make(map[string]int)
	nodes := make(map[string]ProjectComponentNode) // key = "project/component"
	edges := make(map[string][]string)            // key -> list of keys it depends on

	// Collect nodes and edges from templateArgs.terraform.projects
	tf, ok := templateArgs[renderer.FieldTerraform]
	if !ok {
		return nil, subdirComponents, subdirComponentLayers, nil
	}
	tfMap := toMapStringInterface(tf)
	if tfMap == nil {
		return nil, subdirComponents, subdirComponentLayers, nil
	}
	projectsList, ok := tfMap[renderer.FieldProjects]
	if !ok {
		return nil, subdirComponents, subdirComponentLayers, nil
	}
	list, ok := projectsList.([]interface{})
	if !ok {
		return nil, subdirComponents, subdirComponentLayers, nil
	}

	for _, p := range list {
		projectMap := toMapStringInterface(p)
		if projectMap == nil {
			continue
		}
		projectName, _ := projectMap[renderer.FieldName].(string)
		if projectName == "" {
			continue
		}
		// Only consider projects we're initializing
		found := false
		for _, n := range projectNames {
			if n == projectName {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		comps, ok := projectMap[renderer.FieldComponents]
		if !ok {
			continue
		}
		compsList, ok := comps.([]interface{})
		if !ok {
			continue
		}

		for _, c := range compsList {
			compMap := toMapStringInterface(c)
			if compMap == nil {
				continue
			}
			compName, _ := compMap[renderer.FieldName].(string)
			if compName == "" {
				continue
			}
			node := ProjectComponentNode{Project: projectName, Component: compName}
			key := node.String()
			nodes[key] = node

			dependsOn, _ := compMap["depends_on"]
			if dependsOn == nil {
				continue
			}
			depList, ok := dependsOn.([]interface{})
			if !ok {
				continue
			}
			hasCrossProject := false
			for _, d := range depList {
				depMap := toMapStringInterface(d)
				if depMap == nil {
					continue
				}
				depProject, _ := depMap["project"].(string)
				depComp, _ := depMap["component"].(string)
				if depProject == "" || depComp == "" {
					continue
				}
				depKey := depProject + "/" + depComp
				edges[key] = append(edges[key], depKey)
				if depProject != projectName {
					hasCrossProject = true
				}
			}
			if hasCrossProject {
				subdirComponents[projectName] = append(subdirComponents[projectName], compName)
			}
		}
	}

	// Topological sort
	order, err := topologicalSort(nodes, edges)
	if err != nil {
		return nil, nil, nil, err
	}
	dependencyOrder = make([]string, 0, len(order))
	for _, n := range order {
		dependencyOrder = append(dependencyOrder, n.String())
	}

	// Compute layer for each node (in topological order): layer = 1 + max(layer of deps), 0 if no deps
	layers := make(map[string]int)
	for _, n := range order {
		key := n.String()
		deps := edges[key]
		if len(deps) == 0 {
			layers[key] = 0
			continue
		}
		maxDepLayer := -1
		for _, d := range deps {
			if L, ok := layers[d]; ok && L > maxDepLayer {
				maxDepLayer = L
			}
		}
		layers[key] = maxDepLayer + 1
	}

	// Subdir component layers: only for (project, component) in subdir; use DAG layer as promote round (1 = first round after root apply)
	subdirSet := make(map[string]bool)
	for proj, comps := range subdirComponents {
		for _, c := range comps {
			subdirSet[proj+"/"+c] = true
		}
	}
	for key, layer := range layers {
		if subdirSet[key] {
			subdirComponentLayers[key] = layer // 1 = first promote round, 2 = second, etc.
		}
	}

	return dependencyOrder, subdirComponents, subdirComponentLayers, nil
}

func topologicalSort(nodes map[string]ProjectComponentNode, edges map[string][]string) ([]ProjectComponentNode, error) {
	inDegree := make(map[string]int)
	for k := range nodes {
		inDegree[k] = 0
	}
	for _, deps := range edges {
		for _, d := range deps {
			if _, ok := nodes[d]; ok {
				inDegree[d] = inDegree[d] // ensure key exists
			}
		}
	}
	for k, deps := range edges {
		for _, d := range deps {
			if _, ok := nodes[d]; ok {
				inDegree[k]++
			}
		}
	}

	var queue []string
	for k := range nodes {
		if inDegree[k] == 0 {
			queue = append(queue, k)
		}
	}
	var result []ProjectComponentNode
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, nodes[cur])
		for k, deps := range edges {
			for _, d := range deps {
				if d == cur {
					inDegree[k]--
					if inDegree[k] == 0 {
						queue = append(queue, k)
					}
					break
				}
			}
		}
	}
	if len(result) != len(nodes) {
		return nil, fmt.Errorf("dependency cycle detected in project components")
	}
	return result, nil
}

// IsComponentInSubdir returns whether (projectName, componentName) should be rendered to a subdir.
func IsComponentInSubdir(subdirComponents map[string][]string, projectName, componentName string) bool {
	if subdirComponents == nil {
		return false
	}
	list, ok := subdirComponents[projectName]
	if !ok {
		return false
	}
	for _, c := range list {
		if c == componentName {
			return true
		}
	}
	return false
}
