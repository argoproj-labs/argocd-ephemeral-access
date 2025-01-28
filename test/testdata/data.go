package testdata

import _ "embed"

var (
	//go:embed appproject.yaml
	AppProject string

	//go:embed app.yaml.tmpl
	AppYAMLTmpl string

	//go:embed appproject.yaml.tmpl
	AppProjectYAMLTmpl string
)
