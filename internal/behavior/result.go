package behavior

type StepResult string

const (
	ResultUnknown StepResult = "unknown"
	ResultSuccess StepResult = "success"
	ResultFailure StepResult = "failure"
	ResultSkipped StepResult = "skipped"
)
