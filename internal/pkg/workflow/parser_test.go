/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package workflow

import (
	"fmt"
	"github.com/nalej/installer/internal/pkg/workflow/commands/sync"
	"github.com/nalej/installer/internal/pkg/workflow/entities"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

const basicDefinitionNoTemplate = `
{
 "description": "basicDefinitionNoTemplate",
 // This is a comment
 "commands": [
  {"type":"sync", "name": "exec", "cmd": "cmd1"},
  {"type":"sync", "name": "exec", "cmd": "cmd2", "args":["arg1"]}
 ]
}
`

const basicDefinitionTwoCommands = `
{
 "description": "basicDefinitionNoTemplate",
 "commands": [
  {"type":"sync", "name": "exec", "cmd": "cmd1"},
  {"type":"sync", "name": "scp", "targetHost": "127.0.0.1", "credentials":{"username": "username", "password":"passwd", "privateKey":""}, "source":"script.sh", "destination":"/opt/scripts/."}
 ]
}
`

const basicTemplateIteration = `
{
 "description": "basicTemplateIteration",
 "commands": [
  {"type":"sync", "name": "exec", "cmd": "generalCmd", "args":["{{.InstallRequest.InstallId}}", "{{.InstallRequest.ClusterId}}"]}
  {{range $index, $node := .InstallRequest.Nodes }}
  ,{"type":"sync", "name": "exec", "cmd": "cmd{{$index}}", "args":["{{$node}}"]}
  {{end}}
 ]
}
`

var _ = ginkgo.Describe("Parser", func() {
	var parser = NewParser()

	ginkgo.Context("parses a workflow not requiring the template", func(){
		workflow, err := parser.ParseWorkflow(basicDefinitionNoTemplate, "TestParseWorkflow_Basic", EmptyParameters)
		ginkgo.It("must contain cmd1", func(){
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(workflow).ToNot(gomega.BeNil())
			gomega.Expect(len((*workflow).Commands), gomega.Equal(2))
			cmd1 := (*workflow).Commands[0]
			gomega.Expect(cmd1.(*sync.Exec).Cmd).To(gomega.Equal("cmd1"))
		})
	})

	ginkgo.Context("parses a workflow iterating through the nodes", func(){
		numNodes := 10
		params := GetTestParameters(numNodes, true)
		workflow, err := parser.ParseWorkflow(basicTemplateIteration, "TestParseWorkflow_SimpleTemplate", *params)
		ginkgo.It("must have iterated through the nodes", func(){
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(workflow, gomega.Not(gomega.BeNil()))
			gomega.Expect(len(workflow.Commands), gomega.Equal(numNodes + 1))
			firstCmd := (*workflow).Commands[0]
			gomega.Expect(firstCmd.(*sync.Exec).Args[0]).To(gomega.Equal(params.InstallRequest.InstallId))
			gomega.Expect(firstCmd.(*sync.Exec).Args[1]).To(gomega.Equal(params.InstallRequest.ClusterId))
			for i := 0; i < numNodes; i++ {
				toCheck := (*workflow).Commands[i+1]
				gomega.Expect(toCheck.(*sync.Exec).Cmd).To(gomega.Equal(fmt.Sprintf("cmd%d", i)))
				gomega.Expect(toCheck.(*sync.Exec).Args[0]).To(gomega.Equal(fmt.Sprintf("10.1.1.%d", i)))
			}
		})

	})

	ginkgo.Context("parses a simple workflow with two different commands", func(){
		workflow, err := parser.ParseWorkflow(basicDefinitionTwoCommands, "TestParseWorkflow_TwoCommands", EmptyParameters)
		ginkgo.It("must be returned and contain the Exec and SCP commands", func(){
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(workflow, gomega.Not(gomega.BeNil()))
			cmd1 := (*workflow).Commands[0]
			gomega.Expect(cmd1.Name()).To(gomega.Equal(entities.Exec))
			gomega.Expect(cmd1.(*sync.Exec).Cmd).To(gomega.Equal("cmd1"))
			cmd2 := (*workflow).Commands[1]
			gomega.Expect(cmd2.Name()).To(gomega.Equal(entities.SCP))
			gomega.Expect(cmd2.(*sync.SCP).TargetHost).To(gomega.Equal("127.0.0.1"))
		})
	})
})
