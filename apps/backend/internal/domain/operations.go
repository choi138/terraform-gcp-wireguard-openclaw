package domain

import "time"

type RetentionTargetReport struct {
	Target         string    `json:"target"`
	Action         string    `json:"action"`
	Cutoff         time.Time `json:"cutoff"`
	CandidateCount int       `json:"candidate_count"`
	AffectedCount  int       `json:"affected_count"`
}

type RetentionRunReport struct {
	DryRun      bool                    `json:"dry_run"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt time.Time               `json:"completed_at"`
	Targets     []RetentionTargetReport `json:"targets"`
}
