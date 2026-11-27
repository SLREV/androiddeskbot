package bot

import "fmt"

// Agent evaluates candidates for the consensus core.
type Agent struct {
	Key   string
	Name  string
	Sub   string
	Color string
	fn    func(Candidate) Vote
}

// DefaultAgents returns the 8-agent night desk (FEED..DEV), mirroring the
// pipeline: stream → rank → surge → smart traders → copy → holders → whales → dev.
func DefaultAgents() []Agent {
	return []Agent{
		{"feed", "FEED", "new pairs", "#2dd4a7", feedVote},
		{"rank", "RANK", "hot rank", "#38bdf8", rankVote},
		{"surge", "SURGE", "spikes 5m", "#fbbf24", surgeVote},
		{"alpha", "ALPHA", "top traders", "#fb923c", alphaVote},
		{"copy", "COPY", "mirror · TP/SL", "#f472b6", copyVote},
		{"hold", "HOLD", "top-10 share", "#60a5fa", holdVote},
		{"whale", "WHALE", "whale moves", "#a78bfa", whaleVote},
		{"dev", "DEV", "dev wallet", "#f87171", devVote},
	}
}

// PipelineDesc is the "pipeline" card copy shown in the UI.
var PipelineDesc = map[string]string{
	"feed":  "streams every new pair the second it lands",
	"rank":  "ranks what is hot right now",
	"surge": "flags volume spikes in 5 minutes",
	"alpha": "finds top traders already in",
	"copy":  "mirrors their size · TP/SL on every trade",
	"hold":  "checks the top-10 holder share",
	"whale": "watches whale buys and sells",
	"dev":   "checks the dev wallet · never sold",
}

func feedVote(c Candidate) Vote {
	ok := c.AgeMin <= 45
	s := 100 - c.AgeMin
	if s < 0 {
		s = 0
	}
	return Vote{"FEED", ok, s, fmt.Sprintf("age %dm", c.AgeMin)}
}

func rankVote(c Candidate) Vote {
	ok := c.HotRank <= 6
	s := 100 - (c.HotRank-1)*8
	if s < 0 {
		s = 0
	}
	return Vote{"RANK", ok, s, fmt.Sprintf("hot #%d", c.HotRank)}
}

func surgeVote(c Candidate) Vote {
	ok := c.ChgPct >= 15
	s := int(c.ChgPct * 2)
	if s > 100 {
		s = 100
	}
	if s < 0 {
		s = 0
	}
	return Vote{"SURGE", ok, s, fmt.Sprintf("%+.1f%% 1h", c.ChgPct)}
}

func alphaVote(c Candidate) Vote {
	return Vote{"ALPHA", c.TraderScore >= 60, c.TraderScore, fmt.Sprintf("traders %d", c.TraderScore)}
}

func copyVote(c Candidate) Vote {
	ok := c.SmartNetUSD > 5000
	s := int(c.SmartNetUSD / 1500)
	if s > 100 {
		s = 100
	}
	if s < 0 {
		s = 0
	}
	return Vote{"COPY", ok, s, fmt.Sprintf("smart %+.0fk", c.SmartNetUSD/1000)}
}

func holdVote(c Candidate) Vote {
	ok := c.Top10Pct <= 22
	s := int(100 - c.Top10Pct*2)
	if s < 0 {
		s = 0
	}
	return Vote{"HOLD", ok, s, fmt.Sprintf("top10 %.0f%%", c.Top10Pct)}
}

func whaleVote(c Candidate) Vote {
	ok := c.WhaleNetUSD > 0
	s := int(c.WhaleNetUSD / 1500)
	if s > 100 {
		s = 100
	}
	if s < 0 {
		s = 0
	}
	return Vote{"WHALE", ok, s, fmt.Sprintf("net %+.0fk", c.WhaleNetUSD/1000)}
}

func devVote(c Candidate) Vote {
	ok := c.DevPct <= 2.5 && !c.DevSold
	s := int(100 - c.DevPct*20)
	if s < 0 {
		s = 0
	}
	note := fmt.Sprintf("dev %.1f%%", c.DevPct)
	if c.DevSold {
		note = "dev SOLD"
	}
	return Vote{"DEV", ok, s, note}
}
