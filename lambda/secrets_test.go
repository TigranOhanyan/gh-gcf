package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Secret_should_name_the_cursor_key(t *testing.T) {
	envName, ok := secretEnvName("factory-a-secrets", "factory-a-secrets/cursor-api-key")
	assert.True(t, ok)
	assert.Equal(t, "CURSOR_API_KEY", envName)
}

func Test_Secret_should_name_the_factory_app(t *testing.T) {
	idName, idOK := secretEnvName("factory-a-secrets", "factory-a-secrets/factory-app-id")
	keyName, keyOK := secretEnvName("factory-a-secrets", "factory-a-secrets/factory-app-private-key")
	assert.True(t, idOK)
	assert.True(t, keyOK)
	assert.Equal(t, "FACTORY_GITHUB_APP_ID", idName)
	assert.Equal(t, "FACTORY_GITHUB_APP_PRIVATE_KEY", keyName)
}

func Test_Secret_should_name_an_app_key(t *testing.T) {
	envName, ok := secretEnvName("factory-a-secrets", "factory-a-secrets/app/42")
	assert.True(t, ok)
	assert.Equal(t, "FACTORY_GITHUB_APP_42", envName)
}

func Test_Secret_should_ignore_another_prefix(t *testing.T) {
	_, ok := secretEnvName("factory-a-secrets", "factory-b-secrets/app/42")
	assert.False(t, ok)
}
