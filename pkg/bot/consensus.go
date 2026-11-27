package bot

// Consensus is the "8 → 1" core: nothing gets bought unless enough agents
// agree, and the HOLDERS/DEV agents hold a hard veto on red flags.
type Consensus struct {
	MinApprove int
	VetoKeys   map[string]bool
}

// NewConsensus builds the default core (threshold configurable).
func NewConsensus(minApprove int) *Consensus {
	if minApprove < 1 {
		minApprove = 1
	}
	if minApprove > 8 {
		minApprove = 8
	}
	return &Consensus{
		MinApprove: minApprove,
		VetoKeys:   map[string]bool{"hold": true, "dev": true},
	}
}

// Decide runs all agents over the candidate and returns (approved, score).
// Score = average agent confidence (0..100).
func (k *Consensus) Decide(c *Candidate, agents []Agent) (bool, int) {
	approves, sum := 0, 0
	veto := false
	for _, a := range agents {
		v := a.fn(*c)
		c.Votes = append(c.Votes, v)
		if v.Approve {
			approves++
		} else if k.VetoKeys[a.Key] {
			veto = true
		}
		sum += v.Score
	}
	avg := 0
	if len(agents) > 0 {
		avg = sum / len(agents)
	}
	c.Score = avg
	if veto {
		return false, avg
	}
	return approves >= k.MinApprove, avg
}

// ApproveCount counts approving votes already stored on the candidate.
func ApproveCount(c *Candidate) int {
	n := 0
	for _, v := range c.Votes {
		if v.Approve {
			n++
		}
	}
	return n
}
