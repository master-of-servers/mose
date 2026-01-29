// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package cmd

import (
	"github.com/spf13/cobra"
)

// CMTARGETANSIBLE specifies the CM tool that we are targeting.
const CMTARGETANSIBLE = "ansible"

// ansibleCmd represents the ansible command
var ansibleCmd = &cobra.Command{
	Use:   "ansible",
	Short: "Create MOSE payload for ansible",
	Long:  `Create MOSE payload for ansible`,
	Run: func(cmd *cobra.Command, args []string) {
		UserInput.CMTarget = CMTARGETANSIBLE
		UserInput.SetLocalIP()
		UserInput.GenerateParams()
		UserInput.GeneratePayload()
		UserInput.StartTakeover()
	},
}

func init() {
	rootCmd.AddCommand(ansibleCmd)
}
