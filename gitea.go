package git

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"code.gitea.io/sdk/gitea"
	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
)

func NewGiteaAdapter() (*GiteaAdapter, error) {
	// Load configuration from the environment.
	env := &GitConfig{}
	if err := envconfig.Process("ORCHESTRATOR", env); err != nil {
		return nil, err
	}

	client, err := gitea.NewClient(
		env.BaseURL, gitea.SetToken(env.Token))
	if err != nil {
		return nil, err
	}

	return &GiteaAdapter{
		client: client,
		identity: &gitea.Identity{
			Name:  env.IdName,
			Email: env.IdMail,
		},
		env: env,
	}, nil
}

// GetFileContent retrieves raw content of a file
func (g *GiteaAdapter) GetFile(ctx context.Context, projectID uuid.UUID, path string) (*FileNode, error) {
	log.Printf("GetFileContent projectID:%s, path:%s", projectID, path)

	content, _, err := g.client.GetContents(g.env.Owner, projectID.String(), g.env.Branch, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file contents: %w", err)
	}

	decodedStr := content.Content
	// Gitea returns content usually base64 encoded in 'Content' field if it's a file
	if content.Encoding != nil && *content.Encoding == "base64" {
		decodedBytes, err := base64.StdEncoding.DecodeString(*content.Content)
		if err == nil {
			tmp := string(decodedBytes)
			decodedStr = &tmp
		}
	}

	return &FileNode{
		Name:    content.Name,
		Path:    content.Path,
		Type:    FileTypeFile,
		SHA:     content.SHA,
		Size:    content.Size,
		Content: decodedStr,
	}, nil
}

// ListFiles retrieves files. If path is empty, lists root.
func (g *GiteaAdapter) ListFiles(ctx context.Context, projectID uuid.UUID, path string) ([]FileNode, error) {
	log.Printf("[Git Log] ListFiles projectID:%s, path:%s", projectID, path)

	// GetContents returns a list of items if the path is a directory.
	// If path is empty "", it returns the root directory content.
	entries, _, err := g.client.ListContents(g.env.Owner, projectID.String(), g.env.Branch, path)
	if err != nil {
		return nil, fmt.Errorf("failed to list contents at path '%s': %w", path, err)
	}

	var files []FileNode
	for _, entry := range entries {
		var nodeType FileType
		switch entry.Type {
		case "file":
			nodeType = FileTypeFile
		case "dir":
			nodeType = FileTypeDir
		case "symlink":
			nodeType = FileTypeSymlink
		}

		files = append(files, FileNode{
			Name:     entry.Name,
			Path:     entry.Path,
			Type:     nodeType,
			Target:   entry.Target,
			SHA:      entry.SHA,
			Size:     entry.Size,
			Encoding: entry.Encoding,
		})
	}

	return files, nil
}

// CommitFile creates or updates a file
func (g *GiteaAdapter) CommitFile(ctx context.Context, projectID uuid.UUID, path, content, message string) error {
	log.Printf("[Git Log] CommitFile projectID:%s, path:%s, message:%s", projectID, path, message)

	b64Content := base64.StdEncoding.EncodeToString([]byte(content))

	// Check if file exists to decide between Create or Update
	if existing, _, err := g.client.GetContents(g.env.Owner, projectID.String(), g.env.Branch, path); err == nil {
		// File exists -> Update
		_, _, err = g.client.UpdateFile(g.env.Owner, projectID.String(), path, gitea.UpdateFileOptions{
			FileOptions: gitea.FileOptions{
				Message:    message,
				BranchName: g.env.Branch,
				Author:     *g.identity,
				Committer:  *g.identity,
			},
			Content: b64Content,
			SHA:     existing.SHA,
		})
		return err
	}

	// File does not exist -> Create
	_, _, err := g.client.CreateFile(g.env.Owner, projectID.String(), path, gitea.CreateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:    message,
			BranchName: g.env.Branch,
			Author:     *g.identity,
			Committer:  *g.identity,
		},
		Content: b64Content,
	})
	return err
}

// DeleteFile implementation (Basic)
func (g *GiteaAdapter) DeleteFile(ctx context.Context, projectID uuid.UUID, path, message string) error {
	log.Printf("[Git Log] DeleteFile projectID:%s, path:%s, message:%s", projectID, path, message)

	// Gitea requires the SHA of the file to delete it
	existing, _, err := g.client.GetContents(g.env.Owner, projectID.String(), g.env.Branch, path)
	if err != nil {
		return fmt.Errorf("file not found for deletion: %w", err)
	}

	_, err = g.client.DeleteFile(g.env.Owner, projectID.String(), path, gitea.DeleteFileOptions{
		FileOptions: gitea.FileOptions{
			Message:    message,
			BranchName: g.env.Branch,
		},
		SHA: existing.SHA,
	})
	return err
}

// CreateRepository creates a new private repository and returns its full name (owner/name)
func (g *GiteaAdapter) CreateRepository(ctx context.Context, projectID uuid.UUID) (string, error) {
	log.Printf("[Git Log] Creating repository: %s", projectID)

	opt := gitea.CreateRepoOption{
		Name:          projectID.String(),
		Description:   "Managed by GitAPI",
		Private:       true,
		AutoInit:      true, // Initializes with a default branch so it's immediately usable
		DefaultBranch: g.env.Branch,
	}

	repo, _, err := g.client.CreateRepo(opt)
	if err != nil {
		return "", fmt.Errorf("failed to create gitea repository: %w", err)
	}

	return repo.FullName, nil
}

// ScaffoldProjectFiles creates or updates multiple files
func (g *GiteaAdapter) ScaffoldProjectFiles(ctx context.Context, projectID uuid.UUID, files []FileNode) error {
	log.Printf("[Git] Starting Serial Scaffold for %s (%d files)", projectID, len(files))

	for i, file := range files {
		log.Printf("[%d/%d] Committing %s...", i+1, len(files), file.Path)
		msg := fmt.Sprintf("Scaffold path: %s", file.Path)
		err := g.CommitFile(ctx, projectID, file.Path, *file.Content, msg)
		if err != nil {
			log.Printf("[Git Err] Scaffold project: %s path:%s err: %s",
				projectID, file.Path, err.Error())
		}
	}

	log.Printf("[Git] Scaffold completed successfully for %s", projectID)
	return nil
}
