package operationplanner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/thoas/go-funk"
)

type OperationPlanner struct {
	changelist []string
	workDir    string
	command    string
	logger     log.Logger
}

type Operation int

const (
	OperationRun Operation = iota
	OperationScan
	OperationDestroy
)

type DirOperation struct {
	Dir       string
	Operation Operation
}

type OperationBatch []DirOperation

type OperationPlan struct {
	OperationBatches []OperationBatch
	Command          string
	CommonRoot       string
}

func New(changelist []string, workDir string, command string) *OperationPlanner {
	return &OperationPlanner{
		changelist: changelist,
		workDir:    workDir,
		logger:     log.NewLogger(),
		command:    command,
	}
}

func (p *OperationPlan) getBatchSummary(b *OperationBatch) string {
	return strings.Join(funk.Map(*b, func(op DirOperation) string {
		destroyWarning := ""
		if op.Operation == OperationDestroy {
			destroyWarning = " [!!! DESTROY !!!] "
		}
		return fmt.Sprintf("- %s%s", destroyWarning, strings.TrimPrefix(op.Dir, p.CommonRoot))
	}).([]string), "\n")
}

func (p *OperationPlan) GetSummary() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("\nOperation plan for command \"%s\" includes %d batches.\n\n", p.Command, len(p.OperationBatches)))
	for i, b := range p.OperationBatches {
		builder.WriteString(fmt.Sprintf("\n> Batch #%d:\n", i))
		builder.WriteString(p.getBatchSummary(&b))
		builder.WriteString("\n")
	}
	return builder.String()
}

func (p *OperationPlanner) getChangedDirectories() ([]string, error) {
	changedDirs := funk.Map(p.changelist, func(s string) string {
		return filepath.Dir(s)
	}).([]string)

	changedDirs = funk.UniqString(changedDirs)

	p.logger.Debugf("Changed dirs: %s", strings.Join(changedDirs, ","))

	return changedDirs, nil
}

func (p *OperationPlanner) absolutePath(path string) string {
	return filepath.Join(p.workDir, path)
}

func getCommonRoot(plan *OperationPlan, path string) string {
	if plan.CommonRoot == "" {
		return filepath.Dir(path) + string(filepath.Separator)
	}

	currentElems := strings.Split(plan.CommonRoot, string(filepath.Separator))
	if len(currentElems) < 2 {
		return plan.CommonRoot
	}
	pathElems := strings.Split(path, string(filepath.Separator))
	commonRootLength := 0
	for i, elem := range currentElems {
		if i >= len(pathElems) || elem != pathElems[i] {
			break
		}
		commonRootLength++
	}
	if commonRootLength > 1 {
		if pathElems[commonRootLength-1] != "" {
			return strings.Join(pathElems[:commonRootLength], string(filepath.Separator)) + string(filepath.Separator)
		} else {
			return strings.Join(pathElems[:commonRootLength], string(filepath.Separator))
		}
	}
	return string(filepath.Separator)
}

func filterScanOperations(plan *OperationPlan) {
	plan.OperationBatches = funk.Map(plan.OperationBatches, func(b OperationBatch) OperationBatch {
		return funk.Filter(b, func(op DirOperation) bool {
			return op.Operation != OperationScan
		}).([]DirOperation)
	}).([]OperationBatch)

	plan.OperationBatches = funk.Filter(plan.OperationBatches, func(b OperationBatch) bool {
		return len(b) > 0
	}).([]OperationBatch)
}

func isDirRunnable(dir string) (bool, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return funk.Contains(files, func(f os.FileInfo) bool {
		return filepath.Ext(f.Name()) == ".hcl"
	}), nil
}

