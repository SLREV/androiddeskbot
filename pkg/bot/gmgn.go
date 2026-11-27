package bot

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// Feed yields the current candidate universe each cycle.
type Feed interface {
	Candidates() ([]Candidate, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// MockFeed: deterministic-ish demo universe (paper mode, works offline).
// Symbols follow the demo desk vocabulary ($MOGGO, $FLUMP, ...).
// ─────────────────────────────────────────────────────────────────────────────

type mockToken struct {
	symbol string
	mint   string
	price  float64
	base   float64
	ageMin int
	hot    int
}

// MockFeed simulates the pump.fun trenches each cycle.
type MockFeed struct {
	rng    *rand.Rand
	tokens []mockToken
	tick   int
}

// NewMockFeed builds the demo universe.
func NewMockFeed() *MockFeed {
	seeds := []mockToken{
		{"$MOGGO", "Moggo1111111111111111111111111111111111111", 0.0142, 0.0142, 42, 1},
		{"$FLUMP", "Flump111111111111111111111111111111111111", 0.0031, 0.0031, 95, 8},
		{"$DOOBS", "Doobs111111111111111111111111111111111111", 0.0008, 0.0008, 18, 3},
		{"$SKRIM", "Skrim111111111111111111111111111111111111", 0.0521, 0.0521, 260, 5},
		{"$MEOWX", "Meowx111111111111111111111111111111111111", 0.0004, 0.0004, 12, 2},
		{"$BONKAI", "Bonkai11111111111111111111111111111111111", 0.0019, 0.0019, 70, 6},
		{"$RAYBOT", "Raybot11111111111111111111111111111111111", 0.0088, 0.0088, 330, 10},
		{"$DOGE99", "Doge991111111111111111111111111111111111", 0.0002, 0.0002, 25, 4},
		{"$JUPPUP", "Juppup11111111111111111111111111111111111", 0.0061, 0.0061, 480, 12},
		{"$PEPINO", "Pepino11111111111111111111111111111111111", 0.0007, 0.0007, 8, 7},
	}
	return &MockFeed{rng: rand.New(rand.NewSource(20261127)), tokens: seeds}
}

// Candidates advances the universe one tick and returns fresh candidates.
func (f *MockFeed) Candidates() ([]Candidate, error) {
	f.tick++
	out := make([]Candidate, 0, len(f.tokens))
	for i := range f.tokens {
		t := &f.tokens[i]
		// price walk: −6% .. +14% per cycle, biased by hot rank
		drift := (f.rng.Float64()*0.20 - 0.06) + float64(14-t.hot)*0.002
		t.price = t.base * (1 + drift*0.5)
		t.ageMin += 1
		if t.ageMin > 600 {
			t.ageMin = 6 // relaunch sim
			t.base = t.base * (0.4 + f.rng.Float64()*1.2)
		}
		chg := drift * 100
		whale := f.rng.Float64()*120000 - 30000
		if t.hot <= 3 {
			whale += 40000
		}
		c := Candidate{
			Symbol:      t.symbol,
			Mint:        t.mint,
			Price:       t.price,
			ChgPct:      chg,
			VolUSD:      20000 + f.rng.Float64()*900000,
			LiqUSD:      45000 + f.rng.Float64()*500000,
			Top10Pct:    8 + f.rng.Float64()*40,
			DevPct:      f.rng.Float64() * 6,
			DevSold:     f.rng.Float64() < 0.12,
			WhaleNetUSD: whale,
			TraderScore: 20 + f.rng.Intn(80),
			SmartNetUSD: f.rng.Float64()*120000 - 25000,
			HotRank:     t.hot,
			AgeMin:      t.ageMin,
			Note:        fmt.Sprintf("hot #%d · vol $%.0fk", t.hot, (20000+f.rng.Float64()*900000)/1000),
		}
		if whale > 25000 {
			c.Note = fmt.Sprintf("whale +$%dk", int(whale/1000))
		}
		out = append(out, c)
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GMGNFeed: live data via GMGN OpenAPI (needs GMGN_API_KEY). On any error it
// falls back to MockFeed so the desk keeps running. Endpoint paths follow the
// GMGN OpenAPI skill surface (market/trending, trenches/new-pairs) — adjust to
// the current docs when you obtain a key: https://github.com/GMGNAI/gmgn-skills
// ─────────────────────────────────────────────────────────────────────────────

// GMGNFeed queries GMGN for the trenches universe.
type GMGNFeed struct {
	key    string
	base   string
	client *http.Client
	fb     *MockFeed
}

// NewGMGNFeed builds a live feed with automatic mock fallback.
func NewGMGNFeed(key, base string) *GMGNFeed {
	return &GMGNFeed{key: key, base: base, client: &http.Client{Timeout: 8 * time.Second}, fb: NewMockFeed()}
}

type gmgnResp struct {
	Code int `json:"code"`
	Data struct {
		List []struct {
			Symbol    string  `json:"symbol"`
			Address   string  `json:"address"`
			Price     float64 `json:"price"`
			Change1h  float64 `json:"change_1h"`
			Volume    float64 `json:"volume"`
			Liquidity float64 `json:"liquidity"`
			HotRank   int     `json:"hot_rank"`
			AgeMin    int     `json:"age_min"`
		} `json:"list"`
	} `json:"data"`
}

// Candidates returns the live universe, or the mock universe on failure.
func (g *GMGNFeed) Candidates() ([]Candidate, error) {
	req, err := http.NewRequest("GET", g.base+"/market/trending?chain=sol&limit=12", nil)
	if err != nil {
		return g.fb.Candidates()
	}
	req.Header.Set("X-API-KEY", g.key)
	res, err := g.client.Do(req)
	if err != nil {
		return g.fb.Candidates()
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return g.fb.Candidates()
	}
	var parsed gmgnResp
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil || len(parsed.Data.List) == 0 {
		return g.fb.Candidates()
	}
	out := make([]Candidate, 0, len(parsed.Data.List))
	for _, t := range parsed.Data.List {
		out = append(out, Candidate{
			Symbol:  t.Symbol,
			Mint:    t.Address,
			Price:   t.Price,
			ChgPct:  t.Change1h,
			VolUSD:  t.Volume,
			LiqUSD:  t.Liquidity,
			HotRank: t.HotRank,
			AgeMin:  t.AgeMin,
			Note:    "gmgn live",
		})
	}
	return out, nil
}
