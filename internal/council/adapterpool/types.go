package adapterpool

import councilruntime "github.com/ShenJun93/agent-council/internal/council/runtime"

type AdapterID string
type SlotID string

type Adapter struct {
	ID       AdapterID
	Provider councilruntime.Provider
	Runtime  councilruntime.AgentRuntime
}

type Policy struct {
	Slot  SlotID
	Chain []AdapterID
}
