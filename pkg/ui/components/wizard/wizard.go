// Package wizard provides a multi-step form/dialog framework.
// Source: components/wizard/ — WizardProvider.tsx, useWizard.ts, WizardDialogLayout.tsx
package wizard

// Step represents a single step in a wizard flow.
type Step struct {
	ID    string
	Title string
}

// Wizard tracks multi-step form state.
// Go equivalent of TS WizardProvider React context.
type Wizard struct {
	steps       []Step
	currentStep int
	data        map[string]any // accumulated form data
	completed   bool
	cancelled   bool
}

// New creates a wizard with the given steps.
func New(steps []Step) *Wizard {
	return &Wizard{
		steps: steps,
		data:  make(map[string]any),
	}
}

// CurrentStep returns the current step.
func (w *Wizard) CurrentStep() Step {
	if w.currentStep >= len(w.steps) {
		return Step{}
	}
	return w.steps[w.currentStep]
}

// CurrentIndex returns the zero-based step index.
func (w *Wizard) CurrentIndex() int { return w.currentStep }

// TotalSteps returns the number of steps.
func (w *Wizard) TotalSteps() int { return len(w.steps) }

// IsFirst returns true if on the first step.
func (w *Wizard) IsFirst() bool { return w.currentStep == 0 }

// IsLast returns true if on the last step.
func (w *Wizard) IsLast() bool { return w.currentStep == len(w.steps)-1 }

// Next advances to the next step. Returns false if already on the last step.
func (w *Wizard) Next() bool {
	if w.currentStep < len(w.steps)-1 {
		w.currentStep++
		return true
	}
	return false
}

// Prev goes back to the previous step. Returns false if already on the first step.
func (w *Wizard) Prev() bool {
	if w.currentStep > 0 {
		w.currentStep--
		return true
	}
	return false
}

// GoTo jumps to a specific step index.
func (w *Wizard) GoTo(index int) {
	if index >= 0 && index < len(w.steps) {
		w.currentStep = index
	}
}

// Set stores a value in the wizard's data map.
func (w *Wizard) Set(key string, value any) { w.data[key] = value }

// Get retrieves a value from the wizard's data map.
func (w *Wizard) Get(key string) any { return w.data[key] }

// Data returns all accumulated wizard data.
func (w *Wizard) Data() map[string]any { return w.data }

// Complete marks the wizard as completed.
func (w *Wizard) Complete() { w.completed = true }

// Cancel marks the wizard as cancelled.
func (w *Wizard) Cancel() { w.cancelled = true }

// IsCompleted returns true if the wizard finished successfully.
func (w *Wizard) IsCompleted() bool { return w.completed }

// IsCancelled returns true if the wizard was cancelled.
func (w *Wizard) IsCancelled() bool { return w.cancelled }

// Progress returns "Step X of Y" text.
func (w *Wizard) Progress() string {
	return "Step " + itoa(w.currentStep+1) + " of " + itoa(len(w.steps))
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
