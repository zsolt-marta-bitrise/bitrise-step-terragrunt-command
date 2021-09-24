package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/config"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/git"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/operationplanner"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/runner"
)

func run() error {
	logger := log.NewLogger()

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

	p := operationplanner.New(changedFiles, cfg.WorkDir, cfg.Command)
	plan, err := p.PlanOperation()
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}

	logger.Infof("\n=================================================\n\n")
	logger.Infof(plan.GetSummary())

	logger.Infof("\n=================================================\n\n")
	logger.Infof("Running operations in order\n")
	r := runner.New(plan, g, cfg.Command, cfg.BaseBranch)
	if err := r.Run(); err != nil {
		return fmt.Errorf("run operation: %w", err)
	}

	logger.Infof("\n=================================================\n\n")
	logger.Infof(r.GetSummary())

	outputCommand := exec.Command("envman", "add", "--key", "COMMAND_OUTPUT", "--value", constructOutput(plan, r))
	if err := outputCommand.Run(); err != nil {
		return fmt.Errorf("export output with envman: %w", err)
	}

	return nil
}

func constructOutput(plan *operationplanner.OperationPlan, runner *runner.Runner) string {
	return fmt.Sprintf(`
===================   TERRAGRUNT %s  ========================

%s

=======================  RESULTS  ===========================

%s
`, plan.Command, plan.GetSummary(), runner.GetSummary())
}

func main() {
	if err := run(); err != nil {
		log.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
