package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "redpen",
	Short: "Pull GitHub PR review comments into structured JSON + markdown",
	Long: `A tool for fetching GitHub PR review comments, reviews, and discussion
into structured JSON docs guide.

Example:
  redpen fetch --repo org/repo --pr-author user`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Int("parallelism", 4, "number of parallel PR fetches")
	rootCmd.PersistentFlags().Bool("force", false, "ignore cache and re-fetch everything")

	mustBindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	mustBindPFlag("parallelism", rootCmd.PersistentFlags().Lookup("parallelism"))
	mustBindPFlag("force", rootCmd.PersistentFlags().Lookup("force"))
}

func mustBindPFlag(key string, flag *pflag.Flag) {
	if err := viper.BindPFlag(key, flag); err != nil {
		panic(fmt.Sprintf("viper.BindPFlag(%q): %v", key, err))
	}
}

func initConfig() error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("REDPEN")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("reading config: %w", err)
		}
	}

	level := zerolog.InfoLevel
	switch strings.ToLower(viper.GetString("log-level")) {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"},
	).With().Timestamp().Logger()

	return nil
}
