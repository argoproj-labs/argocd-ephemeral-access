package testdata

import _ "embed"

var (
	//go:embed appproject.yaml
	AppProjectYaml string

	//go:embed application.yaml
	ApplicationYaml string
)
