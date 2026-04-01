package policy

import (
	"fmt"
	"strings"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionBlock Decision = "block"
)

type ActionType string

const (
	ActionReadFile       ActionType = "read_file"
	ActionWriteFile      ActionType = "write_file"
	ActionSearch         ActionType = "search"
	ActionBuild          ActionType = "build"
	ActionTest           ActionType = "test"
	ActionLint           ActionType = "lint"
	ActionDiff           ActionType = "diff"
	ActionDelete         ActionType = "delete"
	ActionDeploy         ActionType = "deploy"
	ActionGitPush        ActionType = "git_push"
	ActionAccessCred     ActionType = "access_credential"
	ActionNetworkRequest ActionType = "network_request"
	ActionCommand        ActionType = "command"
)

type ResourceScope string

const (
	ResourceWorkspaceLocal   ResourceScope = "workspace_local"
	ResourceWorkspaceOutside ResourceScope = "workspace_outside"
	ResourceUnknown          ResourceScope = "unknown"
)

type Request struct {
	Action            ActionType
	TargetScopes      []ResourceScope
	DeleteCount       int
	UsesNetwork       bool
	TouchesCredential bool
	Command           string
	Details           string
}

type Result struct {
	Decision Decision
	Reason   string
}

type Policy struct {
	MassDeleteThreshold int
}

func New() Policy {
	return Policy{MassDeleteThreshold: 10}
}

func (p Policy) Evaluate(req Request) Result {
	if req.TouchesCredential || req.Action == ActionAccessCred {
		return blocked("credential access requires blocking")
	}
	if req.UsesNetwork || req.Action == ActionNetworkRequest {
		return blocked("network access requires blocking")
	}
	if req.Action == ActionDeploy {
		return blocked("deploy requires blocking")
	}
	if req.Action == ActionGitPush {
		return blocked("git push requires blocking")
	}
	if req.Action == ActionDelete {
		if req.DeleteCount >= p.massDeleteThreshold() {
			return blocked(fmt.Sprintf("mass delete of %d items requires blocking", req.DeleteCount))
		}
		if containsOutsideScope(req.TargetScopes) {
			return blocked("workspace-external delete requires blocking")
		}
		return allowed("local delete within workspace")
	}
	if req.Action == ActionWriteFile {
		if containsOutsideScope(req.TargetScopes) {
			return blocked("workspace-external write requires blocking")
		}
		return allowed("local write within workspace")
	}
	if req.Action == ActionReadFile || req.Action == ActionSearch || req.Action == ActionBuild || req.Action == ActionTest || req.Action == ActionLint || req.Action == ActionDiff {
		if containsOutsideScope(req.TargetScopes) {
			return blocked("workspace-external target requires blocking")
		}
		return allowed("safe workspace action")
	}
	if req.Action == ActionCommand {
		if containsOutsideScope(req.TargetScopes) {
			return blocked("workspace-external command target requires blocking")
		}
		return allowed("local command within workspace")
	}
	return blocked("unknown action requires review")
}

func (p Policy) massDeleteThreshold() int {
	if p.MassDeleteThreshold <= 0 {
		return 10
	}
	return p.MassDeleteThreshold
}

func containsOutsideScope(scopes []ResourceScope) bool {
	for _, scope := range scopes {
		if scope == ResourceWorkspaceOutside || scope == ResourceUnknown {
			return true
		}
	}
	return false
}

func allowed(reason string) Result {
	return Result{Decision: DecisionAllow, Reason: reason}
}

func blocked(reason string) Result {
	return Result{Decision: DecisionBlock, Reason: reason}
}

func (r Request) Validate() error {
	if strings.TrimSpace(string(r.Action)) == "" {
		return fmt.Errorf("action is required")
	}
	return nil
}
