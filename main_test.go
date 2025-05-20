package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v62/github"
)

// --- Mocks for main.GitHubClient ---

type mockGhClient struct {
	GetAuthenticatedUserFunc func(ctx context.Context) (*github.User, error)
	ListUserRepositoriesFunc func(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error)
}

func (m *mockGhClient) GetAuthenticatedUser(ctx context.Context) (*github.User, error) {
	if m.GetAuthenticatedUserFunc != nil {
		return m.GetAuthenticatedUserFunc(ctx)
	}
	login := "testuser"
	return &github.User{Login: &login}, nil
}

func (m *mockGhClient) ListUserRepositories(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
	if m.ListUserRepositoriesFunc != nil {
		return m.ListUserRepositoriesFunc(ctx, user, opts)
	}
	return []*github.Repository{}, &github.Response{NextPage: 0}, nil
}

// --- Mocks for main.CommandRunner ---

type mockCmdRunner struct {
	RunFunc          func(dir string, name string, args ...string) ([]byte, error)
	RunAndOutputFunc func(dir string, name string, args ...string) error
	executedCmds     []string   // To store executed commands for verification
	commandDetails   []struct { // To store more details about executed commands
		Dir  string
		Name string
		Args []string
	}
}

func newMockCmdRunner() *mockCmdRunner {
	return &mockCmdRunner{
		executedCmds: []string{},
		commandDetails: []struct {
			Dir  string
			Name string
			Args []string
		}{},
	}
}

func (mcr *mockCmdRunner) Run(dir string, name string, args ...string) ([]byte, error) {
	cmdString := fmt.Sprintf("dir: '%s', cmd: %s %s", dir, name, strings.Join(args, " "))
	mcr.executedCmds = append(mcr.executedCmds, cmdString)
	mcr.commandDetails = append(mcr.commandDetails, struct {
		Dir  string
		Name string
		Args []string
	}{Dir: dir, Name: name, Args: args})

	if mcr.RunFunc != nil {
		return mcr.RunFunc(dir, name, args...)
	}
	// Default behavior: success, no output
	return []byte{}, nil
}

func (mcr *mockCmdRunner) RunAndOutput(dir string, name string, args ...string) error {
	cmdString := fmt.Sprintf("dir: '%s', cmd: %s %s (interactive)", dir, name, strings.Join(args, " "))
	mcr.executedCmds = append(mcr.executedCmds, cmdString)
	mcr.commandDetails = append(mcr.commandDetails, struct {
		Dir  string
		Name string
		Args []string
	}{Dir: dir, Name: name, Args: args})

	if mcr.RunAndOutputFunc != nil {
		return mcr.RunAndOutputFunc(dir, name, args...)
	}
	// Default behavior: success
	return nil
}

