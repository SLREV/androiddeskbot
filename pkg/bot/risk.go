package bot

// Manage applies the per-trade risk rules: take-profit, hard stop, and a
// ratcheting trailing stop once a position is +10%.
// Returns "hold", "tp", "sl" or "trail".
func Manage(p *Position, takePct, stopPct float64) string {
	pnl := p.PnLPct
	if pnl >= takePct {
		return "tp"
	}
	if pnl <= -stopPct {
		return "sl"
	}
	// trailing stop: after +10%, never give back more than stopPct from peak
	if pnl >= 10 {
		trailStop := pnl - stopPct
		if p.StopPct < trailStop {
			p.StopPct = trailStop // ratchet up
		}
		if pnl <= p.StopPct && p.StopPct > 0 {
			return "trail"
		}
	}
	return "hold"
}
