/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package k8s

import (
	"encoding/json"
	"fmt"
	"github.com/nalej/derrors"
	"github.com/nalej/installer/internal/pkg/errors"
	"github.com/nalej/installer/internal/pkg/workflow/entities"
	"github.com/rs/zerolog/log"
	"github.com/satori/go.uuid"
	"k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const TargetNamespace = "nalej"

type CreateManagementConfig struct {
	Kubernetes
	PublicHost     string `json:"public_host"`
	PublicPort     string `json:"public_port"`
	DockerUsername string `json:"docker_username"`
	DockerPassword string `json:"docker_password"`
}

func NewCreateManagementConfig(
	kubeConfigPath string,
	publicHost string, publicPort string,
	dockerUsername string, dockerPassword string) *CreateManagementConfig {
	return &CreateManagementConfig{
		Kubernetes: Kubernetes{
			GenericSyncCommand: *entities.NewSyncCommand(entities.CreateManagementConfig),
			KubeConfigPath:     kubeConfigPath,
		},
		PublicHost:     publicHost,
		PublicPort:     publicPort,
		DockerUsername: dockerUsername,
		DockerPassword: dockerPassword,
	}
}

func NewCreateManagementConfigFromJSON(raw []byte) (*entities.Command, derrors.Error) {
	cmc := &CreateManagementConfig{}
	if err := json.Unmarshal(raw, &cmc); err != nil {
		return nil, derrors.NewInvalidArgumentError(errors.UnmarshalError, err)
	}
	cmc.CommandID = entities.GenerateCommandID(cmc.Name())
	var r entities.Command = cmc
	return &r, nil
}

func (cmc *CreateManagementConfig) createConfigMap() derrors.Error {
	config := &v1.ConfigMap{
		TypeMeta: v12.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      "management-config",
			Namespace: TargetNamespace,
			Labels:    map[string]string{"cluster": "management"},
		},
		Data: map[string]string{
			"public_host": cmc.PublicHost,
			"public_port": cmc.PublicPort},
	}

	client := cmc.Client.CoreV1().ConfigMaps(config.Namespace)
	log.Debug().Interface("configMap", config).Msg("creating management config")
	created, err := client.Create(config)
	if err != nil {
		return derrors.AsError(err, "cannot create configmap")
	}
	log.Debug().Interface("created", created).Msg("new config map has been created")
	return nil
}

func (cmc *CreateManagementConfig) createDockerSecret() derrors.Error {
	docker := &v1.Secret{
		TypeMeta: v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      "docker-credentials",
			Namespace: TargetNamespace,
			Labels:    map[string]string{"cluster": "management"},
		},
		Data: map[string][]byte{
			"username": []byte(cmc.DockerUsername),
			"password": []byte(cmc.DockerPassword),
		},
		Type: v1.SecretTypeOpaque,
	}
	client := cmc.Client.CoreV1().Secrets(docker.Namespace)
	created, err := client.Create(docker)
	if err != nil {
		return derrors.AsError(err, "cannot create docker secret")
	}
	log.Debug().Interface("created", created).Msg("new secret has been created")
	return nil
}

func (cmc *CreateManagementConfig) createAuthSecret() derrors.Error {
	docker := &v1.Secret{
		TypeMeta: v12.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      "authx-secret",
			Namespace: TargetNamespace,
			Labels:    map[string]string{"cluster": "management", "component": "authx"},
		},
		Data: map[string][]byte{
			"secret": []byte(uuid.NewV4().String()),
		},
		Type: v1.SecretTypeOpaque,
	}
	client := cmc.Client.CoreV1().Secrets(docker.Namespace)
	created, err := client.Create(docker)
	if err != nil {
		return derrors.AsError(err, "cannot create authx secret")
	}
	log.Debug().Interface("created", created).Msg("new secret has been created")
	return nil
}

func (cmc *CreateManagementConfig) Run(workflowID string) (*entities.CommandResult, derrors.Error) {
	connectErr := cmc.Connect()
	if connectErr != nil {
		return nil, connectErr
	}

	cErr := cmc.CreateNamespacesIfNotExist(TargetNamespace)
	if cErr != nil {
		return entities.NewCommandResult(false, "cannot create namespace", cErr), nil
	}

	err := cmc.createConfigMap()
	if err != nil {
		return entities.NewCommandResult(
			false, "cannot create management config", err), nil
	}

	err = cmc.createDockerSecret()
	if err != nil {
		return entities.NewCommandResult(
			false, "cannot create management config", err), nil
	}
	err = cmc.createAuthSecret()
	if err != nil {
		return entities.NewCommandResult(
			false, "cannot create management config", err), nil
	}

	return entities.NewSuccessCommand([]byte("management cluster config has been created")), nil
}

func (cmc *CreateManagementConfig) String() string {
	return fmt.Sprintf("SYNC CreateManagementConfig publicHost: %s, publicPort: %s", cmc.PublicHost, cmc.PublicPort)
}

func (cmc *CreateManagementConfig) PrettyPrint(indentation int) string {
	return strings.Repeat(" ", indentation) + cmc.String()
}

func (cmc *CreateManagementConfig) UserString() string {
	return fmt.Sprintf("Creating management cluster config with public address %s:%s", cmc.PublicHost, cmc.PublicPort)
}
