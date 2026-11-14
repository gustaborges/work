package skillinstall

import skills "github.com/gustaborges/work/ai-skills"

func planAll(ds []Destination, force bool) ([]Plan, []Outcome, bool) {
	plans := []Plan{}
	out := make([]Outcome, len(ds))
	ok := true
	for i, d := range ds {
		digest, err := skills.Digest(d.Skill)
		obs := Observation{}
		if err == nil {
			obs = Observe(d, digest)
		} else {
			obs.Result.Error = "skill-io-failed"
		}
		plans = append(plans, Plan{d, obs, force})
		out[i] = Outcome{Destination: d, Action: "not-attempted", Commit: "none"}
		token := obs.Result.Error
		if obs.Result.State == "recovery_required" {
			token = ""
		}
		if !force && (obs.Result.State == "foreign" || obs.Result.State == "modified") {
			token = "skill-target-conflict"
		}
		if token != "" {
			ok = false
			out[i].Action = "failed"
			out[i].Error = token
		}
	}
	return plans, out, ok
}
