package context

// Source: context/QueuedMessageContext.tsx

// QueuedMessageState tracks whether a message is queued for display.
// Go equivalent of TS QueuedMessageContextValue.
type QueuedMessageState struct {
	IsQueued     bool
	IsFirst      bool
	PaddingWidth int // width reduction for container padding
}

// DefaultQueuedMessageState returns a non-queued state.
func DefaultQueuedMessageState() QueuedMessageState {
	return QueuedMessageState{PaddingWidth: 4} // PADDING_X=2 * 2 sides = 4
}
