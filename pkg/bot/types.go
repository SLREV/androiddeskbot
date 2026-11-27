// Package bot is the full-stack core of Grok Bot: 8 data agents, a consensus
// core, a night-mode session envelope, risk management, and a paper/live
// executor for pump.fun bonding-curve tokens — plus the HTTP+SSE dashboard.
package bot

import "time"

// Vote is one agent's verdict on a candidate.
type Vote struct {
	Agent   string `json:"agent"`
	Approve bool   `json:"approve"`
	Score   int    `json:"score"` // 0..100 confidence
	Note    string `json:"note"`
}

// Candidate is a token proposed by the feed layer.
type Candidate struct {
	Symbol      string  `json:"symbol"`
	Mint        string  `json:"mint"`
	Price       float64 `json:"price"`
	ChgPct      float64 `json:"chg_pct"` // 1h change
	VolUSD      float64 `json:"vol_usd"`
	LiqUSD      float64 `json:"liq_usd"`
	Top10Pct    float64 `json:"top10_pct"`
	DevPct      float64 `json:"dev_pct"`
	DevSold     bool    `json:"dev_sold"`
	WhaleNetUSD float64 `json:"whale_net_usd"`
	TraderScore int     `json:"trader_score"` // 0..100 top-trader presence
	SmartNetUSD float64 `json:"smart_net_usd"`
	HotRank     int     `json:"hot_rank"` // 1 = hottest
	AgeMin      int     `json:"age_min"`  // minutes since launch
	Note        string  `json:"note"`     // feed headline, e.g. "whale +$28k"
	Votes       []Vote  `json:"votes"`
	Consensus   bool    `json:"consensus"`
	Score       int     `json:"score"`
}

// Position is an open paper/live position.
type Position struct {
	Symbol     string    `json:"symbol"`
	Mint       string    `json:"mint"`
	SizeUSD    float64   `json:"size_usd"`
	EntryPrice float64   `json:"entry_price"`
	Price      float64   `json:"price"`
	PnLPct     float64   `json:"pnl_pct"`
	StopPct    float64   `json:"stop_pct"`
	TakePct    float64   `json:"take_pct"`
	Paper      bool      `json:"paper"`
	TxSig      string    `json:"tx_sig"`
	Opened     time.Time `json:"opened"`
}

// TradeSnap is one closed trade row in the feed.
type TradeSnap struct {
	Time   string  `json:"time"`
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	USD    float64 `json:"usd"`
	Price  float64 `json:"price"`
	PnLPct float64 `json:"pnl_pct"`
	Note   string  `json:"note"`
	Win    bool    `json:"win"`
	Paper  bool    `json:"paper"`
}

// LogSnap is one logger line.
type LogSnap struct {
	Time  string `json:"time"`
	Agent string `json:"agent"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// AgentSnap is the live-load card of one agent.
type AgentSnap struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Sub   string `json:"sub"`
	Color string `json:"color"`
	Load  int    `json:"load"`
	Last  string `json:"last"`
	Ready bool   `json:"ready"`
}

// TickSnap is one ticker chip.
type TickSnap struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Chg    float64 `json:"chg"`
}

// Snapshot is the single JSON document the dashboard renders from.
type Snapshot struct {
	Running        bool        `json:"running"`
	SessionOn      bool        `json:"session_on"`
	SessionArmed   bool        `json:"session_armed"`
	Mode           string      `json:"mode"`
	Exec           string      `json:"exec"`
	Now            string      `json:"now"`
	SessionStart   string      `json:"session_start"`
	SessionEnd     string      `json:"session_end"`
	SessionElapsed string      `json:"session_elapsed"`
	TimeLeft       string      `json:"time_left"`
	BudgetUSD      float64     `json:"budget_usd"`
	SpentUSD       float64     `json:"spent_usd"`
	EquityUSD      float64     `json:"equity_usd"`
	PnLUSD         float64     `json:"pnl_usd"`
	PnLPct         float64     `json:"pnl_pct"`
	Mult           float64     `json:"mult"`
	Trades         int         `json:"trades"`
	Wins           int         `json:"wins"`
	Losses         int         `json:"losses"`
	Signals        int         `json:"signals"`
	Scanned        int         `json:"scanned"`
	OpsPerMin      int         `json:"ops_min"`
	Agents         []AgentSnap `json:"agents"`
	Ticker         []TickSnap  `json:"ticker"`
	TradeFeed      []TradeSnap `json:"trade_feed"`
	Positions      []Position  `json:"positions"`
	Chart          []float64   `json:"chart"`
	Logs           []LogSnap   `json:"logs"`
	LastConsensus  string      `json:"last_consensus"`
	ConsensusMin   int         `json:"consensus_min"`
	ConsensusOf    int         `json:"consensus_of"`
	PipelineReady  int         `json:"pipeline_ready"`
	KilledTotal    int         `json:"killed_total"`
	MaxTradePct    float64     `json:"max_trade_pct"`
	TakePct        float64     `json:"take_pct"`
	StopPct        float64     `json:"stop_pct"`
	LiveReady      bool        `json:"live_ready"`
}
