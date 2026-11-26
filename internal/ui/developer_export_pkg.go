package ui

// exportSchemaPackage is the Go file of a schema package for a module the
// tree does not have yet, as the export includes it beside a new form: it
// embeds every definition in its directory, so adding a form later is adding
// its file. It has the two functions every schema package has (Schema, and
// Names for the Developer module's catalogue).
func exportSchemaPackage(module string) string {
	return "// Package " + module + " holds the " + module + " form definitions.\n" +
		"package " + module + "\n\n" +
		"import (\n\t\"embed\"\n\t\"sort\"\n\t\"strings\"\n)\n\n" +
		"//go:embed *.json\n" +
		"var schemaFS embed.FS\n\n" +
		"// Schema returns the definition called name; it panics on a name the\n" +
		"// package does not embed, which is a mistake in the caller.\n" +
		"func Schema(name string) []byte {\n" +
		"\tdata, err := schemaFS.ReadFile(name + \".json\")\n" +
		"\tif err != nil {\n" +
		"\t\tpanic(\"" + module + ": unknown schema \" + name + \": \" + err.Error())\n" +
		"\t}\n" +
		"\treturn data\n" +
		"}\n\n" +
		"// Names lists the schemas this package embeds, sorted, without the .json\n" +
		"// suffix. The Developer module's schema browser uses it.\n" +
		"func Names() []string {\n" +
		"\tentries, _ := schemaFS.ReadDir(\".\")\n" +
		"\tvar out []string\n" +
		"\tfor _, e := range entries {\n" +
		"\t\tif n := e.Name(); strings.HasSuffix(n, \".json\") {\n" +
		"\t\t\tout = append(out, strings.TrimSuffix(n, \".json\"))\n" +
		"\t\t}\n" +
		"\t}\n" +
		"\tsort.Strings(out)\n" +
		"\treturn out\n" +
		"}\n"
}
