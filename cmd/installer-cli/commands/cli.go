/*
 * Copyright 2018 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package commands

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var installKubernetes bool

var cliCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Nalej platform",
	Long:  `Install the Nalej platform`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		log.Info().Msg("Launching API!")
	},
}

func init() {
	rootCmd.AddCommand(cliCmd)
}