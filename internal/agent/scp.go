package agent

type SCP interface {
	Copy(content, filepath string) error
}
