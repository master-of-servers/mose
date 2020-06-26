// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package cmd

import (
	"github.com/spf13/cobra"
)

var CMTARGETANSIBLE = "ansible"

// ansibleCmd represents the ansible command
var ansibleCmd = &cobra.Command{
	Use:   "ansible",
	Short: "Create MOSE payload for ansible takeover",
	Long:  `Create MOSE payload for ansible takeover`,
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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// ansibleCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// ansibleCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
