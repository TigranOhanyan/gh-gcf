package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const envSecretPrefix = "FACTORY_SECRET_PREFIX"
const envFactoryID = "FACTORY_GITHUB_APP_ID"
const cursorSecret = "cursor-api-key"
const factoryIDSecret = "factory-app-id"
const factoryKeySecret = "factory-app-private-key"
const appSecretPath = "app/"

func loadFixedSecrets(ctx context.Context) (err error) {
	prefix := os.Getenv(envSecretPrefix)
	if prefix == "" {
		return
	}
	client, err := secretClient(ctx)
	if err != nil {
		return
	}
	err = putRequired(ctx, client, prefix, cursorSecret)
	if err != nil {
		return
	}
	err = putRequired(ctx, client, prefix, factoryIDSecret)
	if err != nil {
		return
	}
	err = putRequired(ctx, client, prefix, factoryKeySecret)
	return
}

func loadAppSecrets(ctx context.Context, factoryDir string) (err error) {
	prefix := os.Getenv(envSecretPrefix)
	if prefix == "" {
		return
	}
	ids, err := commands.ListAppIDs(factoryDir)
	if err != nil {
		return
	}
	client, err := secretClient(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		text := strconv.FormatInt(id, 10)
		secretName := prefix + "/" + appSecretPath + text
		envName, known := secretEnvName(prefix, secretName)
		if !known {
			err = fmt.Errorf("secret %s is not a factory secret", secretName)
			return
		}
		var value string
		value, err = secretValue(ctx, client, secretName)
		if err != nil {
			return
		}
		missing := value == ""
		if missing {
			err = fmt.Errorf("secret %s has no value", secretName)
			return
		}
		err = os.Setenv(envName, value)
		if err != nil {
			return
		}
	}
	return
}

func putRequired(ctx context.Context, client *secretsmanager.Client, prefix string, name string) (err error) {
	secretName := prefix + "/" + name
	envName, known := secretEnvName(prefix, secretName)
	if !known {
		err = fmt.Errorf("secret %s is not a factory secret", secretName)
		return
	}
	var value string
	value, err = secretValue(ctx, client, secretName)
	if err != nil {
		return
	}
	missing := value == ""
	if missing {
		err = fmt.Errorf("secret %s has no value", secretName)
		zap.L().Error("secret", zap.String("name", secretName), zap.Error(err))
		return
	}
	err = os.Setenv(envName, value)
	return
}

func secretClient(ctx context.Context) (client *secretsmanager.Client, err error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		zap.L().Error("aws config", zap.Error(err))
		return
	}
	client = secretsmanager.NewFromConfig(cfg)
	return
}

func secretValue(ctx context.Context, client *secretsmanager.Client, name string) (value string, err error) {
	input := &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)}
	out, err := client.GetSecretValue(ctx, input)
	if err != nil {
		var missing *types.ResourceNotFoundException
		absent := errors.As(err, &missing)
		if absent {
			err = nil
			return
		}
		zap.L().Error("secret", zap.String("name", name), zap.Error(err))
		return
	}
	if out.SecretString != nil {
		value = *out.SecretString
	}
	return
}

func secretEnvName(prefix string, secretName string) (envName string, ok bool) {
	rest, found := strings.CutPrefix(secretName, prefix+"/")
	if !found {
		return
	}
	switch rest {
	case cursorSecret:
		envName = envCursorKey
		ok = true
	case factoryIDSecret:
		envName = envFactoryID
		ok = true
	case factoryKeySecret:
		envName = envFactoryKey
		ok = true
	default:
		id, isApp := strings.CutPrefix(rest, appSecretPath)
		if !isApp || id == "" {
			return
		}
		_, parseErr := strconv.ParseInt(id, 10, 64)
		if parseErr != nil {
			return
		}
		envName = appSecretPrefix + id
		ok = true
	}
	return
}
