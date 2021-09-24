package operationplanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
	"github.com/thoas/go-funk"
)

type OperationPlanner struct {
	changelist []string
	workDir    string
	logger     log.Logger
}

type Operation int

const (
	OPERATION_RUN  Operation = iota
	OPERATION_SCAN Operation = iota
	OPERATION_DESTROY
)

type DirOperation struct {
	Dir       string
	Operation Operation
}

type OperationBatch []DirOperation

type OperationPlan struct {
	OperationBatches []OperationBatch
}

func New(changelist []string, workDir string) *OperationPlanner {
	return &OperationPlanner{
		changelist: changelist,
		workDir:    workDir,
		logger:     log.NewLogger(),
	}
}

func (p *OperationPlanner) getChangedDirectories() ([]string, error) {
	changedDirs := funk.Map(p.changelist, func(s string) string {
		return filepath.Dir(s)
	}).([]string)

	changedDirs = funk.UniqString(changedDirs)

	p.logger.Printf("Changed dirs: %s", strings.Join(changedDirs, ","))

	return changedDirs, nil
}

func (p *OperationPlanner) absolutePath(path string) string {
	return filepath.Join(p.workDir, path)
}

func (p *OperationPlanner) PlanOperation() (*OperationPlan, error) {
	p.logger.Printf("Planning %s based on %d changes", p.workDir, len(p.changelist))
	p.logger.Debugf("Initial changelist %s", strings.Join(p.changelist, ",\n"))

	changedDirs, err := p.getChangedDirectories()
	if err != nil {
		return nil, fmt.Errorf("getting changed directories %w", err)
	}

	currentLayer := funk.Map(changedDirs, func(s string) DirOperation {
		var op Operation
		if filepath.Ext(s) == ".hcl" {
			op = OPERATION_RUN
		} else {
			op = OPERATION_SCAN
		}
		return DirOperation{Dir: p.absolutePath(s), Operation: op} // TODO destroy on deleted dirs
	}).([]DirOperation)

	plan := OperationPlan{}
	plan.OperationBatches = []OperationBatch{currentLayer}

	for len(currentLayer) > 0 {
		nextLayer := []DirOperation{}

		p.logger.Infof("Walking path for layer %d", len(plan.OperationBatches))
		err := filepath.Walk(p.workDir, func(path string, info os.FileInfo, err error) error {
			if filepath.Ext(path) != ".hcl" && filepath.Ext(path) != ".tf" { //Only check TF or HCL files
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

			if funk.Contains(deps, func(s string) bool { // Is any of the dependencies include something from the current batch?
				return funk.Contains(currentLayer, func(op DirOperation) bool {
					return op.Dir == s
				})
			}) {
				p.logger.Debugf("Found match among dependencies of %s in current layer", path)
				var op DirOperation
				if filepath.Ext(path) == ".hcl" {
					op = DirOperation{Dir: filepath.Dir(path), Operation: OPERATION_RUN}
				} else {
					op = DirOperation{Dir: filepath.Dir(path), Operation: OPERATION_SCAN}
				}
				nextLayer = append(nextLayer, op)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking files: %w", err)
		}

		nextLayer = funk.Uniq(nextLayer).([]DirOperation)

		p.logger.Infof("Found %d new items", len(nextLayer))
		p.logger.Debugf("%s", strings.Join(funk.Map(nextLayer, func(op DirOperation) string {
			return op.Dir
		}).([]string), ", "))

		if len(nextLayer) > 0 {
			plan.OperationBatches = append(plan.OperationBatches, nextLayer)
		}
		currentLayer = nextLayer
	}

	return &plan, nil
}
