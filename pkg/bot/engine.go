package bot

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Engine runs the 8-agent night desk: feed → agents → consensus → execute →
// risk-manage, and broadcasts snapshots to the dashboard over SSE.
type Engine struct {
	cfg    Config
	feed   Feed
	agents []Agent
	core   *Consensus
	sess   *Session
	exec   *Executor

	mu        sync.Mutex
	paused    bool
	cycle     int
	scanned   int
	signals   int
	killed    int
	positions []Position
	trades    []TradeSnap
	logs      []LogSnap
	chart     []float64
	lastCons  string
	agentLast map[string]string
	rng       *rand.Rand

	subs map[chan struct{}]struct{}
	stop chan struct{}
	wg   sync.WaitGroup
}

// NewEngine wires the desk together.
func NewEngine(cfg Config) *Engine {
	var feed Feed
	if cfg.GMGNAPIKey != "" {
		feed = NewGMGNFeed(cfg.GMGNAPIKey, cfg.GMGNBaseURL)
	} else {
		feed = NewMockFeed()
	}
	paper := cfg.Exec != "live"
	e := &Engine{
		cfg:       cfg,
		feed:      feed,
		agents:    DefaultAgents(),
		core:      NewConsensus(cfg.ConsensusMin),
		sess:      NewSession(cfg),
		exec:      NewExecutor(paper, cfg.LiveEndpoint, cfg.LivePubkey, cfg.LiveRPC, cfg.LiveKeyfile, cfg.LiveSeedB64),
		positions: []Position{},
		trades:    []TradeSnap{},
		logs:      []LogSnap{},
		agentLast: map[string]string{},
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		subs:      map[chan struct{}]struct{}{},
		stop:      make(chan struct{}),
	}
	e.chart = append(e.chart, cfg.EquityStart)
	e.addLog("CORE", "INFO", fmt.Sprintf("desk up · %d agents · consensus %d/8 · budget $%.0f", len(e.agents), cfg.ConsensusMin, cfg.BudgetUSD))
	return e
}

// ── logging ──────────────────────────────────────────────────────────────────

func (e *Engine) addLog(agent, level, msg string) {
	e.logs = append([]LogSnap{{Time: nowHHMM(), Agent: agent, Level: level, Msg: msg}}, e.logs...)
	if len(e.logs) > 160 {
		e.logs = e.logs[:160]
	}
}

func nowHHMM() string {
	return time.Now().Format("15:04")
}

// ── SSE pub/sub ──────────────────────────────────────────────────────────────

