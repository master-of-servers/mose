// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/master-of-servers/mose/pkg/chefutils"
	"github.com/master-of-servers/mose/pkg/moseutils"
	"os"

	"github.com/spf13/cobra"
)

const CMTARGETCHEF = "chef"

// chefCmd represents the chef command
var chefCmd = &cobra.Command{
	Use:   "chef",
	Short: "Create MOSE payload for chef takeover",
	Long:  `Create MOSE payload for chef takeover`,
	Run: func(cmd *cobra.Command, args []string) {
		UserInput.CMTarget = CMTARGETCHEF
		UserInput.SetLocalIP()
		UserInput.GenerateParams()
		UserInput.GeneratePayload()
		UserInput.StartTakeover()
		ans, err := moseutils.AskUserQuestion("Is your target a chef workstation? ", UserInput.OSTarget)
		if err != nil {
			log.Fatal().Err(err).Msg("Quitting")
		}
		if ans {
			log.Info().Msg("Nothing left to do locally, continue all remaining activities on the target workstation.")
			os.Exit(0)
		}

		ans, err = moseutils.AskUserQuestion("Is your target a chef server? ", UserInput.OSTarget)
		if err != nil {
			log.Fatal().Msg("Quitting")
		}
		if ans {
			chefutils.SetupChefWorkstationContainer(UserInput)
			os.Exit(0)
		} else {
			log.Error().Msg("Invalid chef target")
		}
	},
}

func init() {
	rootCmd.AddCommand(chefCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// chefCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// chefCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
