// Copyright 2020 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS,
// the U.S. Government retains certain rights in this software.

package cmd

import (
	"os"
	"path/filepath"

	"github.com/master-of-servers/mose/pkg/moseutils"
	"github.com/master-of-servers/mose/pkg/system"
	"github.com/master-of-servers/mose/pkg/userinput"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	// UserInput contains the parameters specified by the user
	UserInput userinput.UserInput
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "github.com/master-of-servers/mose",
	Short: "MOSE is a post exploitation tool for configuration management systems.",
	Long:  `MOSE is a post exploitation tool that enables security professionals with little or no experience with configuration management (CM) technologies to leverage them to compromise environments. CM tools, such as Puppet, Chef, Salt, and Ansible are used to provision systems in a uniform manner based on their function in a network. Upon successfully compromising a CM server, an attacker can use these tools to run commands on any and all systems that are in the CM server's inventory. However, if the attacker does not have experience with these types of tools, there can be a very time-consuming learning curve. MOSE allows an operator to specify what they want to run without having to get bogged down in the details of how to write code specific to a proprietary CM tool. It also automatically incorporates the desired commands into existing code on the system, removing that burden from the user. MOSE allows the operator to choose which assets they want to target within the scope of the server's inventory, whether this is a subset of clients or all clients. This is useful for targeting specific assets such as web servers or choosing to take over all of the systems in the CM server's inventory.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $PWD/.settings.yaml)")

	rootCmd.PersistentFlags().StringP("osarch", "a", "", "Architecture that the target CM tool is running on")
	rootCmd.PersistentFlags().StringP("cmd", "c", "", "Command to run on the targets")
	rootCmd.PersistentFlags().Bool("debug", false, "Display debug output")
	rootCmd.PersistentFlags().Int("exfilport", 9090, "Port used to exfil data from chef server (default 9090, 443 with SSL)")
	rootCmd.PersistentFlags().StringP("filepath", "f", "", "Output binary locally at <filepath>")
	rootCmd.PersistentFlags().StringP("fileupload", "u", "", "File upload option")
	rootCmd.PersistentFlags().StringP("localip", "l", "", "Local IP Address")
	rootCmd.PersistentFlags().StringP("payloadname", "m", "my_cmd", "Name for backdoor payload")
	rootCmd.PersistentFlags().StringP("ostarget", "o", "linux", "Operating system that the target CM server is on")
	rootCmd.PersistentFlags().Int("websrvport", 8090, "Port used to serve payloads (default 8090, 443 with SSL)")
	rootCmd.PersistentFlags().String("remoteuploadpath", "/root/.definitelynotevil", "Remote file path to upload a script to (used in conjunction with -fu)")
	rootCmd.PersistentFlags().StringP("rhost", "r", "", "Set the remote host for /etc/hosts in the chef workstation container (format is hostname:ip)")
	rootCmd.PersistentFlags().Bool("ssl", false, "Serve payload over TLS")
	rootCmd.PersistentFlags().Int("tts", 60, "Number of seconds to serve the payload")
	rootCmd.PersistentFlags().Bool("nocolor", false, "Disable colors for mose")

	path := system.Gwd()
	rootCmd.PersistentFlags().String("payloads", filepath.Join(path, "payloads"), "Location of payloads output by mose")
	rootCmd.PersistentFlags().String("basedir", path, "Location of payloads output by mose")

	viper.BindPFlag("osarch", rootCmd.PersistentFlags().Lookup("osarch"))
	viper.BindPFlag("cmd", rootCmd.PersistentFlags().Lookup("cmd"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("exfilport", rootCmd.PersistentFlags().Lookup("exfilport"))
	viper.BindPFlag("filepath", rootCmd.PersistentFlags().Lookup("filepath"))
	viper.BindPFlag("fileupload", rootCmd.PersistentFlags().Lookup("fileupload"))
	viper.BindPFlag("localip", rootCmd.PersistentFlags().Lookup("localip"))
	viper.BindPFlag("payloadname", rootCmd.PersistentFlags().Lookup("payloadname"))
	viper.BindPFlag("ostarget", rootCmd.PersistentFlags().Lookup("ostarget"))
	viper.BindPFlag("websrvport", rootCmd.PersistentFlags().Lookup("websrvport"))
	viper.BindPFlag("remoteuploadpath", rootCmd.PersistentFlags().Lookup("remoteuploadpath"))
	viper.BindPFlag("rhost", rootCmd.PersistentFlags().Lookup("rhost"))
	viper.BindPFlag("ssl", rootCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("tts", rootCmd.PersistentFlags().Lookup("tts"))
	viper.BindPFlag("nocolor", rootCmd.PersistentFlags().Lookup("nocolor"))

	viper.BindPFlag("payloads", rootCmd.PersistentFlags().Lookup("payloads"))
	viper.BindPFlag("basedir", rootCmd.PersistentFlags().Lookup("basedir"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find current directory.
		cur, err := os.Getwd()
		if err != nil {
			log.Error().Err(err).Msg("")
			os.Exit(1)
		}

		// Search config in home directory with name ".github.com/master-of-servers/mose" (without extension).
		viper.AddConfigPath(cur)
		viper.SetConfigType("yaml")
		viper.SetConfigName("settings")
	}
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.Error().Err(err).Msg("Error reading in config file")
	}

	err := viper.Unmarshal(&UserInput)

	if UserInput.Cmd == "" && UserInput.FileUpload == "" {
		log.Fatal().Msg("You must specify a CM target and a command or file to upload.")
	}

	if UserInput.Cmd != "" && UserInput.FileUpload != "" {
		log.Fatal().Msg("You must specify a CM target, a command or file to upload, and an operating system.")
	}

	// Set port option for webserver
	if UserInput.ServeSSL && UserInput.WebSrvPort == 8090 {
		UserInput.WebSrvPort = 443
	}

	// Set port option for exfilling files from a Chef Server
	if UserInput.ServeSSL && UserInput.ExfilPort == 9090 {
		UserInput.ExfilPort = 443
	}

	if err != nil {
		log.Error().Err(err).Msg("Error unmarshalling config file")
	}
	moseutils.NOCOLOR = UserInput.NoColor
	moseutils.SetupLogger(UserInput.Debug)
}