func (p *OperationPlanner) getInitialBatch() (OperationBatch, error) {
	changedDirs, err := p.getChangedDirectories()
	if err != nil {
		return nil, fmt.Errorf("getting changed directories %w", err)
	}

	return funk.Map(changedDirs, func(dir string) DirOperation {
		var op Operation
		exists, err := pathutil.IsPathExists(dir)
		if err != nil {
			panic(err) // TODO rescue
		}
		runnable := false
		if exists {
			runnable, err = isDirRunnable(dir)
			if err != nil {
				panic(err) // TODO rescue
			}
		}
		if !exists {
			op = OperationDestroy
		} else if runnable {
			op = OperationRun
		} else {
			op = OperationScan
		}

		return DirOperation{Dir: p.absolutePath(dir), Operation: op} // TODO destroy on deleted dirs
	}).([]DirOperation), nil
}

func (p *OperationPlanner) PlanOperation() (*OperationPlan, error) {
	p.logger.Infof("Planning %s based on %d changes", p.workDir, len(p.changelist))
	p.logger.Debugf("Initial changelist: %s", strings.Join(p.changelist, ",\n"))

	var err error
	initialBatch, err := p.getInitialBatch()
	if err != nil {
		return nil, fmt.Errorf("getting initial batch: %w", err)
	}

	destroyBatch := funk.Filter(initialBatch, func(op DirOperation) bool {
		return op.Operation == OperationDestroy
	}).([]DirOperation)
	currentBatch := funk.Map(initialBatch, func(op DirOperation) DirOperation {
		if op.Operation == OperationDestroy {
			// Include destroyed modules in scanning
			return DirOperation{Dir: op.Dir, Operation: OperationScan}
		} else {
			return op
		}
	}).([]DirOperation)

	plan := OperationPlan{
		Command: p.command,
	}
	plan.OperationBatches = []OperationBatch{currentBatch}

	for len(currentBatch) > 0 {
		var nextBatch []DirOperation

		p.logger.Infof("Walking path for layer %d", len(plan.OperationBatches))
		err := filepath.Walk(p.workDir, func(path string, info os.FileInfo, err error) error {
			if filepath.Ext(path) != ".hcl" && filepath.Ext(path) != ".tf" { // Only check TF or HCL files
				return nil
			}
			if funk.Contains(strings.Split(path, string(filepath.Separator)), func(p string) bool { // Skip hidden directories, they are usually caches
				return strings.HasPrefix(p, ".")
			}) {
				return nil
			}

			deps, err := getDependencies(path)
			if err != nil {
				return fmt.Errorf("getting dependencies of %s: %w", path, err)
			}

			// Is any of the dependencies include something from the current batch?
			if funk.Contains(deps, func(s string) bool {
				return funk.Contains(currentBatch, func(op DirOperation) bool {
					return op.Dir == s
				})
			}) {
				p.logger.Debugf("Found match among dependencies of %s in current layer", path)
				var op DirOperation
				if filepath.Ext(path) == ".hcl" {
					op = DirOperation{Dir: filepath.Dir(path), Operation: OperationRun}
				} else {
					op = DirOperation{Dir: filepath.Dir(path), Operation: OperationScan}
				}
				plan.CommonRoot = getCommonRoot(&plan, path)
				nextBatch = append(nextBatch, op)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking files: %w", err)
		}

		nextBatch = funk.Uniq(nextBatch).([]DirOperation)

		p.logger.Infof("Found %d new items", len(nextBatch))
		p.logger.Debugf("%s", strings.Join(funk.Map(nextBatch, func(op DirOperation) string {
			return strings.TrimPrefix(op.Dir, plan.CommonRoot)
		}).([]string), ", "))

		if len(nextBatch) > 0 {
			plan.OperationBatches = append(plan.OperationBatches, nextBatch)
		}
		currentBatch = nextBatch
	}

	// No need to scan anymore, keep only dirs which will be executed
	filterScanOperations(&plan)
	// Prepend destroyables to catch errors with dependent modules
	plan.OperationBatches = append([]OperationBatch{destroyBatch}, plan.OperationBatches...)

	return &plan, nil
}
