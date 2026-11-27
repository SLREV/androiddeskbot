package bot

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration (env-driven, .env-friendly).
type Config struct {
	Bind         string
	Port         int
	Mode         string // demo | live
	Exec         string // paper | live
	GMGNAPIKey   string
	GMGNBaseURL  string
	BudgetUSD    float64 // session envelope, e.g. 95
	MaxTradePct  float64 // max % of budget per trade, e.g. 10
	TakePct      float64 // take-profit per position, e.g. 25
	StopPct      float64 // hard stop per position, e.g. 6
	ConsensusMin int     // approvals required of 8 agents, e.g. 6
	SessionStart string  // "22:15"
	SessionEnd   string  // "08:15"
	CycleSecs    int
	DataDir      string
	LiveEndpoint string // LIVE_TRADE_ENDPOINT  — trade-builder (tx unsigned)
	LivePubkey   string // LIVE_WALLET_PUBKEY   — base58
	LiveRPC      string // LIVE_RPC_ENDPOINT    — JSON-RPC Solana
	LiveKeyfile  string // LIVE_WALLET_KEYFILE  — keypair Solana CLI (JSON)
	LiveSeedB64  string // LIVE_WALLET_SEED_B64 — alternatif: seed 32-byte base64
	EquityStart  float64
}

// LoadConfig reads configuration from environment with sane demo defaults.
func LoadConfig() Config {
	cfg := Config{
		Bind:         env("DESK_BIND", "127.0.0.1"),
		Port:         envInt("DESK_PORT", 8788),
		Mode:         env("DESK_MODE", "demo"),
		Exec:         env("DESK_EXECUTION_MODE", "paper"),
		GMGNAPIKey:   os.Getenv("GMGN_API_KEY"),
		GMGNBaseURL:  env("GMGN_BASE_URL", "https://api.gmgn.ai"),
		BudgetUSD:    envFloat("SESSION_BUDGET_USD", 95),
		MaxTradePct:  envFloat("MAX_TRADE_PCT", 10),
		TakePct:      envFloat("TAKE_PCT", 25),
		StopPct:      envFloat("STOP_PCT", 6),
		ConsensusMin: envInt("CONSENSUS_MIN", 6),
		SessionStart: env("SESSION_START", "22:15"),
		SessionEnd:   env("SESSION_END", "08:15"),
		CycleSecs:    envInt("CYCLE_SECS", 20),
		DataDir:      env("DESK_DATA_DIR", "."),
		LiveEndpoint: os.Getenv("LIVE_TRADE_ENDPOINT"),
		LivePubkey:   os.Getenv("LIVE_WALLET_PUBKEY"),
		LiveRPC:      os.Getenv("LIVE_RPC_ENDPOINT"),
		LiveKeyfile:  os.Getenv("LIVE_WALLET_KEYFILE"),
		LiveSeedB64:  os.Getenv("LIVE_WALLET_SEED_B64"),
	}
	cfg.EquityStart = cfg.BudgetUSD
	if cfg.ConsensusMin < 1 {
		cfg.ConsensusMin = 1
	}
	if cfg.ConsensusMin > 8 {
		cfg.ConsensusMin = 8
	}
	return cfg
}

// HHMMToMin parses "HH:MM" into minutes since midnight; ok=false when invalid.
func HHMMToMin(s string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
