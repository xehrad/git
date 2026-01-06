package git

import (
	"code.gitea.io/sdk/gitea"
)

const (
	FileTypeFile    FileType = "file"
	FileTypeDir     FileType = "dir"
	FileTypeSymlink FileType = "symlink"
)

type (
	// FileType indicates if it is a file or directory
	FileType string

	GiteaAdapter struct {
		client   *gitea.Client
		identity *gitea.Identity
		env      *GitConfig
	}

	// FileNode represents a file or directory in the project
	FileNode struct {
		Name     string     `json:"name"`
		Path     string     `json:"path"`
		Type     FileType   `json:"type"`
		Target   *string    `json:"target,omitempty"` // `target` is populated when `type` is `symlink`, otherwise null
		SHA      string     `json:"sha"`
		Size     int64      `json:"size"`
		Content  *string    `json:"content,omitempty"`  // Content is empty for directories or list operations
		Children []FileNode `json:"children,omitempty"` // Children is populated for directories when listing recursively
	}

	// GitConfig holds Gitea connection settings
	GitConfig struct {
		BaseURL           string `envconfig:"ORCHESTRATOR_GIT_BASE_URL" required:"true"` // e.g., "http://gitea.default.svc.cluster.local:3000"
		Token             string `envconfig:"ORCHESTRATOR_GIT_TOKEN"    required:"true"` // Personal Access Token for Gitea
		IdName            string `envconfig:"ORCHESTRATOR_GIT_ID_NAME"      default:"ZamineBazi Orchestrator"`
		IdMail            string `envconfig:"ORCHESTRATOR_GIT_ID_EMAIL"     default:"bot@zaminebazi.com"`
		Owner             string `envconfig:"ORCHESTRATOR_GIT_OWNER_NAME"   default:"zaminebazi"`
		Branch            string `envconfig:"ORCHESTRATOR_GIT_BRANCH_NAME"  default:"main"`
		CreateRepoPrivate bool   `envconfig:"ORCHESTRATOR_GIT_REPO_PRIVATE" default:"false"`
		CreateRepoInit    bool   `envconfig:"ORCHESTRATOR_GIT_REPO_INIT"    default:"true"`
	}
)
