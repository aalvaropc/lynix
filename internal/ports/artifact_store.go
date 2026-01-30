package ports

import "github.com/aalvaropc/lynix/internal/domain"

// ArtifactStore persists run artifacts for reproducibility.
type ArtifactStore interface {
	SaveRun(run domain.RunArtifact) (id string, err error)
	// ListRuns can be added later (MVP: optional).
}
