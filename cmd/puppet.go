// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package cmd

import (
	"github.com/spf13/cobra"
)

// CMTARGETPUPPET specifies the CM tool that we are targeting.
const CMTARGETPUPPET = "puppet"

// puppetCmd represents the puppet command
var puppetCmd = &cobra.Command{
	Use:   "puppet",
	Short: "Create MOSE payload for puppet",
	Long:  `Create MOSE payload for puppet`,
	Run: func(cmd *cobra.Command, args []string) {
		UserInput.CMTarget = CMTARGETPUPPET
		UserInput.SetLocalIP()
		UserInput.GenerateParams()
		UserInput.GeneratePayload()
		UserInput.StartTakeover()
	},
}

func init() {
	rootCmd.AddCommand(puppetCmd)
}
