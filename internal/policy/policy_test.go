package policy_test

import (
	"testing"

	"gorchera/internal/policy"
)

func TestEvaluateAllowsSafeWorkspaceActions(t *testing.T) {
	t.Parallel()

	p := policy.New()
	cases := []policy.Request{
		{Action: policy.ActionReadFile},
		{Action: policy.ActionSearch},
		{Action: policy.ActionBuild},
		{Action: policy.ActionTest},
		{Action: policy.ActionLint},
		{Action: policy.ActionDiff},
		{Action: policy.ActionWriteFile, TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceLocal}},
		{Action: policy.ActionDelete, TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceLocal}, DeleteCount: 1},
		{Action: policy.ActionCommand, TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceLocal}},
	}

	for _, req := range cases {
		req := req
		t.Run(string(req.Action), func(t *testing.T) {
			t.Parallel()

			result := p.Evaluate(req)
			if result.Decision != policy.DecisionAllow {
				t.Fatalf("expected allow, got %s (%s)", result.Decision, result.Reason)
			}
		})
	}
}

func TestEvaluateBlocksRiskyActions(t *testing.T) {
	t.Parallel()

	p := policy.New()
	cases := []struct {
		name string
		req  policy.Request
	}{
		{
			name: "external write",
			req:  policy.Request{Action: policy.ActionWriteFile, TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceOutside}},
		},
		{
			name: "network request",
			req:  policy.Request{Action: policy.ActionNetworkRequest, UsesNetwork: true},
		},
		{
			name: "deploy",
			req:  policy.Request{Action: policy.ActionDeploy},
		},
		{
			name: "git push",
			req:  policy.Request{Action: policy.ActionGitPush},
		},
		{
			name: "credential access",
			req:  policy.Request{Action: policy.ActionAccessCred, TouchesCredential: true},
		},
		{
			name: "mass delete",
			req:  policy.Request{Action: policy.ActionDelete, TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceLocal}, DeleteCount: 10},
		},
		{
			name: "external delete",
			req:  policy.Request{Action: policy.ActionDelete, TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceOutside}, DeleteCount: 1},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := p.Evaluate(tc.req)
			if result.Decision != policy.DecisionBlock {
				t.Fatalf("expected block, got %s (%s)", result.Decision, result.Reason)
			}
			if result.Reason == "" {
				t.Fatal("expected blocking reason")
			}
		})
	}
}

func TestPolicyUsesDefaultMassDeleteThreshold(t *testing.T) {
	t.Parallel()

	p := policy.Policy{}
	result := p.Evaluate(policy.Request{
		Action:       policy.ActionDelete,
		TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceLocal},
		DeleteCount:  9,
	})
	if result.Decision != policy.DecisionAllow {
		t.Fatalf("expected allow below default threshold, got %s (%s)", result.Decision, result.Reason)
	}

	result = p.Evaluate(policy.Request{
		Action:       policy.ActionDelete,
		TargetScopes: []policy.ResourceScope{policy.ResourceWorkspaceLocal},
		DeleteCount:  10,
	})
	if result.Decision != policy.DecisionBlock {
		t.Fatalf("expected block at default threshold, got %s (%s)", result.Decision, result.Reason)
	}
}

func TestValidateRequiresAction(t *testing.T) {
	t.Parallel()

	if err := (policy.Request{}).Validate(); err == nil {
		t.Fatal("expected validation error for missing action")
	}
}
