package agent

import (
	"fmt"
	"sync"
)

// MockSSHClient implements agent.SCP interface for testing
type MockSSHClient struct {
	// Map of filepaths to content
	filePaths map[string]string
	mu        sync.RWMutex
}

// NewMockSSHClient creates a new in-memory stub SCP client
func NewMockSSHClient() *MockSSHClient {
	return &MockSSHClient{
		filePaths: make(map[string]string),
	}
}

// Copy implements the agent.SCP interface
// It copies the given content to the specified filepath (in-memory)
func (s *MockSSHClient) Copy(content, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store the file content
	s.filePaths[filePath] = content
	return nil
}

// DeleteFile simulates deleting a file
func (s *MockSSHClient) DeleteFile(filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the file exists
	if _, exists := s.filePaths[filePath]; !exists {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Delete the file
	delete(s.filePaths, filePath)
	return nil
}

// FileExists checks if a file exists
func (s *MockSSHClient) FileExists(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.filePaths[filePath]
	return exists
}

// Reset clears all files and operations
func (s *MockSSHClient) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.filePaths = make(map[string]string)
}
