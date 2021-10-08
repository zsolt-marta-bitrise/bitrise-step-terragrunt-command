package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/config"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/git"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/operationplanner"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/runner"
)

func run() error {
	logger := log.NewLogger()
	logger.EnableDebugLog(true)

	cfg, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("new gitops config: %w", err)
	}
	stepconf.Print(cfg)

	g := git.New(cfg.RepositoryUrl, cfg.WorkDir, logger)
	changedFiles, err := g.GetChangedFiles(cfg.BaseBranch)
	if err != nil {
		return fmt.Errorf("get changed files: %w", err)
	}

	p := operationplanner.New(changedFiles, cfg.WorkDir, cfg.Command, logger)
	plan, err := p.PlanOperation()
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}

	logger.Infof("\n=================================================\n\n")
	logger.Infof(plan.GetSummary())

	sigs := make(chan os.Signal, 1)
	cancelChan := make(chan bool)
	errChan := make(chan error)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigs:
			logger.Infof("Operation cancelled.")
			cancelChan <- true
		}
	}()

	logger.Infof("\n=================================================\n\n")
	logger.Infof("Running operations in order\n")
	go func() {
		r := runner.New(plan, g, cfg.Command, cfg.BaseBranch, logger, cancelChan)
		if err := r.Run(); err != nil {
			errChan <- fmt.Errorf("run operation: %w", err)
			return
		}

		logger.Infof("\n=================================================\n\n")
		logger.Infof(r.GetSummary())

		outputCommand := exec.Command("envman", "add", "--key", "COMMAND_OUTPUT", "--value", constructOutput(plan, r))
		if err := outputCommand.Run(); err != nil {
			errChan <- fmt.Errorf("export output with envman: %w", err)
			return
		}

		errChan <- nil
	}()

	err = <-errChan
	close(cancelChan)

	return err
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
