package change

type SafetyController struct {
	MaxChangesPerHour int
	EmergencyFreeze   bool
}

func NewSafetyController(max int) *SafetyController {
	return &SafetyController{
		MaxChangesPerHour: max,
		EmergencyFreeze:   false,
	}
}

func (s *SafetyController) AllowChange(currentChanges int) bool {
	if s.EmergencyFreeze {
		return false
	}
	if currentChanges >= s.MaxChangesPerHour {
		return false
	}
	return true
}

func (s *SafetyController) Freeze() {
	s.EmergencyFreeze = true
}

func (s *SafetyController) Unfreeze() {
	s.EmergencyFreeze = false
}
