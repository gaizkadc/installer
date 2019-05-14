/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package ingress

import (
	"encoding/json"
	"fmt"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/installer/internal/pkg/errors"
	"github.com/nalej/installer/internal/pkg/workflow/commands/sync/k8s"
	"github.com/nalej/installer/internal/pkg/workflow/entities"
	"github.com/rs/zerolog/log"
	"strings"
)

type InstallVpnServerLB struct {
	k8s.Kubernetes
	PlatformType    string `json:"platform_type"`
}

func NewInstallVpnServerLB(kubeConfigPath string, platformType string) *InstallVpnServerLB {
	return &InstallVpnServerLB{
		Kubernetes: k8s.Kubernetes{
			GenericSyncCommand: *entities.NewSyncCommand(entities.InstallZtPlanetLB),
			KubeConfigPath:     kubeConfigPath,
		},
		PlatformType:    platformType,
	}
}

func NewInstallVpnServerLBFromJSON(raw []byte) (*entities.Command, derrors.Error) {
	ccc := &InstallVpnServerLB{}
	if err := json.Unmarshal(raw, &ccc); err != nil {
		return nil, derrors.NewInvalidArgumentError(errors.UnmarshalError, err)
	}
	ccc.CommandID = entities.GenerateCommandID(ccc.Name())
	var r entities.Command = ccc
	return &r, nil
}

func (imd *InstallVpnServerLB) Run (workflowID string) (*entities.CommandResult, derrors.Error) {
	connectErr := imd.Connect()
	if connectErr != nil {
		return nil, connectErr
	}

	switch imd.PlatformType {
	case grpc_installer_go.Platform_AZURE.String():
		return imd.InstallLoadBalancer(workflowID)
	case grpc_installer_go.Platform_BAREMETAL.String():
		// Baremetal relies on Loadbalancers.
		return imd.InstallLoadBalancer(workflowID)
	case grpc_installer_go.Platform_MINIKUBE.String():
		return imd.InstallMinikube(workflowID)
	}
	log.Warn().Str("platformType", imd.PlatformType).Msg("unsupported platform type")
	return entities.NewCommandResult(
		false, "unsupported platform type", nil), nil
}

func (imd *InstallVpnServerLB) InstallLoadBalancer (workflowID string) (*entities.CommandResult, derrors.Error) {
	azureService := AzureZTPlanetService
	err := imd.CreateService(&azureService)
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("error creating VPN Server LB service")
		return entities.NewCommandResult(
			false, "cannot install service", err), nil
	}
	msg := fmt.Sprintf("VPN Server installed on %s", imd.PlatformType)
	return entities.NewSuccessCommand([]byte(msg)), nil
}

func (imd *InstallVpnServerLB) InstallMinikube (workflowID string) (*entities.CommandResult, derrors.Error) {
	err := imd.CreateService(&MinikubeConsulService)
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("error creating  VPN Server LB service")
		return entities.NewCommandResult(
			false, "cannot install service", err), nil
	}
	return entities.NewSuccessCommand([]byte("VPN Server installed on Minikube")), nil
}

func (imd *InstallVpnServerLB) String () string {
	return fmt.Sprintf("SYNC InstallVpnServerLB on %s", imd.PlatformType)
}

func (imd *InstallVpnServerLB) PrettyPrint (indentation int) string {
	return strings.Repeat(" ", indentation) + imd.String()
}

func (imd *InstallVpnServerLB) UserString () string {
	return fmt.Sprintf("Installing VPN Server loadbalancer")
}