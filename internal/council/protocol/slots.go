package protocol

import (
	"fmt"

	councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"
)

func (s *SlotRuntimes) Validate() error {
	if s == nil {
		return fmt.Errorf("slot runtimes are required")
	}
	required := []struct {
		name string
		rt   councilruntime.AgentRuntime
	}{
		{"researcher-1", s.Researcher1}, {"researcher-2", s.Researcher2},
		{"reviewer-1", s.Reviewer1}, {"reviewer-2", s.Reviewer2},
		{"challenger", s.Challenger}, {"judge-1", s.Judge1}, {"judge-2", s.Judge2},
	}
	for _, item := range required {
		if item.rt == nil {
			return fmt.Errorf("adaptive slot %s runtime is required", item.name)
		}
	}
	return nil
}

func (e Engine) validateRuntimes() error {
	if e.Slots != nil {
		return e.Slots.Validate()
	}
	if e.Claude == nil || e.Codex == nil {
		return fmt.Errorf("both Claude and Codex runtimes are required")
	}
	if e.ChallengerProvider != councilruntime.ProviderClaude && e.ChallengerProvider != councilruntime.ProviderCodex {
		return fmt.Errorf("challenger provider must be explicitly set to claude or codex")
	}
	return nil
}

func (e Engine) runtimeResearcher1() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Researcher1
	}
	return e.Claude
}
func (e Engine) runtimeResearcher2() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Researcher2
	}
	return e.Codex
}
func (e Engine) runtimeReviewer1() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Reviewer1
	}
	return e.Claude
}
func (e Engine) runtimeReviewer2() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Reviewer2
	}
	return e.Codex
}
func (e Engine) runtimeChallenger() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Challenger
	}
	if e.ChallengerProvider == councilruntime.ProviderCodex {
		return e.Codex
	}
	return e.Claude
}
func (e Engine) runtimeJudge1() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Judge1
	}
	return e.Claude
}
func (e Engine) runtimeJudge2() councilruntime.AgentRuntime {
	if e.Slots != nil {
		return e.Slots.Judge2
	}
	return e.Codex
}
