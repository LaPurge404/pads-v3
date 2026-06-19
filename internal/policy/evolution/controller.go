package evolution

type Controller struct {
	Mode Mode
}

func NewController() *Controller {
	return &Controller{Mode: ModeStable}
}

func (c *Controller) Decide(candidate int, current int, confidence float64) bool {

	if c.Mode == ModeLocked {
		return false
	}

	if confidence < 0.4 {
		return false
	}

	return candidate >= current
}
