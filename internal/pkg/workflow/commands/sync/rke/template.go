/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package rke

import (
	"bytes"
	"gopkg.in/yaml.v2"
	"html/template"

	"github.com/nalej/derrors"
)

// ClusterTemplate contains the YAML template for the cluster configuration required by RKE.
// Notice that the version of kubernetes and their associated images has been extracted from:
// https://github.com/rancher/types/blob/master/apis/management.cattle.io/v3/k8s_defaults.go#L14
// TODO Check roles depending on number of nodes: 1, 2, 3, 3+ with ssh_key on nodes or at cluster level.
const ClusterTemplate string = `
# Autogenerated by Inframgr installer.
# Do not modify this file

# Target nodes
nodes:
{{ range $index, $targetNode := .TargetNodes }}
- address: "{{$targetNode}}"
  user: "{{$.NodeUsername}}"
{{if lt $index 3 }}  role: ["etcd", "controlplane", "worker"]
  labels:
    nalej.com/role: "management"
{{else}}  role: ["worker"]
  labels:
    nalej.com/role: "compute"{{end}}
{{end}}

# Cluster level SSH private key
ssh_key_path: "{{$.PrivateKeyPath}}"

# Set the name of the Kubernetes cluster  
cluster_name: "{{$.ClusterName}}"

# Kubernetes version to be installed
kubernetes_version: v1.9.7-rancher2-1

# TODO:
# Provisioner needs un-escalated RunAsUser (what user id?)
addons: |-
  ---
  apiVersion: v1
  kind: Namespace
  metadata:
    name: nalej

`

// RKETemplate structure with the template content.
type RKETemplate struct {
	content string
}

// NewRKETemplate creates a new RKETemplate with a given template.
func NewRKETemplate(content string) *RKETemplate {
	return &RKETemplate{content}
}

// ParseTemplate processes the golang templating on the RKE template and
// returns a string with the content of the file.
func (t *RKETemplate) ParseTemplate(config *ClusterConfig) (string, derrors.Error) {
	ft := template.New("RKE cluster.yaml")
	ft, err := ft.Parse(t.content)
	if err != nil {
		return "", derrors.NewInternalError("cannot parse workflow template file", err)
	}
	buf := new(bytes.Buffer)
	err = ft.Execute(buf, *config)
	if err != nil {
		return "", derrors.NewInternalError("cannot parse RKE cluster template file", err)
	}
	return buf.String(), nil
}

// ValidateYAML checks if a given content can be parsed as YAML.
func (t *RKETemplate) ValidateYAML(content string) derrors.Error {
	m := make(map[interface{}]interface{})
	err := yaml.Unmarshal([]byte(content), &m)
	return derrors.AsError(err, "invalid YAML file")
}
