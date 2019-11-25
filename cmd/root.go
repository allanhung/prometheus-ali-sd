/*
Copyright © 2019 Allan Hung <hung.allan@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var cfgFile string
var logLevel string
var logFile string
var logger *logrus.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "prometheus-ali-sd",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogger)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.prometheus-ali-sd.yaml)")
	rootCmd.PersistentFlags().StringVar(&logFile, "logfile", "", "log file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "loglevel", "info", "log level  [trace, debug, info, warn, error, fatal, panic] (default info)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".prometheus-ali-sd" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".prometheus-ali-sd")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func initLogger() {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		fmt.Println("Can't parse log level:", err)
		os.Exit(1)
	}
	logger = &logrus.Logger{
		Level: level,
		Formatter: &logrus.TextFormatter{
			DisableColors:   false,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		},
	}
	logger.SetLevel(level)
	logger.Out = os.Stdout
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			logger.Out = file
		} else {
			logger.Errorf("Unable log to file: %s, using default stdout\n", logFile)
		}
	}
	logger.Debug("log level set to: ", logLevel)
}