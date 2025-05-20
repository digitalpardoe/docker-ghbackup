package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// CommandRunner defines an interface for running external commands.
type CommandRunner interface {
	Run(dir string, name string, args ...string) ([]byte, error)
	RunAndOutput(dir string, name string, args ...string) error
}

// DefaultCommandRunner is the default implementation of CommandRunner using os/exec.
type DefaultCommandRunner struct{}

func (dcr *DefaultCommandRunner) Run(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

func (dcr *DefaultCommandRunner) RunAndOutput(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GitHubClient defines an interface for interacting with the GitHub API.
// This helps in mocking the client for tests.
type GitHubClient interface {
	GetAuthenticatedUser(ctx context.Context) (*github.User, error)
	ListUserRepositories(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error)
}

// RealGitHubClient is a wrapper around the go-github client.
type RealGitHubClient struct {
	client *github.Client
}

func NewRealGitHubClient(token string) *RealGitHubClient {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &RealGitHubClient{client: github.NewClient(tc)}
}

func (rgc *RealGitHubClient) GetAuthenticatedUser(ctx context.Context) (*github.User, error) {
	user, _, err := rgc.client.Users.Get(ctx, "")
	return user, err
}

func (rgc *RealGitHubClient) ListUserRepositories(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error) {
	return rgc.client.Repositories.List(ctx, user, opts)
}

// App holds application dependencies and configuration.
type App struct {
	GithubToken  string
	BackupFolder string
	GhClient     GitHubClient
	CmdRunner    CommandRunner
	// Functions for filesystem operations, allowing them to be mocked
	Stat   func(name string) (os.FileInfo, error)
	MkdirAll func(path string, perm os.FileMode) error
	Getwd  func() (string, error)
	Chdir  func(dir string) error
}

// runApp contains the core logic of the application.
func (app *App) runApp(ctx context.Context) error {
	log.Println("Starting GitHub backup...")

	if app.GithubToken == "" {
		return fmt.Errorf("Error: GITHUB_SECRET environment variable is not set.")
	}

	if app.BackupFolder == "" {
		app.BackupFolder = "/ghbackup" // Default if not set by caller
	}

	if err := app.MkdirAll(app.BackupFolder, 0755); err != nil {
		return fmt.Errorf("Error creating backup folder %s: %v", app.BackupFolder, err)
	}

	if output, err := app.CmdRunner.Run("", "git", "config", "--global", "--add", "safe.directory", "*"); err != nil {
		return fmt.Errorf("Error setting global git config: %v\nOutput: %s", err, string(output))
	}

	user, err := app.GhClient.GetAuthenticatedUser(ctx)
	if err != nil {
		return fmt.Errorf("Error getting authenticated user: %v", err)
	}
	username := *user.Login

	log.Println("Fetching repositories...")
	opt := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allRepos []*github.Repository
	for {
		repos, resp, err := app.GhClient.ListUserRepositories(ctx, "", opt)
		if err != nil {
			return fmt.Errorf("Error listing repositories: %v", err)
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	log.Printf("Found %d repositories to backup.\n", len(allRepos))

	for _, repo := range allRepos {
		repoFullName := *repo.FullName
		// repoName := *repo.Name // Not used, can be removed if not needed elsewhere
		log.Printf("Backing up repository: %s\n", repoFullName)

		authenticatedCloneURL := fmt.Sprintf("https://%s:%s@github.com/%s.git", username, app.GithubToken, repoFullName)
		unauthenticatedCloneURL := fmt.Sprintf("https://github.com/%s.git", repoFullName)
		backupPath := filepath.Join(app.BackupFolder, repoFullName+".git")

		if _, err := app.Stat(backupPath); os.IsNotExist(err) {
			log.Printf("Backup for %s does not exist, cloning...\n", repoFullName)
			if err := app.CmdRunner.RunAndOutput("", "git", "clone", "--mirror", "--no-checkout", "--progress", authenticatedCloneURL, backupPath); err != nil {
				log.Printf("Error cloning repository %s: %v\n", repoFullName, err)
				continue // Continue with the next repository
			}

			originalWd, err := app.Getwd()
			if err != nil {
				log.Printf("Error getting current working directory for %s: %v\n", repoFullName, err)
				continue
			}
			if err := app.Chdir(backupPath); err != nil {
				log.Printf("Error changing directory to %s: %v\n", backupPath, err)
				continue
			}

			log.Printf("Fetching LFS objects for %s\n", repoFullName)
			if err := app.CmdRunner.RunAndOutput(backupPath, "git", "lfs", "fetch", "--all"); err != nil {
				log.Printf("Error fetching LFS objects for %s: %v\n", repoFullName, err)
				// Non-fatal, continue to set remote
			}

			log.Printf("Setting remote URL to unauthenticated for %s\n", repoFullName)
			if output, err := app.CmdRunner.Run(backupPath, "git", "remote", "set-url", "origin", unauthenticatedCloneURL); err != nil {
				log.Printf("Error setting remote URL to unauthenticated for %s: %v\nOutput: %s\n", repoFullName, err, string(output))
			}

			if err := app.Chdir(originalWd); err != nil {
				log.Printf("Error changing directory back to original for %s: %v\n", repoFullName, err)
			}

		} else if err == nil { // Backup exists
			log.Printf("Backup for %s exists, updating...\n", repoFullName)
			originalWd, err := app.Getwd()
			if err != nil {
				log.Printf("Error getting current working directory for update of %s: %v\n", repoFullName, err)
				continue
			}
			if err := app.Chdir(backupPath); err != nil {
				log.Printf("Error changing directory to %s for update: %v\n", backupPath, err)
				continue
			}

			log.Printf("Setting remote URL to authenticated for %s for update\n", repoFullName)
			if output, err := app.CmdRunner.Run(backupPath, "git", "remote", "set-url", "origin", authenticatedCloneURL); err != nil {
				log.Printf("Error setting remote URL to authenticated for %s: %v\nOutput: %s\n", repoFullName, err, string(output))
				if err := app.Chdir(originalWd); err != nil { // Try to change back even if set-url failed
					log.Printf("Error changing directory back to original for %s after auth set-url fail: %v\n", repoFullName, err)
				}
				continue
			}

			log.Printf("Updating remote for %s\n", repoFullName)
			if err := app.CmdRunner.RunAndOutput(backupPath, "git", "remote", "update"); err != nil {
				log.Printf("Error updating remote for %s: %v\n", repoFullName, err)
				// Non-fatal, continue to fetch LFS and set unauthenticated remote
			}

			log.Printf("Fetching LFS objects for %s\n", repoFullName)
			if err := app.CmdRunner.RunAndOutput(backupPath, "git", "lfs", "fetch", "--all"); err != nil {
				log.Printf("Error fetching LFS objects for %s: %v\n", repoFullName, err)
			}

			log.Printf("Setting remote URL to unauthenticated for %s\n", repoFullName)
			if output, err := app.CmdRunner.Run(backupPath, "git", "remote", "set-url", "origin", unauthenticatedCloneURL); err != nil {
				log.Printf("Error setting remote URL to unauthenticated for %s: %v\nOutput: %s\n", repoFullName, err, string(output))
			}

			if err := app.Chdir(originalWd); err != nil {
				log.Printf("Error changing directory back to original for %s after update: %v\n", repoFullName, err)
			}
		} else { // Some other error with os.Stat
			log.Printf("Error checking backup status for %s: %v\n", repoFullName, err)
			continue
		}
		log.Printf("Finished backing up repository: %s\n", repoFullName)
	}

	log.Println("GitHub backup completed.")
	return nil
}

func main() {
	githubToken := os.Getenv("GITHUB_SECRET")
	backupFolder := os.Getenv("BACKUP_FOLDER")

	app := &App{
		GithubToken:  githubToken,
		BackupFolder: backupFolder,
		GhClient:     NewRealGitHubClient(githubToken),
		CmdRunner:    &DefaultCommandRunner{},
		Stat:         os.Stat,
		MkdirAll:     os.MkdirAll,
		Getwd:        os.Getwd,
		Chdir:        os.Chdir,
	}

	if err := app.runApp(context.Background()); err != nil {
		log.Fatal(err) // log.Fatal will print the error and exit(1)
	}
}
