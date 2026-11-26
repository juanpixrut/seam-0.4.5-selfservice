package ui

// rowFxMode is the value of <body data-fe-fxmode>, which SeamRowFx (app.js)
// reads: "scoped" (rows grow and shrink only inside data-fe-fx elements, the
// default), "all" or "off". Anything else is "scoped".
func rowFxMode(cfg Config) string {
	switch cfg.RowEffects {
	case "all", "off":
		return cfg.RowEffects
	}
	return "scoped"
}
