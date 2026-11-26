package woaction

import "embed"

//go:embed create.json edit.json
var schemaFS embed.FS

func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("wo_action: unknown schema " + name + ": " + err.Error())
	}
	return data
}
