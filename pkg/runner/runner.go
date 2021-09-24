package runner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
	"github.com/thoas/go-funk"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/config"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/operationplanner"
)

var extractorRegexes = [...]*regexp.Regexp{
	regexp.MustCompile(`(?i)\s*#\s+[\w\[\]\-._[:cntrl:]]+\s+will\s+be`),
	regexp.MustCompile(`(?i)\s*Plan:`),
	regexp.MustCompile(`(?i)warning`),
	regexp.MustCompile(`(?i)error`),
}

type Runner struct {
	plan             *operationplanner.OperationPlan
	Command          string
	ExtractedOutputs map[string]string
	logger           log.Logger
}

func New(plan *operationplanner.OperationPlan, command string) *Runner {
	return &Runner{
		plan:             plan,
		ExtractedOutputs: map[string]string{},
		logger:           log.NewLogger(),
		Command:          command,
	}
}

func (r *Runner) runCommand(op operationplanner.DirOperation) (string, error) {
	f := command.NewFactory(env.NewRepository())
	opts := &command.Opts{
		Dir: op.Dir,
		Env: []string{},
	}

	cmd := f.Create("terragrunt", []string{r.Command}, opts)

	if out, err := cmd.RunAndReturnTrimmedCombinedOutput(); err != nil {
		r.logger.Warnf(out)
		return out, fmt.Errorf("running %s in %s: err: %w", r.Command, op.Dir, err)
	} else {
		r.logger.Infof(out)
		return createCommandSummary(op, r.Command, r.plan, extractCommandOutputLines(out)), nil
	}
}

func createCommandSummary(op operationplanner.DirOperation, command string, plan *operationplanner.OperationPlan, outputLines []string) string {
	return fmt.Sprintf("### Operation \"%s\" key info:\n(in directory %s)\n\n%s\n", command, strings.TrimPrefix(op.Dir, plan.CommonRoot), strings.Join(outputLines, "\n...\n"))
}

func extractCommandOutputLines(optext string) []string {
	lines := strings.Split(optext, "\n")
	var matchingLines []string
	for _, line := range lines {
		if funk.Contains(extractorRegexes, func(r *regexp.Regexp) bool {
			return r.MatchString(line)
		}) {
			matchingLines = append(matchingLines, "> "+line)
		}
	}
	return matchingLines
}

func (r *Runner) runBatch(b operationplanner.OperationBatch) error {
	logger := log.NewLogger()
	for _, op := range b {
		if op.Operation == operationplanner.OperationScan {
			r.logger.Debugf("Skipping scan")
			continue
		}
		if op.Operation == operationplanner.OperationDestroy {
			if r.Command != config.CommandApply {
				r.logger.Infof("Skipping destroy operation when command is %s", r.Command)
				continue
			}
		}
		logger.Infof("Running operation (command \"%s\") in %s", r.Command, strings.TrimPrefix(op.Dir, r.plan.CommonRoot))

		if optext, err := r.runCommand(op); err != nil {
			return fmt.Errorf("running operation: %w", err)
		} else if len(optext) > 0 {
			r.ExtractedOutputs[op.Dir] = optext
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
