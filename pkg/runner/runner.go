package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
	"github.com/lunixbochs/vtclean"
	"github.com/thoas/go-funk"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/config"
	"github.com/zsolt-marta-bitrise/bitrise-step-terragrunt-command/pkg/operationplanner"
)

var extractorRegexes = [...]*regexp.Regexp{
	regexp.MustCompile(`(?i)#\s+[\w\[\]\-._"]+\s+will\s+be`),
	regexp.MustCompile(`(?i)plan:`),
	regexp.MustCompile(`(?i)warning`),
	regexp.MustCompile(`(?i)error`),
	regexp.MustCompile(`(?i)outputs`),
	regexp.MustCompile(`\s+(?:~|->|\+|-/\+|\+/-)\s+`),
	regexp.MustCompile(`(?i)no\s+changes`),
}

type codeRepository interface {
	GetChangedFiles(baseBranch string) ([]string, error)
	CheckoutBranch(branch string) error
	GetCurrentBranch() (string, error)
}

type Runner struct {
	plan             *operationplanner.OperationPlan
	codeRepository   codeRepository
	baseBranch       string
	Command          string
	CommandSummaries map[string]string
	logger           log.Logger
}

func New(plan *operationplanner.OperationPlan, codeRepository codeRepository, command string, baseBranch string, logger log.Logger) *Runner {
	return &Runner{
		plan:             plan,
		CommandSummaries: map[string]string{},
		logger:           logger,
		Command:          command,
		codeRepository:   codeRepository,
		baseBranch:       baseBranch,
	}
}

func containsHCLFile(dir string) (bool, error) {
	contents, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return funk.Contains(contents, func(fi os.FileInfo) bool {
		return filepath.Ext(fi.Name()) == ".hcl"
	}), nil
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
		r.logger.Printf(out)
		return createCommandSummary(op, r.Command, r.plan, extractCommandOutputLines(out)), nil
	}
}

func createCommandSummary(op operationplanner.DirOperation, command string, plan *operationplanner.OperationPlan, outputLines []string) string {
	return fmt.Sprintf("### Operation \"%s\" key points:\n(in directory %s)\n\n%s\n\n-------------------------------------\n\n",
		command,
		strings.TrimPrefix(op.Dir, plan.CommonRoot),
		strings.Join(outputLines, "\n...\n"))
}

func extractCommandOutputLines(optext string) []string {
	cleantext := vtclean.Clean(optext, false)
	lines := strings.Split(cleantext, "\n")
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

	originalBranch := ""
	if b.RunOnBaseBranch {
		var err error
		if originalBranch, err = r.codeRepository.GetCurrentBranch(); err != nil {
			return err
		}
		r.logger.Debugf("On branch %s", originalBranch)
		r.logger.Infof("Checking out base branch %s", r.baseBranch)
		if err := r.codeRepository.CheckoutBranch(r.baseBranch); err != nil {
			return fmt.Errorf("checkout base branch: %w", err)
		}
	}

	for _, op := range b.Operations {
		runnable, err := containsHCLFile(op.Dir)
		if err != nil {
			return err
		}
		if !runnable {
			r.logger.Infof("Skipping non-runnable directory %s", strings.TrimPrefix(op.Dir, r.plan.CommonRoot))
			continue
		}

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
			r.CommandSummaries[op.Dir] = optext
		}
	}

	if b.RunOnBaseBranch {
		r.logger.Infof("Checking out original branch %s", originalBranch)
		if err := r.codeRepository.CheckoutBranch(originalBranch); err != nil {
			return fmt.Errorf("checkout original branch: %w", err)
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

func (r *Runner) GetSummary() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Execution results of command \"%s\":\nRan in the root directory %s.\n\n", r.Command, r.plan.CommonRoot))
	for _, b := range r.plan.OperationBatches {
		for _, d := range b.Operations {
			builder.WriteString(r.CommandSummaries[d.Dir])
			builder.WriteString("")
		}
	}
	return builder.String()
}
