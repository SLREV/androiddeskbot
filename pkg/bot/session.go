package bot

import (
	"fmt"
	"sync"
	"time"
)

// Session is the night-mode envelope: a time window (e.g. 22:15 → 08:15),
// a fixed budget (e.g. $95) and per-trade sizing limits. This is what the
// "Connect pump.fun" dialog arms — budget, window, max %/trade, TP/SL.
type Session struct {
	mu        sync.Mutex
	startMin  int
	endMin    int
	budgetUSD float64
	spentUSD  float64
	realized  float64
	wins      int
	losses    int
	armed     bool
	startedAt time.Time
}

// NewSession creates the envelope from config.
func NewSession(cfg Config) *Session {
	s := &Session{budgetUSD: cfg.BudgetUSD, startedAt: time.Now()}
	if m, ok := HHMMToMin(cfg.SessionStart); ok {
		s.startMin = m
	}
	if m, ok := HHMMToMin(cfg.SessionEnd); ok {
		s.endMin = m
	}
	return s
}

// InWindow reports whether now is inside the configured clock window
// (handles overnight windows such as 22:15 → 08:15).
func (s *Session) InWindow(now time.Time) bool {
	m := now.Hour()*60 + now.Minute()
	if s.startMin == s.endMin {
		return true // 24h
	}
	if s.startMin < s.endMin {
		return m >= s.startMin && m < s.endMin
	}
	return m >= s.startMin || m < s.endMin // overnight wrap
}

// SetArm arms/disables the session (the Connect button).
func (s *Session) SetArm(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on && !s.armed {
		s.startedAt = time.Now()
	}
	s.armed = on
}

// Armed reports whether the envelope is armed.
func (s *Session) Armed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.armed
}

// Active reports whether trading is permitted now (armed and in window).
func (s *Session) Active(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.armed && s.InWindow(now)
}

// Allow checks the sizing rule: usd ≤ max % of budget and budget not exhausted.
func (s *Session) Allow(usd, maxTradePct float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if usd <= 0 || s.spentUSD+usd > s.budgetUSD+0.001 {
		return false
	}
	maxPer := s.budgetUSD * maxTradePct / 100
	return usd <= maxPer+0.001
}

// Spend records deployed capital.
func (s *Session) Spend(usd float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spentUSD += usd
}

// Record closes a trade into the win/loss scoreboard and realized PnL.
func (s *Session) Record(win bool, pnl float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.realized += pnl
	if win {
		s.wins++
	} else {
		s.losses++
	}
}

// Stats returns envelope numbers for the dashboard.
func (s *Session) Stats(now time.Time) (armed, active bool, spent, realized float64, wins, losses int, elapsed, left string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	armed, spent, realized, wins, losses = s.armed, s.spentUSD, s.realized, s.wins, s.losses
	active = s.armed && s.InWindow(now)
	elapsed = fmt.Sprintf("%dh %dm in", int(time.Since(s.startedAt).Hours()), int(time.Since(s.startedAt).Minutes())%60)
	// time left in window
	m := now.Hour()*60 + now.Minute()
	var leftMin int
	if s.startMin == s.endMin {
		leftMin = 0
	} else if s.startMin < s.endMin {
		if m >= s.startMin && m < s.endMin {
			leftMin = s.endMin - m
		}
	} else {
		if m >= s.startMin {
			leftMin = 24*60 - m + s.endMin
		} else if m < s.endMin {
			leftMin = s.endMin - m
		}
	}
	if leftMin > 0 {
		left = fmt.Sprintf("%dh %dm left", leftMin/60, leftMin%60)
	} else {
		left = "outside window"
	}
	return
}
