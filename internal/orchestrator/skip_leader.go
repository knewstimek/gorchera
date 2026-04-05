package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"gorchera/internal/domain"
)

const defaultMaxEvalRetries = 3

// runSkipLeaderLoop drives executor->evaluator retries without a leader LLM.
// On each iteration the orchestrator synthesizes a LeaderOutput from the job
// goal and injects cumulative evaluator failure context into task_text, so the
// executor can fix issues without regressing earlier corrections.
//
// Loop behaviour:
//   - evaluator passed  -> job done
//   - evaluator blocked -> job blocked immediately (dependency or external issue)
//   - evaluator failed  -> inject failure reason + history, retry up to MaxEvalRetries
//   - max_steps reached -> job blocked ("max_steps_exceeded")
//
// Important: evaluateCompletion calls applyEvaluatorJobState which mutates
// job.Status (failed/blocked). We inspect report.Status directly and reset
// job.Status to Running before retrying, mirroring what the leader loop does
// at the top of each iteration.
func (s *Service) runSkipLeaderLoop(ctx context.Context, job *domain.Job) (*domain.Job, error) {
	maxRetries := job.MaxEvalRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxEvalRetries
	}

	// failHistory accumulates evaluator failure reasons across retries.
	// Each entry is "Attempt N: <reason>". Capped at 3 to keep task_text compact.
	var failHistory []string

	for attempt := 0; job.CurrentStep < job.MaxSteps; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		task := buildSkipLeaderTask(job, attempt+1, failHistory)
		if err := s.runWorkerStep(ctx, job, task); err != nil {
			return nil, err
		}
		// runWorkerStep may set job.Status to blocked/failed on worker error.
		if job.Status == domain.JobStatusBlocked || job.Status == domain.JobStatusFailed {
			return job, nil
		}

		report, err := s.evaluateCompletion(ctx, job)
		if err != nil {
			return nil, err
		}
		// Do NOT check job.Status here: applyEvaluatorJobState already mutated it
		// based on report.Status. We decide retry vs. terminal from report.Status
		// directly, and reset job.Status before continuing when retrying.

		if report.Passed {
			job.Status = domain.JobStatusDone
			job.Summary = report.Reason
			s.clearJobRuntimeState(job)
			s.addEvent(job, "job_completed", report.Reason)
			s.touch(job)
			if err := s.state.SaveJob(ctx, job); err != nil {
				return nil, err
			}
			if err := s.handleChainCompletion(ctx, job); err != nil {
				return nil, err
			}
			return job, nil
		}

		// evaluator blocked: dependency or external issue -- surface immediately.
		// job.Status is already JobStatusBlocked from applyEvaluatorJobState.
		if report.Status == "blocked" {
			return job, nil
		}

		// evaluator failed: accumulate reason and retry if budget remains.
		failHistory = append(failHistory, fmt.Sprintf("Attempt %d: %s", attempt+1, report.Reason))
		if len(failHistory) > 3 {
			failHistory = failHistory[len(failHistory)-3:]
		}

		if attempt >= maxRetries-1 {
			// Budget exhausted. applyEvaluatorJobState already set job.Status=Failed
			// and job.FailureReason. Override reason to include retry count.
			job.FailureReason = fmt.Sprintf("evaluator failed after %d retries: %s", maxRetries, report.Reason)
			s.touch(job)
			if err := s.state.SaveJob(ctx, job); err != nil {
				return nil, err
			}
			return job, nil
		}

		// Reset job state for the next attempt, mirroring the leader loop's
		// status reset at the top of each iteration.
		job.Status = domain.JobStatusRunning
		job.FailureReason = ""
		s.addEvent(job, "skip_leader_retry", fmt.Sprintf("retry %d/%d: %s", attempt+1, maxRetries, report.Reason))
	}

	return s.blockJobWithEvent(ctx, job, "job_blocked", "max_steps_exceeded")
}

// buildSkipLeaderTask synthesizes a LeaderOutput from the job goal and
// accumulated evaluator failure history. No LLM call is made.
func buildSkipLeaderTask(job *domain.Job, attempt int, failHistory []string) domain.LeaderOutput {
	var b strings.Builder
	b.WriteString(job.Goal)

	if len(job.DoneCriteria) > 0 {
		b.WriteString("\n\nDone criteria:\n")
		for _, c := range job.DoneCriteria {
			b.WriteString("- ")
			b.WriteString(c)
			b.WriteString("\n")
		}
	}

	if len(failHistory) > 0 {
		b.WriteString("\nPrevious attempts were rejected -- fix the issues below WITHOUT regressing earlier fixes:\n")
		for _, entry := range failHistory {
			b.WriteString("  ")
			b.WriteString(entry)
			b.WriteString("\n")
		}
	}

	return domain.LeaderOutput{
		Action:   "run_worker",
		Target:   "B",
		TaskType: "implement",
		TaskText: b.String(),
		Reason:   fmt.Sprintf("skip_leader attempt %d", attempt),
	}
}
