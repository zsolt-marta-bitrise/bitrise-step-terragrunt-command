package git

import (
	"fmt"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/retry"
)

type Git struct {
	workDir string
	gitURL  string
	logger  log.Logger
}

func New(gitURL string, workDir string, logger log.Logger) *Git {
	return &Git{
		workDir: workDir,
		gitURL:  gitURL,
		logger:  logger,
	}
}

func (git *Git) command(args ...string) command.Command {
	factory := command.NewFactory(env.NewRepository())
	opts := &command.Opts{
		Dir: git.workDir,
		Env: []string{},
	}
	return factory.Create("git", args, opts)
}

func (git *Git) fetchBaseBranch(baseBranch string) error {
	if err := retry.Times(2).Wait(3 * time.Second).Try(func(attempt uint) error {
		return git.command("fetch", git.gitURL, baseBranch).Run()
	}); err != nil {
		return fmt.Errorf("failed to git-fetch (url: %s) error: %w",
			git.gitURL, err)
	}

	return nil
}

func (git *Git) GetChangedFiles(baseBranch string) ([]string, error) {
	if err := git.fetchBaseBranch(baseBranch); err != nil {
		return []string{}, fmt.Errorf("failed to fetch base branch: %w", err)
	}

	diffOutput, err := git.command("diff", fmt.Sprintf("%s...", baseBranch), "--name-only").RunAndReturnTrimmedOutput()
	if err != nil {
		git.logger.Warnf(diffOutput)
		return []string{}, fmt.Errorf("failed to diff git, err: %w", err)
	}

	diffArr := strings.Split(diffOutput, "\n")

	return diffArr, nil
}

func (git *Git) CheckoutBranch(branch string) error {
	output, err := git.command("checkout", branch).RunAndReturnTrimmedOutput()
	if err != nil {
		git.logger.Warnf(output)
		return err
	}
	git.logger.Debugf(output)

	return nil
}

func (git *Git) GetCurrentBranch() (string, error) {
	output, err := git.command("rev-parse", "--abbrev-ref", "HEAD").RunAndReturnTrimmedOutput()
	if err != nil {
		git.logger.Warnf(output)
		return "", err
	}
	return strings.TrimSpace(output), nil
}
