package season

// EnumerateActions returns the concrete legal action surface for a tick.
// The first action is always hold, followed by non-hold actions in stable
// opportunity/command/option order.
func EnumerateActions(tick TickDefinition) []SimulatedAction {
	actions := []SimulatedAction{{Command: "hold"}}
	for _, opportunity := range tick.Opportunities {
		for _, command := range opportunity.AllowedCommands {
			if command == "hold" {
				continue
			}
			if len(opportunity.AllowedOptions) == 0 {
				actions = append(actions, SimulatedAction{
					Command: command,
					Target:  opportunity.OpportunityID,
				})
				continue
			}
			for _, option := range opportunity.AllowedOptions {
				actions = append(actions, SimulatedAction{
					Command: command,
					Target:  opportunity.OpportunityID,
					Option:  option,
				})
			}
		}
	}
	return actions
}
