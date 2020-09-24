// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package cmd

import (
	"github.com/spf13/cobra"
)

// CMTARGETSALT specifies the CM tool that we are targeting.
const CMTARGETSALT = "salt"

// saltCmd represents the salt command
var saltCmd = &cobra.Command{
	Use:   "salt",
	Short: "Create MOSE payload for salt",
	Long:  `Create MOSE payload for salt`,
	Run: func(cmd *cobra.Command, args []string) {
		UserInput.CMTarget = CMTARGETSALT
		UserInput.SetLocalIP()
		UserInput.GenerateParams()
		UserInput.GeneratePayload()
		UserInput.StartTakeover()
	},
}

func init() {
	rootCmd.AddCommand(saltCmd)
}
