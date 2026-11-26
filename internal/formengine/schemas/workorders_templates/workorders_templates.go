package workorders_templates

import "embed"

//go:embed create.json step-2.json
var schemaFS embed.FS

func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("workorders_templates: unknown schema " + name + ": " + err.Error())
	}
	return data
}
