package proactivity

import "fmt"

func BuildPlan(p Perception, g Goal) Plan {
	switch g.Kind {
	case GoalVisitPark:
		return Plan{
			Goal: g,
			Steps: []string{
				fmt.Sprintf("Confirm the next two hours stay %s and dry.", p.NowWeather.Condition.DisplayName()),
				"Leave in about 30 minutes for a nearby park.",
				"Re-sense before leaving; drop the suggestion if rain or a meeting appears.",
			},
		}
	case GoalUmbrellaReminder:
		when := p.TomorrowAM.At
		whenText := when.Format("2006-01-02 15:04 MST")
		if when.IsZero() {
			whenText = "tomorrow 08:00"
		}
		return Plan{
			Goal: g,
			Steps: []string{
				"Ask the weather source for tomorrow 08:00 (already rain/storm in this tick).",
				"Create a calendar event and alarm at " + whenText + " via Shaoruru’s umbrella flow (not executed in this tick).",
				"Do not create a second reminder if the title already exists.",
			},
		}
	case GoalCommitmentNudge:
		when := g.WindowStart.Format("15:04")
		if g.WindowStart.IsZero() {
			when = "soon"
		}
		return Plan{
			Goal: g,
			Steps: []string{
				"Surface the remembered commitment before the start time (" + when + ").",
				"Do not create a calendar event; this is a memory nudge, not a new booking.",
			},
		}
	case GoalNone:
		return Plan{Goal: g, Steps: []string{"Keep sensing; do not notify."}}
	default:
		return Plan{Goal: g, Steps: []string{"Unhandled goal kind; do not notify."}}
	}
}
