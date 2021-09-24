package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/config"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/git"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/operationplanner"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/runner"
)

func run() error {
	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("new gitops config: %w", err)
	}
	stepconf.Print(cfg)

	g := git.New(cfg.RepositoryUrl, cfg.WorkDir)
	changedFiles, err := g.GetChangedFiles(cfg.BaseBranch)
	if err != nil {
		return fmt.Errorf("get changed files: %w", err)
	}

	p := operationplanner.New(changedFiles, cfg.WorkDir)
	plan, err := p.PlanOperation()
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}

	r := runner.New(plan, cfg.Command)
	if err := r.Run(); err != nil {
		return fmt.Errorf("run operation: %w", err)
	}

	logger := log.NewLogger()
	logger.Infof("Plan for %d files:\n\n", len(r.PlanOutputs))

	for module, optext := range r.PlanOutputs {
		logger.Infof("======================================================================\n\n", module)
		logger.Infof("Terragrunt command output for %s\n\n", module)
		logger.Infof("%s\n\n", optext)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