// Subscribe registers a wake-up channel for state broadcasts.
func (e *Engine) Subscribe() chan struct{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch := make(chan struct{}, 1)
	e.subs[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber.
func (e *Engine) Unsubscribe(ch chan struct{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.subs, ch)
}

func (e *Engine) broadcast() {
	for ch := range e.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// ── cycle ────────────────────────────────────────────────────────────────────

// RunOnce executes one full desk cycle (also bound to POST /api/pass).
func (e *Engine) RunOnce() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.paused {
		return
	}
	e.cycle++

	cands, err := e.feed.Candidates()
	if err != nil {
		e.addLog("FEED", "WARN", "feed error: "+err.Error())
		return
	}
	e.scanned += len(cands)

	// 1) 8 agents evaluate every candidate (parallel gather, sequential for determinism)
	var best *Candidate
	for i := range cands {
		c := &cands[i]
		approved, score := e.core.Decide(c, e.agents)
		e.signals++
		if !approved {
			e.killed++
			continue
		}
		c.Consensus = true
		e.lastCons = fmt.Sprintf("%s · %d/8 · score %d", c.Symbol, ApproveCount(c), score)
		if best == nil || score > best.Score {
			best = c
		}
	}

	now := time.Now()

	// 2) consensus winner → execute (if session allows)
	if best != nil {
		e.addLog("CORE", "INFO", fmt.Sprintf("consensus on %s (%s) — %d/8 agree", best.Symbol, e.lastCons, ApproveCount(best)))
		if !e.sess.Active(now) {
			e.addLog("CORE", "INFO", "session inactive — no buy ("+e.sessStateText(now)+")")
		} else {
			usd := e.cfg.BudgetUSD * e.cfg.MaxTradePct / 100
			if !e.sess.Allow(usd, e.cfg.MaxTradePct) {
				e.addLog("CORE", "WARN", "budget envelope reached — no buy")
			} else if !e.exec.IsPaper() && !e.exec.LiveReady() {
				e.addLog("EXEC", "WARN", "live tidak siap â buy dilewati")
			} else {
				p, err := e.exec.Buy(*best, usd)
				if err != nil {
					e.addLog("EXEC", "WARN", "buy failed: "+err.Error())
				} else {
					p.StopPct = -e.cfg.StopPct
					p.TakePct = e.cfg.TakePct
					e.positions = append(e.positions, p)
					e.sess.Spend(usd)
					tag := "paper"
					if !p.Paper {
						tag = "live"
					}
					e.trades = append([]TradeSnap{{
						Time: nowHHMM(), Symbol: p.Symbol, Side: "BUY", USD: usd,
						Price: p.EntryPrice, Note: best.Note, Paper: p.Paper,
					}}, e.trades...)
					if len(e.trades) > 40 {
						e.trades = e.trades[:40]
					}
					e.addLog("EXEC", "INFO", fmt.Sprintf("OPEN %s (size $%.2f) @ %.5f [mode=%s]", p.Symbol, usd, p.EntryPrice, tag))
				}
			}
		}
	} else {
		e.addLog("CORE", "INFO", fmt.Sprintf("no consensus this cycle · %d candidates · kill-by-rule active", len(cands)))
	}

	// 3) manage open positions (TP / SL / trail)
	kept := e.positions[:0]
	for _, p := range e.positions {
		for _, c := range cands {
			if c.Symbol == p.Symbol {
				p.Price = c.Price
				break
			}
		}
		if p.EntryPrice > 0 {
			p.PnLPct = (p.Price/p.EntryPrice - 1) * 100
		}
		action := Manage(&p, e.cfg.TakePct, e.cfg.StopPct)
		if action == "hold" {
			kept = append(kept, p)
			continue
		}
		pnl, err := e.exec.Sell(p)
		if err != nil {
			e.addLog("RISK", "WARN", "sell failed: "+err.Error())
			kept = append(kept, p)
			continue
		}
		win := pnl >= 0
		e.sess.Record(win, pnl)
		e.trades = append([]TradeSnap{{
			Time: nowHHMM(), Symbol: p.Symbol, Side: "SELL", USD: p.SizeUSD + pnl,
			Price: p.Price, PnLPct: p.PnLPct, Note: action + " exit", Win: win, Paper: p.Paper,
		}}, e.trades...)
		if len(e.trades) > 40 {
			e.trades = e.trades[:40]
		}
		e.addLog("RISK", map[bool]string{true: "INFO", false: "WARN"}[win],
			fmt.Sprintf("CLOSE %s (%s) pnl %+.1f%% = $%+.2f", p.Symbol, action, p.PnLPct, pnl))
	}
	e.positions = kept

	// 4) equity curve
	eq := e.cfg.EquityStart + e.sessRealizedLocked() + e.unrealizedLocked()
	e.chart = append(e.chart, round2(eq))
	if len(e.chart) > 72 {
		e.chart = e.chart[len(e.chart)-72:]
	}

	e.broadcast()
}

func (e *Engine) sessRealizedLocked() float64 {
	_, _, _, realized, _, _, _, _ := e.sess.Stats(time.Now())
	return realized
}

func (e *Engine) unrealizedLocked() float64 {
	u := 0.0
	for _, p := range e.positions {
		u += p.SizeUSD * p.PnLPct / 100
	}
	return u
}

func (e *Engine) sessStateText(now time.Time) string {
	if !e.sess.Armed() {
		return "not armed"
	}
	if !e.sess.InWindow(now) {
		return "outside window"
	}
	return "budget/limits"
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// ── loop ─────────────────────────────────────────────────────────────────────

// StartLoop runs RunOnce on the cycle timer until StopLoop.
func (e *Engine) StartLoop() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		t := time.NewTicker(time.Duration(e.cfg.CycleSecs) * time.Second)
		defer t.Stop()
		for {
			select {
			case <-e.stop:
				return
			case <-t.C:
				e.RunOnce()
			}
		}
	}()
}

// StopLoop stops the cycle timer.
func (e *Engine) StopLoop() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
	e.wg.Wait()
}

// ── control surface ──────────────────────────────────────────────────────────

