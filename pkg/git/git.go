package git

import (
	"fmt"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/retry"
	"strings"
	"time"
)

type Git struct {
	workDir string
	gitURL  string
}

func New(gitURL string, workDir string) *Git {
	return &Git{
		workDir: workDir,
		gitURL:  gitURL,
	}
}

func (git *Git) command(args ...string) command.Command {
	factory := command.NewFactory(env.NewRepository())
	opts := &command.Opts{
		Dir: git.workDir,
		Env: []string{"GIT_ASKPASS=echo"},
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

	diffOutput, err := git.command("diff", fmt.Sprintf("..%s", baseBranch), "--name-only").RunAndReturnTrimmedOutput()
	if err != nil {
		return []string{}, fmt.Errorf("failed to diff git, err: %w", err)
	}

	diffArr := strings.Split(diffOutput, "\n")

	return diffArr, nil
}