func (mcr *mockCmdRunner) findCommand(name string, argsPrefix ...string) bool {
	for _, detail := range mcr.commandDetails {
		if detail.Name == name {
			match := true
			if len(argsPrefix) > 0 {
				if len(detail.Args) < len(argsPrefix) {
					match = false
				} else {
					for i, prefix := range argsPrefix {
						if detail.Args[i] != prefix {
							match = false
							break
						}
					}
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// --- Mocks for Filesystem Operations ---
type mockFileInfo struct {
	name  string
	isDir bool
}

func (mfi *mockFileInfo) Name() string { return mfi.name }
func (mfi *mockFileInfo) Size() int64  { return 0 }
func (mfi *mockFileInfo) Mode() os.FileMode {
	if mfi.isDir {
		return os.ModeDir
	}
	return 0
}
func (mfi *mockFileInfo) ModTime() time.Time { return time.Now() }
func (mfi *mockFileInfo) IsDir() bool        { return mfi.isDir }
func (mfi *mockFileInfo) Sys() interface{}   { return nil }

var mockFilesystem = make(map[string]*mockFileInfo)
var mockMkdirAllPaths []string
var mockCurrentDir = "/app" // Default mock current directory

func mockStat(name string) (os.FileInfo, error) {
	if fi, ok := mockFilesystem[name]; ok {
		return fi, nil
	}
	return nil, os.ErrNotExist
}

func mockMkdirAll(path string, perm os.FileMode) error {
	mockMkdirAllPaths = append(mockMkdirAllPaths, path)
	// Simulate creating the directory in our mock filesystem
	mockFilesystem[path] = &mockFileInfo{name: filepath.Base(path), isDir: true}
	return nil
}
func mockGetwd() (string, error) {
	return mockCurrentDir, nil
}

func mockChdir(dir string) error {
	// Check if dir exists in mock filesystem or is a subpath of an existing one
	exists := false
	for path := range mockFilesystem {
		if strings.HasPrefix(dir, path) && mockFilesystem[path].IsDir() {
			exists = true
			break
		}
	}
	// Also check if it's the backup folder itself, which might be created by MkdirAll
	for _, p := range mockMkdirAllPaths {
		if p == dir {
			exists = true
			break
		}
	}

	if !exists && dir != "/" && dir != "." && !strings.HasPrefix(dir, "/tmp/") { // Allow /tmp for t.TempDir()
		// A more sophisticated mock would check if 'dir' is a valid path based on mockFilesystem
		// For now, if it's not explicitly in mockFilesystem, assume it's an issue unless it's a root/temp.
		// This part is tricky because git clone creates the directory.
		// Let's assume for tests that target directories for Chdir either exist or are valid.
	}
	mockCurrentDir = dir
	return nil
}

func resetMocks() {
	mockFilesystem = make(map[string]*mockFileInfo)
	mockMkdirAllPaths = []string{}
	mockCurrentDir = "/app" // Reset to a sensible default
}

// --- Test Cases ---

func TestEnvVarParsing(t *testing.T) {
	// These tests are for the main() function's handling of env vars before calling runApp.
	// The App struct itself receives these as direct string values.
	t.Run("GITHUB_SECRET is required by main", func(t *testing.T) {
		// This is implicitly tested by runApp returning an error if GithubToken is empty.
		// A direct test of main() is harder due to log.Fatal.
		app := App{GithubToken: "", BackupFolder: "/some/folder"}
		err := app.runApp(context.Background())
		if err == nil || !strings.Contains(err.Error(), "GITHUB_SECRET environment variable is not set") {
			t.Errorf("Expected error about GITHUB_SECRET, got %v", err)
		}
	})

	t.Run("BACKUP_FOLDER defaults in App if empty", func(t *testing.T) {
		// Test the default logic within App struct/runApp
		app := App{
			GithubToken:  "token",
			BackupFolder: "", // Empty, should default
			GhClient:     &mockGhClient{},
			CmdRunner:    newMockCmdRunner(),
			Stat:         func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }, // Mock Stat
			MkdirAll:     func(path string, perm os.FileMode) error { return nil },              // Mock MkdirAll
			Getwd:        mockGetwd,
			Chdir:        mockChdir,
		}
		// Minimal setup to pass initial checks
		mockGh := app.GhClient.(*mockGhClient)
		mockGh.GetAuthenticatedUserFunc = func(ctx context.Context) (*github.User, error) {
			login := "user"
			return &github.User{Login: &login}, nil
		}
		mockCmd := app.CmdRunner.(*mockCmdRunner)
		mockCmd.RunFunc = func(dir, name string, args ...string) ([]byte, error) { return []byte{}, nil } // Default success for git config

		app.runApp(context.Background()) // Ignore error for this specific default check
		if app.BackupFolder != "/ghbackup" {
			t.Errorf("Expected BackupFolder to default to /ghbackup, got %s", app.BackupFolder)
		}
	})
}

func TestAppRun_ClonePath(t *testing.T) {
	resetMocks()
	ctx := context.Background()
	mockGh := &mockGhClient{}
	mockCmd := newMockCmdRunner()
	tempBackupDir := t.TempDir()

	app := App{
		GithubToken:  "test-token",
		BackupFolder: tempBackupDir,
		GhClient:     mockGh,
		CmdRunner:    mockCmd,
		Stat:         mockStat, // Use our mock Stat
		MkdirAll:     mockMkdirAll,
		Getwd:        mockGetwd,
		Chdir:        mockChdir,
	}

	// Mock GitHub API responses
	username := "testuser"
	mockGh.GetAuthenticatedUserFunc = func(ctx context.Context) (*github.User, error) {
		return &github.User{Login: &username}, nil
	}
	repo1FullName := "testuser/repo1"
	repo1Name := "repo1"
	mockGh.ListUserRepositoriesFunc = func(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
		return []*github.Repository{
			{Name: &repo1Name, FullName: &repo1FullName},
		}, &github.Response{NextPage: 0}, nil
	}

	// Mock command runner behavior (all commands succeed by default if not specified)

	// --- Act ---
	err := app.runApp(ctx)
	if err != nil {
		t.Fatalf("runApp failed: %v", err)
	}

	// --- Assert ---
	// Check MkdirAll was called for backup folder
	foundMkdir := false
	for _, p := range mockMkdirAllPaths {
		if p == tempBackupDir {
			foundMkdir = true
			break
		}
	}
	if !foundMkdir {
		t.Errorf("Expected MkdirAll to be called for %s", tempBackupDir)
	}

	// Check git commands
	expectedRepoPath := filepath.Join(tempBackupDir, repo1FullName+".git")
	authenticatedCloneURL := fmt.Sprintf("https://%s:%s@github.com/%s.git", username, app.GithubToken, repo1FullName)
	unauthenticatedCloneURL := fmt.Sprintf("https://github.com/%s.git", repo1FullName)

	expectedCommands := []struct {
		Dir  string
		Name string
		Args []string
	}{
		{"", "git", []string{"config", "--global", "--add", "safe.directory", "*"}},
		{"", "git", []string{"clone", "--mirror", "--no-checkout", "--progress", authenticatedCloneURL, expectedRepoPath}},
		{expectedRepoPath, "git", []string{"lfs", "fetch", "--all"}},
		{expectedRepoPath, "git", []string{"remote", "set-url", "origin", unauthenticatedCloneURL}},
	}

	if len(mockCmd.commandDetails) != len(expectedCommands) {
		t.Errorf("Expected %d git commands, got %d. Executed: %v", len(expectedCommands), len(mockCmd.commandDetails), mockCmd.executedCmds)
	}

	for i, expCmd := range expectedCommands {
		if i >= len(mockCmd.commandDetails) {
			t.Errorf("Missing expected command: %v", expCmd)
			continue
		}
		actualCmd := mockCmd.commandDetails[i]
		if actualCmd.Dir != expCmd.Dir || actualCmd.Name != expCmd.Name || !reflect.DeepEqual(actualCmd.Args, expCmd.Args) {
			t.Errorf("Command %d mismatch.\nExpected: Dir='%s', Name='%s', Args=%v\nActual:   Dir='%s', Name='%s', Args=%v",
				i, expCmd.Dir, expCmd.Name, expCmd.Args, actualCmd.Dir, actualCmd.Name, actualCmd.Args)
		}
	}
}

func TestAppRun_UpdatePath(t *testing.T) {
	resetMocks()
	ctx := context.Background()
	mockGh := &mockGhClient{}
	mockCmd := newMockCmdRunner()
	tempBackupDir := t.TempDir()

	repo1FullName := "testuser/repo-exists"
	expectedRepoPath := filepath.Join(tempBackupDir, repo1FullName+".git")

	// Simulate existing backup by adding it to our mock filesystem
	mockFilesystem[expectedRepoPath] = &mockFileInfo{name: repo1FullName + ".git", isDir: true}

	app := App{
		GithubToken:  "test-token",
		BackupFolder: tempBackupDir,
		GhClient:     mockGh,
		CmdRunner:    mockCmd,
		Stat:         mockStat, // Use our mock Stat
		MkdirAll:     mockMkdirAll,
		Getwd:        mockGetwd,
		Chdir:        mockChdir,
	}

	username := "testuser"
	mockGh.GetAuthenticatedUserFunc = func(ctx context.Context) (*github.User, error) {
		return &github.User{Login: &username}, nil
	}
	repo1Name := "repo-exists"
	mockGh.ListUserRepositoriesFunc = func(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
		return []*github.Repository{
			{Name: &repo1Name, FullName: &repo1FullName},
		}, &github.Response{NextPage: 0}, nil
	}
	// Mock command runner behavior (all commands succeed by default)

	// --- Act ---
	err := app.runApp(ctx)
	if err != nil {
		t.Fatalf("runApp failed for update path: %v", err)
	}

	// --- Assert ---
	authenticatedCloneURL := fmt.Sprintf("https://%s:%s@github.com/%s.git", username, app.GithubToken, repo1FullName)
	unauthenticatedCloneURL := fmt.Sprintf("https://github.com/%s.git", repo1FullName)

	expectedCommands := []struct {
		Dir  string
		Name string
		Args []string
	}{
		{"", "git", []string{"config", "--global", "--add", "safe.directory", "*"}},
		{expectedRepoPath, "git", []string{"remote", "set-url", "origin", authenticatedCloneURL}},
		{expectedRepoPath, "git", []string{"remote", "update"}},
		{expectedRepoPath, "git", []string{"lfs", "fetch", "--all"}},
		{expectedRepoPath, "git", []string{"remote", "set-url", "origin", unauthenticatedCloneURL}},
	}
	if len(mockCmd.commandDetails) != len(expectedCommands) {
		t.Errorf("Expected %d git commands for update, got %d. Executed: %v", len(expectedCommands), len(mockCmd.commandDetails), mockCmd.executedCmds)
	}
	for i, expCmd := range expectedCommands {
		if i >= len(mockCmd.commandDetails) {
			t.Errorf("Missing expected command (update path): %v", expCmd)
			continue
		}
		actualCmd := mockCmd.commandDetails[i]
		if actualCmd.Dir != expCmd.Dir || actualCmd.Name != expCmd.Name || !reflect.DeepEqual(actualCmd.Args, expCmd.Args) {
			t.Errorf("Command %d mismatch (update path).\nExpected: Dir='%s', Name='%s', Args=%v\nActual:   Dir='%s', Name='%s', Args=%v",
				i, expCmd.Dir, expCmd.Name, expCmd.Args, actualCmd.Dir, actualCmd.Name, actualCmd.Args)
		}
	}
}

func TestAppRun_GitHubUserError(t *testing.T) {
	resetMocks()
	ctx := context.Background()
	mockGh := &mockGhClient{}
	mockCmd := newMockCmdRunner()

	app := App{
		GithubToken:  "test-token",
		BackupFolder: t.TempDir(),
		GhClient:     mockGh,
		CmdRunner:    mockCmd,
		Stat:         mockStat,
		MkdirAll:     mockMkdirAll,
		Getwd:        mockGetwd,
		Chdir:        mockChdir,
	}

	expectedError := "failed to get user"
	mockGh.GetAuthenticatedUserFunc = func(ctx context.Context) (*github.User, error) {
		return nil, errors.New(expectedError)
	}
	// Mock git config to succeed
	mockCmd.RunFunc = func(dir, name string, args ...string) ([]byte, error) { return []byte{}, nil }

	err := app.runApp(ctx)
	if err == nil {
		t.Fatalf("runApp should have failed due to GitHub user error")
	}
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error message to contain '%s', got '%v'", expectedError, err)
	}
}

func TestAppRun_GitHubListReposError(t *testing.T) {
	resetMocks()
	ctx := context.Background()
	mockGh := &mockGhClient{}
	mockCmd := newMockCmdRunner()

	app := App{
		GithubToken:  "test-token",
		BackupFolder: t.TempDir(),
		GhClient:     mockGh,
		CmdRunner:    mockCmd,
		Stat:         mockStat,
		MkdirAll:     mockMkdirAll,
		Getwd:        mockGetwd,
		Chdir:        mockChdir,
	}

	username := "testuser"
	mockGh.GetAuthenticatedUserFunc = func(ctx context.Context) (*github.User, error) {
		return &github.User{Login: &username}, nil
	}
	expectedError := "failed to list repos"
	mockGh.ListUserRepositoriesFunc = func(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
		return nil, nil, errors.New(expectedError)
	}
	mockCmd.RunFunc = func(dir, name string, args ...string) ([]byte, error) { return []byte{}, nil }

	err := app.runApp(ctx)
	if err == nil {
		t.Fatalf("runApp should have failed due to GitHub list repos error")
	}
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error message to contain '%s', got '%v'", expectedError, err)
	}
}

func TestAppRun_GitConfigError(t *testing.T) {
	resetMocks()
	ctx := context.Background()
	mockGh := &mockGhClient{}
	mockCmd := newMockCmdRunner()

	app := App{
		GithubToken:  "test-token",
		BackupFolder: t.TempDir(),
		GhClient:     mockGh,
		CmdRunner:    mockCmd,
		Stat:         mockStat,
		MkdirAll:     mockMkdirAll,
		Getwd:        mockGetwd,
		Chdir:        mockChdir,
	}

	expectedError := "git config failed"
	mockCmd.RunFunc = func(dir, name string, args ...string) ([]byte, error) {
		if name == "git" && args[0] == "config" {
			return nil, errors.New(expectedError)
		}
		return []byte{}, nil
	}

	err := app.runApp(ctx)
	if err == nil {
		t.Fatalf("runApp should have failed due to git config error")
	}
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error message to contain '%s', got '%v'", expectedError, err)
	}
}

func TestAppRun_CloneErrorSkipsRepo(t *testing.T) {
	resetMocks()
	ctx := context.Background()
	mockGh := &mockGhClient{}
	mockCmd := newMockCmdRunner()
	tempBackupDir := t.TempDir()

	app := App{
		GithubToken:  "test-token",
		BackupFolder: tempBackupDir,
		GhClient:     mockGh,
		CmdRunner:    mockCmd,
		Stat:         mockStat,
		MkdirAll:     mockMkdirAll,
		Getwd:        mockGetwd,
		Chdir:        mockChdir,
	}

	username := "testuser"
	mockGh.GetAuthenticatedUserFunc = func(ctx context.Context) (*github.User, error) {
		return &github.User{Login: &username}, nil
	}
	repo1FullName := "testuser/repo1-clone-fails"
	repo1Name := "repo1-clone-fails"
	repo2FullName := "testuser/repo2-should-succeed"
	repo2Name := "repo2-should-succeed"

	mockGh.ListUserRepositoriesFunc = func(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
		return []*github.Repository{
			{Name: &repo1Name, FullName: &repo1FullName},
			{Name: &repo2Name, FullName: &repo2FullName},
		}, &github.Response{NextPage: 0}, nil
	}

	cloneError := errors.New("git clone intentional error")
	mockCmd.RunAndOutputFunc = func(dir string, name string, args ...string) error {
		if name == "git" && args[0] == "clone" && strings.Contains(args[3], repo1FullName) {
			return cloneError
		}
		return nil // Success for other commands (like LFS for repo2)
	}
	// git config and other non-RunAndOutput commands succeed
	mockCmd.RunFunc = func(dir, name string, args ...string) ([]byte, error) { return []byte{}, nil }

	// --- Act ---
	err := app.runApp(ctx) // This error will be nil if any repo succeeds and errors are logged.
	if err != nil {
		t.Fatalf("runApp returned an unexpected error: %v. Expected errors to be logged and skipped.", err)
	}

	// --- Assert ---
	// Check that repo2 was attempted (e.g. its clone command was issued)
	// The mockCmdRunner.RunAndOutputFunc will only be called for clone, remote update, lfs
	// We expect clone for repo1 (fails), then clone for repo2 (succeeds in mock)
	// Then LFS for repo2, then remote set-url for repo2.

	// Check that clone for repo2 was attempted and "succeeded" (mock success)
	repo2Path := filepath.Join(tempBackupDir, repo2FullName+".git")
	authenticatedCloneURLRepo2 := fmt.Sprintf("https://%s:%s@github.com/%s.git", username, app.GithubToken, repo2FullName)

	foundCloneRepo1 := false
	foundCloneRepo2 := false

	for _, detail := range mockCmd.commandDetails {
		if detail.Name == "git" && detail.Args[0] == "clone" {
			if strings.Contains(detail.Args[3], repo1FullName) {
				foundCloneRepo1 = true
			}
			if strings.Contains(detail.Args[3], repo2FullName) {
				foundCloneRepo2 = true
			}
		}
	}

	if !foundCloneRepo1 {
		t.Error("Expected clone attempt for repo1 (which fails)")
	}
	if !foundCloneRepo2 {
		t.Error("Expected clone attempt for repo2")
	}

	// Check LFS fetch for repo2 was attempted
	if !mockCmd.findCommand("git", "lfs", "fetch", "--all") {
		// This check is a bit broad, better to check with dir
		foundLFSForRepo2 := false
		for _, detail := range mockCmd.commandDetails {
			if detail.Dir == repo2Path && detail.Name == "git" && detail.Args[0] == "lfs" && detail.Args[1] == "fetch" {
				foundLFSForRepo2 = true
				break
			}
		}
		if !foundLFSForRepo2 {
			t.Errorf("Expected 'git lfs fetch --all' for repo2 in dir %s", repo2Path)
		}
	}
}

// Minimal main for TestMain to run.
func TestMain(m *testing.M) {
	// No specific setup needed for TestMain itself as tests manage their own mocks.
	// The main.main() is not directly called by tests.
	os.Exit(m.Run())
}

// Note: More error cases could be tested:
// - LFS fetch errors (should be non-fatal for the specific repo)
// - `git remote set-url` errors (both for auth and unauth)
// - `os.Getwd`, `os.Chdir` errors (should skip the repo or handle gracefully)
// - `os.MkdirAll` failure for the main backup folder (should be fatal for runApp)
// - Pagination in ListUserRepositories
