package dockercomposegenerator

import (
	"fmt"
	schema "pgconverge/internal"
	helper "pgconverge/internal/util"
)

func GenerateComposeMap(nodes []schema.Node) map[string]interface{} {
	compose := map[string]interface{}{
		"services": map[string]interface{}{},
		"volumes":  map[string]interface{}{},
	}
	services := compose["services"].(map[string]interface{})
	volumes := compose["volumes"].(map[string]interface{})

	for _, node := range nodes {
		services[node.Name] = map[string]interface{}{
			"image":          "postgres:16",
			"container_name": node.Name,
			"environment": map[string]string{
				"POSTGRES_USER":     node.User,
				"POSTGRES_PASSWORD": node.Password,
				"POSTGRES_DB":       node.Database,
			},
			"ports": []string{fmt.Sprintf("%d:5432", helper.GetPort(node.Name))},
			"volumes": []string{
				fmt.Sprintf("%s_data:/var/lib/postgresql/data", node.Name),
				"./:/scripts",
			},
			"entrypoint": []string{"/scripts/entrypoint.sh", node.Name},
		}
		volumes[node.Name+"_data"] = map[string]interface{}{}
	}

	return compose
}
