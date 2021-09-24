package operationplanner

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

type OperationBatch struct {
	Operations      []DirOperation
	RunOnBaseBranch bool
}

type OperationPlan struct {
	OperationBatches []OperationBatch
	Command          string
	CommonRoot       string
}
