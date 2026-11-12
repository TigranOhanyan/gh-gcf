package workflowfile

import "embed"

//go:embed workflows/factory-deploy.yml workflows/factory-delete.yml workflows/factory-secrets.yml infrastructure.yaml secrets.yaml
var workflows embed.FS

const factoryDeploy = "workflows/factory-deploy.yml"
const factoryDelete = "workflows/factory-delete.yml"
const factorySecrets = "workflows/factory-secrets.yml"
const infrastructure = "infrastructure.yaml"
const secrets = "secrets.yaml"

func ReadFactoryDeploy() (contents []byte, err error) {
	contents, err = workflows.ReadFile(factoryDeploy)
	return
}

func ReadFactoryDelete() (contents []byte, err error) {
	contents, err = workflows.ReadFile(factoryDelete)
	return
}

func ReadFactorySecrets() (contents []byte, err error) {
	contents, err = workflows.ReadFile(factorySecrets)
	return
}

func ReadInfrastructure() (contents []byte, err error) {
	contents, err = workflows.ReadFile(infrastructure)
	return
}

func ReadSecrets() (contents []byte, err error) {
	contents, err = workflows.ReadFile(secrets)
	return
}
