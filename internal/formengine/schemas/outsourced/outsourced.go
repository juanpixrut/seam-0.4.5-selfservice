package outsourced

import "embed"

//go:embed create.json account.json
var schemaFS embed.FS

func Schema(name string) []byte {
	data, err := schemaFS.ReadFile(name + ".json")
	if err != nil {
		panic("outsourced: unknown schema " + name + ": " + err.Error())
	}
	return data
}
