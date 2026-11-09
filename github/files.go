package workflowfile

import "embed"

//go:embed workflows/factory-tick.yml
var workflows embed.FS

const factoryTick = "workflows/factory-tick.yml"

func ReadFactoryTick() (contents []byte, err error) {
	contents, err = workflows.ReadFile(factoryTick)
	return
}
