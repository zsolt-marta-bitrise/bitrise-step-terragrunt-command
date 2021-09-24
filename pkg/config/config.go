package config

import (
	"fmt"

	"github.com/bitrise-io/go-utils/env"
	"github.com/thoas/go-funk"

	"github.com/bitrise-io/go-steputils/stepconf"
)

const (
	CommandPlan     = "plan"
	CommandApply    = "apply"
	CommandValidate = "validate"
)

type Config struct {
	WorkDir       string `env:"work_dir,required"`
	BaseBranch    string `env:"base_branch,required"`
	Command       string `env:"command,required"`
	RepositoryUrl string `env:"repo_url,required"`
}

// NewConfig returns a new configuration initialized from environment variables.
func NewConfig() (Config, error) {
	var cfg Config
	if err := stepconf.NewInputParser(env.NewRepository()).Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse step config: %w", err)
	}

	if !funk.Contains([]string{CommandValidate, CommandPlan, CommandApply}, cfg.Command) {
		return Config{}, fmt.Errorf("invalid command: %s", cfg.Command)
	}

	return cfg, nil
}