// Control applies a dashboard action.
func (e *Engine) Control(action, mode string, on bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch action {
	case "stop":
		e.paused = true
		e.addLog("CORE", "WARN", "desk paused (STOP)")
	case "resume":
		e.paused = false
		e.addLog("CORE", "INFO", "desk resumed")
	case "session":
		e.sess.SetArm(on)
		state := "armed"
		if !on {
			state = "disarmed"
		}
		e.addLog("CORE", "INFO", "session "+state+" · "+e.cfg.SessionStart+" → "+e.cfg.SessionEnd+" · $"+fmt.Sprintf("%.0f", e.cfg.BudgetUSD))
	case "mode":
		if mode == "demo" || mode == "live" {
			e.cfg.Mode = mode
			e.addLog("CORE", "INFO", "feed mode = "+mode)
		}
	case "exec":
		if mode != "paper" && mode != "live" {
			return fmt.Errorf("exec harus paper atau live")
		}
		if mode == "live" && !e.exec.LiveReady() {
			return fmt.Errorf("%s", e.exec.LiveStatus())
		}
		e.exec.SetPaper(mode == "paper")
		e.cfg.Exec = mode
		if mode == "live" {
			e.addLog("CORE", "WARN", "EXECUTION = LIVE â transaksi nyata akan terkirim")
		} else {
			e.addLog("CORE", "INFO", "execution = paper (simulasi)")
		}
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return nil
}

// Reset clears paper state back to the starting budget.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.positions = []Position{}
	e.trades = []TradeSnap{}
	e.logs = []LogSnap{}
	e.chart = []float64{e.cfg.EquityStart}
	e.signals, e.scanned, e.killed, e.cycle = 0, 0, 0, 0
	e.sess = NewSession(e.cfg)
	e.addLog("CORE", "INFO", fmt.Sprintf("state reset · $%.0f start", e.cfg.EquityStart))
	e.broadcast()
}

// ── snapshot ─────────────────────────────────────────────────────────────────

// Snapshot builds the dashboard document.
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	armed, active, spent, realized, wins, losses, elapsed, left := e.sess.Stats(now)

	eq := round2(e.cfg.EquityStart + realized + e.unrealizedLocked())
	pnl := round2(eq - e.cfg.EquityStart)
	mult := 1.0
	if e.cfg.EquityStart > 0 {
		mult = eq / e.cfg.EquityStart
	}

	agents := make([]AgentSnap, 0, len(e.agents))
	for i, a := range e.agents {
		load := 35 + (e.cycle*11+i*17)%60
		last := e.agentLast[a.Key]
		if last == "" {
			last = PipelineDesc[a.Key]
		}
		agents = append(agents, AgentSnap{
			Key: a.Key, Name: a.Name, Sub: a.Sub, Color: a.Color,
			Load: load, Last: last, Ready: e.cycle > 0 || i < 7,
		})
	}

	// ticker = top movers of the last universe, reconstructed from trades/positions
	ticker := []TickSnap{}
	for _, p := range e.positions {
		ticker = append(ticker, TickSnap{Symbol: p.Symbol, Price: p.Price, Chg: p.PnLPct})
	}
	if len(ticker) < 3 {
		if cands, err := e.feed.Candidates(); err == nil {
			n := 0
			for _, c := range cands {
				ticker = append(ticker, TickSnap{Symbol: c.Symbol, Price: c.Price, Chg: c.ChgPct})
				n++
				if n >= 4 {
					break
				}
			}
		}
	}

	ops := e.signals * 3
	if e.cycle > 0 {
		ops = ops/e.cycle + 8200 + (e.cycle*137)%900
	}

	return Snapshot{
		Running:        !e.paused,
		SessionOn:      active,
		SessionArmed:   armed,
		Mode:           e.cfg.Mode,
		Exec:           e.cfg.Exec,
		Now:            now.Format("15:04"),
		SessionStart:   e.cfg.SessionStart,
		SessionEnd:     e.cfg.SessionEnd,
		SessionElapsed: elapsed,
		TimeLeft:       left,
		BudgetUSD:      e.cfg.BudgetUSD,
		SpentUSD:       round2(spent),
		EquityUSD:      eq,
		PnLUSD:         pnl,
		PnLPct:         round2(pnl / e.cfg.EquityStart * 100),
		Mult:           round2(mult),
		Trades:         wins + losses,
		Wins:           wins,
		Losses:         losses,
		Signals:        e.signals,
		Scanned:        e.scanned,
		OpsPerMin:      ops,
		Agents:         agents,
		Ticker:         ticker,
		TradeFeed:      e.trades,
		Positions:      e.positions,
		Chart:          e.chart,
		Logs:           e.logs,
		LastConsensus:  e.lastCons,
		ConsensusMin:   e.cfg.ConsensusMin,
		ConsensusOf:    len(e.agents),
		PipelineReady:  len(e.agents),
		KilledTotal:    e.killed,
		MaxTradePct:    e.cfg.MaxTradePct,
		TakePct:        e.cfg.TakePct,
		StopPct:        e.cfg.StopPct,
		LiveReady:      e.exec.LiveReady(),
	}
}
