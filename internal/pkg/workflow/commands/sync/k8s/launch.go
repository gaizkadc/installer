/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package k8s

import (
	"encoding/json"
	"fmt"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	entities2 "github.com/nalej/installer/internal/pkg/entities"
	"github.com/nalej/installer/internal/pkg/errors"
	"github.com/nalej/installer/internal/pkg/workflow/entities"
	"github.com/rs/zerolog/log"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"path"
	"reflect"

	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const AzureStorageClass = "managed-premium"

var ProductionImagePullSecret = &v1.LocalObjectReference{
	Name: entities2.ProdRegistryName,
}

var StagingImagePullSecret = &v1.LocalObjectReference{
	Name: entities2.StagingRegistryName,
}

var DevImagePullSecret = &v1.LocalObjectReference{
	Name: entities2.DevRegistryName,
}

var ProductionImagePullSecrets = []v1.LocalObjectReference{*ProductionImagePullSecret}
var StagingImagePullSecrets = []v1.LocalObjectReference{*ProductionImagePullSecret, *StagingImagePullSecret}
var DevImagePullSecrets = []v1.LocalObjectReference{*ProductionImagePullSecret, *StagingImagePullSecret, *DevImagePullSecret}

// LaunchComponents is a command that reads a directory for YAML files and triggers the creation
// of those entities in Kubernetes.
type LaunchComponents struct {
	Kubernetes
	Namespaces    []string `json:"namespaces"`
	ComponentsDir string   `json:"componentsDir"`
	PlatformType  string   `json:"platform_type"`
	Environment   string   `json:"environment"`
}

// NewLaunchComponents creates a new LaunchComponents command.
func NewLaunchComponents(kubeConfigPath string, namespaces []string, componentsDir string, targetPlatform string) *LaunchComponents {
	return &LaunchComponents{
		Kubernetes: Kubernetes{
			GenericSyncCommand: *entities.NewSyncCommand(entities.LaunchComponents),
			KubeConfigPath:     kubeConfigPath,
		},
		Namespaces:    namespaces,
		ComponentsDir: componentsDir,
		PlatformType:  targetPlatform,
	}
}

// NewLaunchComponentsFromJSON creates an LaunchComponents command from a JSON object.
func NewLaunchComponentsFromJSON(raw []byte) (*entities.Command, derrors.Error) {
	lc := &LaunchComponents{}
	if err := json.Unmarshal(raw, &lc); err != nil {
		return nil, derrors.NewInvalidArgumentError(errors.UnmarshalError, err)
	}
	lc.CommandID = entities.GenerateCommandID(lc.Name())
	var r entities.Command = lc
	return &r, nil
}

// Run the command.
func (lc *LaunchComponents) Run(workflowID string) (*entities.CommandResult, derrors.Error) {

	connectErr := lc.Connect()
	if connectErr != nil {
		return nil, connectErr
	}

	targetEnvironment, found := entities2.TargetEnvironmentFromString[lc.Environment]
	if !found {
		return nil, derrors.NewInvalidArgumentError("cannot determine target environment").WithParams(lc.Environment)
	}

	for _, target := range lc.Namespaces {
		createErr := lc.createNamespace(target)
		if createErr != nil {
			return nil, createErr
		}
	}

	fileInfo, err := ioutil.ReadDir(lc.ComponentsDir)
	if err != nil {
		return nil, derrors.AsError(err, "cannot read components dir")
	}
	numLaunched := 0
	for _, file := range fileInfo {
		if strings.HasSuffix(file.Name(), ".yaml") {
			log.Info().Str("file", file.Name()).Msg("processing component")
			err := lc.launchComponent(path.Join(lc.ComponentsDir, file.Name()), targetEnvironment)
			if err != nil {
				return entities.NewCommandResult(false, "cannot launch component", err), nil
			}
			numLaunched++
		}
	}
	msg := fmt.Sprintf("%d components have been launched", numLaunched)
	return entities.NewCommandResult(true, msg, nil), nil
}

// ListComponents obtains a list of the files that need to be installed.
// TODO Overwrite files if a *.yaml.minikube file is found on the same entity with a MINIKUBE environment.
func (lc *LaunchComponents) ListComponents() []string {
	fileInfo, err := ioutil.ReadDir(lc.ComponentsDir)
	if err != nil {
		log.Fatal().Err(err).Str("componentsDir", lc.ComponentsDir).Msg("cannot read components dir")
	}
	result := make([]string, 0)
	for _, file := range fileInfo {
		if strings.HasSuffix(file.Name(), ".yaml") {
			result = append(result, file.Name())
		}
	}
	return result
}

// adaptDeployment modifies the deployment to include image pull secrets depending on the type of environment.
func (lc *LaunchComponents) adaptDeployment(deployment *appsv1.Deployment, targetEnvironment entities2.TargetEnvironment) *appsv1.Deployment {
	aux := deployment
	switch targetEnvironment {
	case entities2.Production:
		aux.Spec.Template.Spec.ImagePullSecrets = ProductionImagePullSecrets
	case entities2.Staging:
		aux.Spec.Template.Spec.ImagePullSecrets = StagingImagePullSecrets
	case entities2.Development:
		aux.Spec.Template.Spec.ImagePullSecrets = DevImagePullSecrets
	}
	return aux
}

// launchComponent triggers the creation of a given component from a YAML file
func (lc *LaunchComponents) launchComponent(componentPath string, targetEnvironment entities2.TargetEnvironment) derrors.Error {
	log.Debug().
		Str("path", componentPath).
		Str("targetEnvironment", entities2.TargetEnvironmentToString[targetEnvironment]).
		Msg("launch component")

	raw, err := ioutil.ReadFile(componentPath)
	if err != nil {
		return derrors.AsError(err, "cannot read component file")
	}
	log.Debug().Msg("parsing component")

	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode([]byte(raw), nil, nil)
	if err != nil {
		fmt.Printf("%#v", err)
	}

	switch o := obj.(type) {
	case *batchV1.Job:
		return lc.CreateJob(obj.(*batchV1.Job))
	case *appsv1.Deployment:
		return lc.CreateDeployment(lc.adaptDeployment(obj.(*appsv1.Deployment), targetEnvironment))
	case *appsv1.DaemonSet:
		return lc.launchDaemonSet(obj.(*appsv1.DaemonSet))
	case *v1.Service:
		return lc.CreateService(obj.(*v1.Service))
	case *v1.Secret:
		return lc.launchSecret(obj.(*v1.Secret))
	case *v1.ServiceAccount:
		return lc.CreateServiceAccount(obj.(*v1.ServiceAccount))
	case *v1.ConfigMap:
		return lc.CreateConfigMap(obj.(*v1.ConfigMap))
	case *rbacv1.RoleBinding:
		return lc.CreateRoleBinding(obj.(*rbacv1.RoleBinding))
	case *rbacv1.ClusterRole:
		return lc.CreateClusterRole(obj.(*rbacv1.ClusterRole))
	case *rbacv1.ClusterRoleBinding:
		return lc.CreateClusterRoleBinding(obj.(*rbacv1.ClusterRoleBinding))
	case *policyv1beta1.PodSecurityPolicy:
		return lc.launchPodSecurityPolicy(obj.(*policyv1beta1.PodSecurityPolicy))
	case *v1.PersistentVolume:
		return lc.launchPersistentVolume(obj.(*v1.PersistentVolume))
	case *v1.PersistentVolumeClaim:
		return lc.launchPersistentVolumeClaim(obj.(*v1.PersistentVolumeClaim))
	case *policyv1beta1.PodDisruptionBudget:
		return lc.launchPodDisruptionBudget(obj.(*policyv1beta1.PodDisruptionBudget))
	case *appsv1.StatefulSet:
		return lc.launchStatefulSet(obj.(*appsv1.StatefulSet))
	case *v1beta1.Ingress:
		return lc.launchIngress(obj.(*v1beta1.Ingress))
	default:
		log.Warn().Str("type", reflect.TypeOf(o).String()).Msg("Unknown entity")
		return derrors.NewUnimplementedError("object not supported").WithParams(o)
	}

	return derrors.NewInternalError("no case was executed")
}

// LaunchDaemonSet creates a Kubernetes DaemonSet.
func (lc *LaunchComponents) launchDaemonSet(daemonSet *appsv1.DaemonSet) derrors.Error {
	client := lc.Client.AppsV1().DaemonSets(daemonSet.Namespace)
	log.Debug().Interface("daemonSet", daemonSet).Msg("unmarshalled")
	created, err := client.Create(daemonSet)
	if err != nil {
		return derrors.AsError(err, "cannot create daemon set")
	}
	log.Debug().Interface("created", created).Msg("new daemon set has been created")
	return nil
}

// LaunchPodSecurityPolicy creates a Kubernetes PodSecurityPolicy.
func (lc *LaunchComponents) launchPodSecurityPolicy(policy *policyv1beta1.PodSecurityPolicy) derrors.Error {
	client := lc.Client.PolicyV1beta1().PodSecurityPolicies()
	log.Debug().Interface("policy", policy).Msg("unmarshalled")
	created, err := client.Create(policy)
	if err != nil {
		return derrors.AsError(err, "cannot create pod security policy")
	}
	log.Debug().Interface("created", created).Msg("new pod security policy has been created")
	return nil
}

// LaunchSecret creates a Kubernetes Secret.
func (lc *LaunchComponents) launchSecret(secret *v1.Secret) derrors.Error {
	client := lc.Client.CoreV1().Secrets(secret.Namespace)
	log.Debug().Interface("secret", secret).Msg("unmarshalled")
	created, err := client.Create(secret)
	if err != nil {
		return derrors.AsError(err, "cannot create secret")
	}
	log.Debug().Interface("created", created).Msg("new secret has been created")
	return nil
}

// createNamespace creates a Kubernetes namespace.
func (lc *LaunchComponents) createNamespace(name string) derrors.Error {
	namespaceClient := lc.Client.CoreV1().Namespaces()
	opts := metaV1.ListOptions{}
	list, err := namespaceClient.List(opts)
	if err != nil {
		return derrors.AsError(err, "cannot obtain the namespace list")
	}
	found := false
	for _, n := range list.Items {
		log.Debug().Interface("n", n).Msg("A namespace")
		if n.Name == name {
			found = true
			break
		}
	}

	if !found {
		toCreate := v1.Namespace{
			ObjectMeta: metaV1.ObjectMeta{
				Name: name,
			},
		}
		created, err := namespaceClient.Create(&toCreate)
		if err != nil {
			return derrors.AsError(err, "cannot create namespace")
		}
		log.Debug().Interface("created", created).Msg("namespaces has been created")
	} else {
		log.Debug().Str("namespace", name).Msg("namespace already exists")
	}
	return nil
}

// LaunchPersistenceVolume creates a Kubernetes PersistenceVolume.
func (lc *LaunchComponents) launchPersistentVolume(pv *v1.PersistentVolume) derrors.Error {
	client := lc.Client.CoreV1().PersistentVolumes()

	if lc.PlatformType == grpc_installer_go.Platform_AZURE.String() {
		log.Debug().Msg("Modifying storageClass")
		sc := AzureStorageClass
		pv.Spec.StorageClassName = sc
	}

	log.Debug().Interface("pv", pv).Msg("unmarshalled")
	created, err := client.Create(pv)
	if err != nil {
		return derrors.AsError(err, "cannot create persistent volume")
	}
	log.Debug().Interface("created", created).Msg("new persistent volume has been created")
	return nil
}

// LaunchPersistenceVolumeClaim creates a Kubernetes PersistentVolumeClaim.
func (lc *LaunchComponents) launchPersistentVolumeClaim(pvc *v1.PersistentVolumeClaim) derrors.Error {
	client := lc.Client.CoreV1().PersistentVolumeClaims(pvc.Namespace)

	if lc.PlatformType == grpc_installer_go.Platform_AZURE.String() {
		log.Debug().Msg("Modifying storageClass")
		sc := AzureStorageClass
		pvc.Spec.StorageClassName = &sc
	}

	log.Debug().Interface("pvc", pvc).Msg("unmarshalled")
	created, err := client.Create(pvc)
	if err != nil {
		return derrors.AsError(err, "cannot create persistent volume claim")
	}
	log.Debug().Interface("created", created).Msg("new persistent volume claim has been created")
	return nil
}

func (lc *LaunchComponents) launchPodDisruptionBudget(pdb *policyv1beta1.PodDisruptionBudget) derrors.Error {
	client := lc.Client.PolicyV1beta1().PodDisruptionBudgets(pdb.Namespace)
	log.Debug().Interface("pdb", pdb).Msg("unmarshalled")
	created, err := client.Create(pdb)
	if err != nil {
		return derrors.AsError(err, "cannot create pod disruption budget")
	}
	log.Debug().Interface("created", created).Msg("new pod disruption budget")
	return nil
}

func (lc *LaunchComponents) launchStatefulSet(ss *appsv1.StatefulSet) derrors.Error {
	client := lc.Client.AppsV1().StatefulSets(ss.Namespace)
	log.Debug().Interface("ss", ss).Msg("unmarshalled")
	created, err := client.Create(ss)
	if err != nil {
		return derrors.AsError(err, "cannot create stateful set")
	}
	log.Debug().Interface("created", created).Msg("new stateful set")
	return nil
}

func (lc *LaunchComponents) launchIngress(ingress *v1beta1.Ingress) derrors.Error {
	client := lc.Client.ExtensionsV1beta1().Ingresses(ingress.Namespace)
	log.Debug().Interface("ingress", ingress).Msg("unmarshalled")
	created, err := client.Create(ingress)
	if err != nil {
		return derrors.AsError(err, "cannot create ingress")
	}
	log.Debug().Interface("created", created).Msg("new ingress set")
	return nil
}

func (lc *LaunchComponents) String() string {
	return fmt.Sprintf("SYNC LaunchComponents from %s", lc.ComponentsDir)
}

func (lc *LaunchComponents) PrettyPrint(indentation int) string {
	simpleIden := strings.Repeat(" ", indentation) + "  "
	entrySep := simpleIden + "  "
	cStr := ""
	for _, c := range lc.ListComponents() {
		cStr = cStr + "\n" + entrySep + c
	}
	return strings.Repeat(" ", indentation) + lc.String() + cStr
}

func (lc *LaunchComponents) UserString() string {
	return fmt.Sprintf("Launching K8s components from %s for %s", lc.ComponentsDir, lc.Environment)
}
