package agent

import (
	"fmt"
	"sync"
)

// MockSSH implements agent.SCP interface for testing
type MockSSH struct {
	// Map of filepaths to content
	filePaths map[string]string
	mu        sync.RWMutex
}

// NewMockSSHClient creates a new in-memory stub SCP client
func NewMockSSHClient() *MockSSH {
	return &MockSSH{
		filePaths: make(map[string]string),
	}
}

// Copy implements the agent.SCP interface
// It copies the given content to the specified filepath (in-memory)
func (s *MockSSH) Copy(content, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store the file content
	s.filePaths[filePath] = content
	return nil
}

// DeleteFile simulates deleting a file
func (s *MockSSH) DeleteFile(filePath string) error {
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
func (s *MockSSH) FileExists(filePath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.filePaths[filePath]
	return exists
}

// Reset clears all files and operations
func (s *MockSSH) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.filePaths = make(map[string]string)
}
