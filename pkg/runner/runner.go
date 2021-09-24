package runner

import (
	"fmt"
	"regexp"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/operationplanner"
)

type Runner struct {
	plan        *operationplanner.OperationPlan
	Command     string
	PlanOutputs map[string]string
	logger      log.Logger
}

func New(plan *operationplanner.OperationPlan, command string) *Runner {
	return &Runner{
		plan:        plan,
		PlanOutputs: map[string]string{},
		logger:      log.NewLogger(),
		Command:     command,
	}
}

func (r *Runner) runCommand(dir string) (string, error) {
	f := command.NewFactory(env.NewRepository())
	opts := &command.Opts{
		Dir: dir,
		Env: []string{},
	}

	cmd := f.Create("terragrunt", []string{r.Command}, opts)

	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		return out, fmt.Errorf("running plan in %s: err: %w", dir, err)
	} else {
		return extractCommandOutput(out), nil
	}
}

func extractCommandOutput(optext string) string {
	r := regexp.MustCompile(`(?s).*Terraform\s+will\s+[^:]+:\s+(.+)`)

	match := r.FindStringSubmatch(optext)
	if len(match) < 2 {
		return ""
	}

	return match[1]
}

func (r *Runner) runBatch(b operationplanner.OperationBatch) error {
	logger := log.NewLogger()
	for _, op := range b {
		if op.Operation != operationplanner.OPERATION_RUN {
			continue
		}
		logger.Infof("Running operation in %s", op.Dir)

		if optext, err := r.runCommand(op.Dir); err != nil {
			r.logger.Warnf(optext)
			return fmt.Errorf("running operation: %w", err)
		} else if len(optext) > 0 {
			r.logger.Debugf(optext)
			r.PlanOutputs[op.Dir] = optext
		}
	}

	return nil
}

func (r *Runner) Run() error {
	logger := log.NewLogger()
	for i, batch := range r.plan.OperationBatches {
		logger.Infof("Running batch %d", i)

		if err := r.runBatch(batch); err != nil {
			return err
		}
	}

	return nil
}
