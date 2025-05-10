package agent

type SSHClient interface {
	Copy(content, filepath string) error
	DeleteFile(filepath string) error
	Close() error
}
