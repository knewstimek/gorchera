package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorechera/internal/domain"
	"gorechera/internal/orchestrator"
	runtimeexec "gorechera/internal/runtime"
)

type RuntimeProcessListView struct {
	Processes []runtimeexec.ProcessHandle `json:"processes"`
	Note      string                      `json:"note,omitempty"`
}

type RuntimeProcessHandleView = runtimeexec.ProcessHandle

type RuntimeHarnessView struct {
	JobID           string                      `json:"job_id"`
	Goal            string                      `json:"goal"`
	Status          domain.JobStatus            `json:"status"`
	Provider        domain.ProviderName         `json:"provider"`
	WorkspaceDir    string                      `json:"workspace_dir,omitempty"`
	StepCount       int                         `json:"step_count"`
	EventCount      int                         `json:"event_count"`
	PendingApproval bool                        `json:"pending_approval"`
	ProcessCount    int                         `json:"process_count"`
	Processes       []runtimeexec.ProcessHandle `json:"processes,omitempty"`
	Available       bool                        `json:"available"`
	Note            string                      `json:"note,omitempty"`
}

func BuildRuntimeHarnessView(job *domain.Job, processes []runtimeexec.ProcessHandle) RuntimeHarnessView {
	return RuntimeHarnessView{
		JobID:           job.ID,
		Goal:            job.Goal,
		Status:          job.Status,
		Provider:        job.Provider,
		WorkspaceDir:    job.WorkspaceDir,
		StepCount:       len(job.Steps),
		EventCount:      len(job.Events),
		PendingApproval: job.PendingApproval != nil,
		ProcessCount:    len(processes),
		Processes:       copyProcessHandles(processes),
		Available:       true,
		Note:            "job harness view is assembled from persisted job state and tracked runtime processes",
	}
}

func (s *Server) handleHarness(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/harness")
	if path == "" || path == "/" {
		http.NotFound(w, r)
		return
	}

	path = strings.TrimPrefix(path, "/")

	if path == "processes" {
		switch r.Method {
		case http.MethodGet:
			processes, err := s.orchestrator.ListHarnessProcesses(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, RuntimeProcessListView{
				Processes: copyProcessHandles(processes),
				Note:      "runtime processes are managed by the orchestrator service",
			})
		case http.MethodPost:
			var req StartHarnessProcessRequest
			if err := decodeJSONBody(r, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			handle, err := s.orchestrator.StartHarnessProcess(r.Context(), buildHarnessStartRequest(req))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, handle)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if strings.HasPrefix(path, "processes/") {
		rest := strings.TrimPrefix(path, "processes/")
		if strings.HasSuffix(rest, "/stop") {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			pidPart := strings.TrimSuffix(rest, "/stop")
			pidPart = strings.TrimSuffix(pidPart, "/")
			pid, err := strconv.Atoi(pidPart)
			if err != nil || pid <= 0 {
				http.Error(w, "invalid pid", http.StatusBadRequest)
				return
			}
			handle, err := s.orchestrator.StopHarnessProcess(r.Context(), pid)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, handle)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		pid, err := strconv.Atoi(rest)
		if err != nil || pid <= 0 {
			http.Error(w, "invalid pid", http.StatusBadRequest)
			return
		}
		handle, err := s.orchestrator.GetHarnessProcess(r.Context(), pid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, handle)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleJobHarness(w http.ResponseWriter, r *http.Request, jobID, path string) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		job, err := s.orchestrator.Get(r.Context(), jobID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		processes, err := s.orchestrator.ListJobHarnessProcesses(r.Context(), jobID)
		if err != nil {
			writeHarnessError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, BuildRuntimeHarnessView(job, processes))
		return
	}

	if path == "processes" {
		switch r.Method {
		case http.MethodGet:
			processes, err := s.orchestrator.ListJobHarnessProcesses(r.Context(), jobID)
			if err != nil {
				writeHarnessError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, RuntimeProcessListView{
				Processes: copyProcessHandles(processes),
				Note:      "job-scoped runtime processes are owned by the requested job",
			})
		case http.MethodPost:
			var req StartHarnessProcessRequest
			if err := decodeJSONBody(r, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			handle, err := s.orchestrator.StartJobHarnessProcess(r.Context(), jobID, buildHarnessStartRequest(req))
			if err != nil {
				writeHarnessError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, handle)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if strings.HasPrefix(path, "processes/") {
		rest := strings.TrimPrefix(path, "processes/")
		if strings.HasSuffix(rest, "/stop") {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			pidPart := strings.TrimSuffix(rest, "/stop")
			pidPart = strings.TrimSuffix(pidPart, "/")
			pid, err := strconv.Atoi(pidPart)
			if err != nil || pid <= 0 {
				http.Error(w, "invalid pid", http.StatusBadRequest)
				return
			}
			handle, err := s.orchestrator.StopJobHarnessProcess(r.Context(), jobID, pid)
			if err != nil {
				writeHarnessError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, handle)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		pid, err := strconv.Atoi(rest)
		if err != nil || pid <= 0 {
			http.Error(w, "invalid pid", http.StatusBadRequest)
			return
		}
		handle, err := s.orchestrator.GetJobHarnessProcess(r.Context(), jobID, pid)
		if err != nil {
			writeHarnessError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, handle)
		return
	}

	http.NotFound(w, r)
}

func buildHarnessStartRequest(req StartHarnessProcessRequest) runtimeexec.StartRequest {
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	return runtimeexec.StartRequest{
		Request: runtimeexec.Request{
			Category:       req.Category,
			Command:        req.Command,
			Args:           append([]string(nil), req.Args...),
			Dir:            req.Dir,
			Env:            append([]string(nil), req.Env...),
			Timeout:        timeout,
			MaxOutputBytes: req.MaxOutputBytes,
		},
		Name:   req.Name,
		LogDir: req.LogDir,
		Port:   req.Port,
	}
}

func copyProcessHandles(processes []runtimeexec.ProcessHandle) []runtimeexec.ProcessHandle {
	if len(processes) == 0 {
		return nil
	}
	out := make([]runtimeexec.ProcessHandle, len(processes))
	copy(out, processes)
	return out
}

func writeHarnessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, runtimeexec.ErrProcessNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, runtimeexec.ErrNotAllowed):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, orchestrator.ErrHarnessOwnershipMismatch):
		http.Error(w, err.Error(), http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
