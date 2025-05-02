package scp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

// Client represents an SSH client that can perform SCP operations
type Client struct {
	client *ssh.Client
}

// Options holds the configuration options for creating an SCP client
type Options struct {
	Host           string
	Username       string
	PrivateKeyPath string // Path to the private key file
	Timeout        time.Duration
}

// NewClient creates a new SCP client with the given options
func NewClient(opts Options) (*Client, error) {
	var authMethods []ssh.AuthMethod

	// Add private key authentication
	if opts.PrivateKeyPath == "" {
		return nil, fmt.Errorf("private key path is required")
	}

	// Read the private key file
	privateKeyData, err := os.ReadFile(opts.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	authMethods = append(authMethods, ssh.PublicKeys(signer))

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Configure SSH client
	config := &ssh.ClientConfig{
		User:            opts.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // WARNING: Only for development/testing
		Timeout:         timeout,
	}

	// Connect to SSH server
	client, err := ssh.Dial("tcp", opts.Host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH server: %w", err)
	}

	return &Client{client: client}, nil
}

// CopyFile copies the given content to a remote file via SCP
func CopyFile(opts Options, content []byte, remotePath string) error {
	// Create a new SCP client
	scpClient, err := NewClient(opts)
	if err != nil {
		return err
	}
	defer scpClient.Close()

	// Use the client to copy the content
	return scpClient.CopyBytes(content, remotePath)
}

// Copy implements the agent.SCP interface
// It copies the given content to the remote file specified by filepath
func (c *Client) Copy(content, remotePath string) error {
	return c.CopyBytes([]byte(content), remotePath)
}

// CopyBytes copies the given byte content to the remote file specified by filepath
func (c *Client) CopyBytes(contentBytes []byte, remotePath string) error {
	// Create a new SSH session
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Set up pipes for stdin/stdout/stderr
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	// Ensure the remote directory exists
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		mkdirSession, err := c.client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create mkdir session: %w", err)
		}

		err = mkdirSession.Run(fmt.Sprintf("mkdir -p %s", remoteDir))
		mkdirSession.Close()
		if err != nil {
			return fmt.Errorf("failed to create remote directory: %w", err)
		}
	}

	contentLen := len(contentBytes)

	// Start the SCP command in sink mode (receiving files)
	cmd := fmt.Sprintf("scp -t %s", remotePath)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start SCP command: %w", err)
	}

	// SCP protocol: check for acknowledgment
	buffer := make([]byte, 1)
	if _, err = stdout.Read(buffer); err != nil {
		return fmt.Errorf("failed to read SCP acknowledgment: %w", err)
	}
	if buffer[0] != 0 {
		return fmt.Errorf("SCP acknowledgment error: %s", stderrBuf.String())
	}

	// SCP protocol: send file info
	fileMode := "0644" // Default permissions for files
	// No timestamp needed for SCP implementation
	if _, err = fmt.Fprintf(stdin, "C%s %d %s\n", fileMode, contentLen, filepath.Base(remotePath)); err != nil {
		return fmt.Errorf("failed to send file info: %w", err)
	}

	// SCP protocol: check for acknowledgment
	if _, err = stdout.Read(buffer); err != nil {
		return fmt.Errorf("failed to read SCP acknowledgment after file info: %w", err)
	}
	if buffer[0] != 0 {
		return fmt.Errorf("SCP acknowledgment error after file info: %s", stderrBuf.String())
	}

	// SCP protocol: send file content
	if _, err = stdin.Write(contentBytes); err != nil {
		return fmt.Errorf("failed to send file content: %w", err)
	}

	// SCP protocol: send null byte to indicate end of content
	if _, err = stdin.Write([]byte{0}); err != nil {
		return fmt.Errorf("failed to send end-of-file marker: %w", err)
	}

	// SCP protocol: check for acknowledgment
	if _, err = stdout.Read(buffer); err != nil {
		return fmt.Errorf("failed to read final SCP acknowledgment: %w", err)
	}
	if buffer[0] != 0 {
		return fmt.Errorf("final SCP acknowledgment error: %s", stderrBuf.String())
	}

	// Close stdin to signal we're done
	stdin.Close()

	// Wait for the command to complete
	if err := session.Wait(); err != nil {
		return fmt.Errorf("SCP command failed: %w: %s", err, stderrBuf.String())
	}

	return nil
}

// Close closes the underlying SSH client connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
