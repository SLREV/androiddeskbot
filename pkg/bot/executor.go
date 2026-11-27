package bot

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"
)

// Executor fills trades in paper mode, or delegates to the live router.
type Executor struct {
	paper bool
	live  *LiveRouter
}

// NewExecutor builds the executor. paper=true simulates fills (demo default).
// Live wiring: builder endpoint (returns unsigned tx), Solana RPC endpoint,
// wallet pubkey (base58), and a key source — Solana CLI keyfile (LIVE_WALLET_KEYFILE)
// or a 32-byte base64 seed (LIVE_WALLET_SEED_B64). A bad key never crashes
// startup; it only keeps live disabled with a clear status message.
func NewExecutor(paper bool, liveEndpoint, pubkey, rpcEndpoint, keyfile, seedB64 string) *Executor {
	key, keyErr := loadLiveKey(keyfile, seedB64)
	lr := &LiveRouter{
		endpoint: liveEndpoint,
		pubkey:   pubkey,
		rpc:      rpcEndpoint,
		key:      key,
		keyErr:   keyErr,
		client:   &http.Client{Timeout: 20 * time.Second},
	}
	// sanity check: pubkey harus cocok dengan key yang dimuat
	if keyErr == nil && key != nil && pubkey != "" {
		want, err := base58Decode(pubkey)
		if err != nil {
			lr.keyErr = fmt.Errorf("LIVE_WALLET_PUBKEY bukan base58 valid: %w", err)
		} else if !bytes.Equal(want, key.Public().(ed25519.PublicKey)) {
			lr.keyErr = errors.New("LIVE_WALLET_PUBKEY tidak cocok dengan private key — periksa .env")
		}
	}
	return &Executor{paper: paper, live: lr}
}

// SetPaper switches execution mode at runtime (paper ⇄ live).
func (e *Executor) SetPaper(paper bool) { e.paper = paper }

// IsPaper reports the current execution mode.
func (e *Executor) IsPaper() bool { return e.paper }

// LiveReady reports whether the live router is fully configured and capable of
// signing + sending a transaction right now.
func (e *Executor) LiveReady() bool { return e.live.ready() }

// LiveStatus describes live-readiness in one human line (doctor + error msgs).
func (e *Executor) LiveStatus() string { return e.live.status() }

// Buy opens a position of usd at the candidate price (+0.5% slippage).
func (e *Executor) Buy(c Candidate, usd float64) (Position, error) {
	if e.paper {
		fill := c.Price * 1.005
		return Position{
			Symbol:     c.Symbol,
			Mint:       c.Mint,
			SizeUSD:    usd,
			EntryPrice: fill,
			Price:      fill,
			Paper:      true,
			TxSig:      "paper-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Opened:     time.Now(),
		}, nil
	}
	sig, err := e.live.Buy(c.Mint, usd)
	if err != nil {
		return Position{}, err
	}
	return Position{
		Symbol:     c.Symbol,
		Mint:       c.Mint,
		SizeUSD:    usd,
		EntryPrice: c.Price,
		Price:      c.Price,
		Paper:      false,
		TxSig:      sig,
		Opened:     time.Now(),
	}, nil
}

// Sell closes a position at its current price; returns realized USD PnL.
func (e *Executor) Sell(p Position) (float64, error) {
	if e.paper {
		return p.SizeUSD * p.PnLPct / 100, nil
	}
	if err := e.live.Sell(p.Mint); err != nil {
		return 0, err
	}
	return p.SizeUSD * p.PnLPct / 100, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// LiveRouter — pola "unsigned tx + sign lokal":
//   1. POST trade-intent ke trade-builder (LIVE_TRADE_ENDPOINT) → tx unsigned.
//   2. SignAndSend: transaksi ditandatangani LOKAL dengan ed25519 (stdlib),
//      private key TIDAK PERNAH keluar dari proses ini.
//   3. Kirim raw tx ke RPC (LIVE_RPC_ENDPOINT) via JSON-RPC sendTransaction.
// Selama konfigurasi belum lengkap, setiap panggilan gagal dengan pesan jelas —
// tidak ada transaksi buta.
// ─────────────────────────────────────────────────────────────────────────────

// LiveRouter talks to a pump.fun trade-builder service and a Solana RPC.
type LiveRouter struct {
	endpoint string // LIVE_TRADE_ENDPOINT — trade-builder (tx unsigned)
	pubkey   string // LIVE_WALLET_PUBKEY  — base58
	rpc      string // LIVE_RPC_ENDPOINT   — JSON-RPC Solana
	key      ed25519.PrivateKey
	keyErr   error // kesalahan pemuatan key (memblokir live, bukan crash)
	client   *http.Client
}

type tradeIntent struct {
	Action string  `json:"action"`
	Mint   string  `json:"mint"`
	Amount float64 `json:"amount"`
	PubKey string  `json:"publicKey"`
}

type tradeBuild struct {
	Transaction string `json:"transaction"` // base64 unsigned tx (kunci utama)
	Tx          string `json:"tx"`          // fallback: beberapa builder memakai "tx"
	UnsignedTx  string `json:"unsigned_tx"` // fallback lain
	TradeID     string `json:"trade_id"`
}

func (r *LiveRouter) ready() bool {
	return r.endpoint != "" && r.pubkey != "" && r.rpc != "" && r.key != nil && r.keyErr == nil
}

// status → satu baris keterangan untuk manusia.
func (r *LiveRouter) status() string {
	if r.keyErr != nil {
		return "live belum siap: " + r.keyErr.Error()
	}
	var missing []string
	if r.endpoint == "" {
		missing = append(missing, "LIVE_TRADE_ENDPOINT")
	}
	if r.rpc == "" {
		missing = append(missing, "LIVE_RPC_ENDPOINT")
	}
	if r.pubkey == "" {
		missing = append(missing, "LIVE_WALLET_PUBKEY")
	}
	if r.key == nil {
		missing = append(missing, "LIVE_WALLET_KEYFILE atau LIVE_WALLET_SEED_B64")
	}
	if len(missing) > 0 {
		return "live belum siap: isi " + strings.Join(missing, " + ") + " di .env"
	}
	return "live siap: endpoint + rpc + wallet terkonfigurasi (sign lokal aktif)"
}

// Buy requests an unsigned buy transaction, signs it locally, sends it.
func (r *LiveRouter) Buy(mint string, usd float64) (string, error) {
	if !r.ready() {
		return "", errors.New(r.status())
	}
	body, _ := json.Marshal(tradeIntent{Action: "buy", Mint: mint, Amount: usd, PubKey: r.pubkey})
	res, err := r.client.Post(r.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("trade-builder tidak terjangkau: %w", err)
	}
	defer res.Body.Close()
	var b tradeBuild
	if err := json.NewDecoder(res.Body).Decode(&b); err != nil {
		return "", fmt.Errorf("respons builder tidak valid: %w", err)
	}
	tx := firstNonEmpty(b.Transaction, b.Tx, b.UnsignedTx)
	if tx == "" {
		return "", errors.New("builder returned empty transaction")
	}
	return r.SignAndSend(tx)
}

// Sell requests an unsigned sell transaction, signs it locally, sends it.
func (r *LiveRouter) Sell(mint string) error {
	if !r.ready() {
		return errors.New(r.status())
	}
	body, _ := json.Marshal(tradeIntent{Action: "sell", Mint: mint, PubKey: r.pubkey})
	res, err := r.client.Post(r.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("trade-builder tidak terjangkau: %w", err)
	}
	defer res.Body.Close()
	var b tradeBuild
	if err := json.NewDecoder(res.Body).Decode(&b); err != nil {
		return fmt.Errorf("respons builder tidak valid: %w", err)
	}
	tx := firstNonEmpty(b.Transaction, b.Tx, b.UnsignedTx)
	if tx == "" {
		return errors.New("builder returned empty transaction")
	}
	_, err = r.SignAndSend(tx)
	return err
}

// SignAndSend menandatangani transaksi Solana secara LOKAL (ed25519, stdlib)
// lalu mengirimkannya ke RPC. Alur:
//
//	decode base64 → [compact-u16 jumlah signature][signature][message]
//	sign(message) → ganti signature pertama → POST sendTransaction (base64)
//
// Private key tidak pernah keluar dari proses ini; trade-builder hanya melihat
// transaksi unsigned. Mengembalikan signature base58 dari RPC.
func (r *LiveRouter) SignAndSend(txB64 string) (string, error) {
	if r.key == nil {
		return "", errors.New(r.status())
	}
	raw, err := decodeB64(txB64)
	if err != nil {
		return "", fmt.Errorf("decode tx base64: %w", err)
	}
	n, off, err := compactU16(raw)
	if err != nil {
		return "", err
	}
	if n < 1 || off+n*64 > len(raw) {
		return "", errors.New("struktur transaksi tidak valid (signature count tidak konsisten)")
	}
	msg := raw[off+n*64:] // message = sisa byte setelah slot signature
	if len(msg) == 0 {
		return "", errors.New("struktur transaksi tidak valid (message kosong)")
	}
	sig := ed25519.Sign(r.key, msg)
	signed := make([]byte, 0, len(raw))
	signed = append(signed, raw[:off]...)    // prefix + compact-u16 count
	signed = append(signed, sig...)          // signature asli menggantikan placeholder
	signed = append(signed, raw[off+64:]...) // signature lain (bila ada) + message
	return r.sendRaw(signed)
}

// sendRaw submits a signed transaction via JSON-RPC sendTransaction.
func (r *LiveRouter) sendRaw(signed []byte) (string, error) {
	payload := map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "sendTransaction",
		"params": []any{
			base64.StdEncoding.EncodeToString(signed),
			map[string]any{"encoding": "base64", "skipPreflight": false},
		},
	}
	body, _ := json.Marshal(payload)
	res, err := r.client.Post(r.rpc, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("RPC tidak terjangkau: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("respons RPC tidak valid: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("RPC error: %s", out.Error.Message)
	}
	if out.Result == "" {
		return "", errors.New("RPC tidak mengembalikan signature")
	}
	return out.Result, nil
}

// ── helpers crypto/format ────────────────────────────────────────────────────

// loadLiveKey memuat kunci dari file keypair Solana CLI (JSON array 64/32 angka)
// atau seed base64 32 byte. Kembalikan (nil, nil) bila belum dikonfigurasi.
func loadLiveKey(keyfile, seedB64 string) (ed25519.PrivateKey, error) {
	if s := strings.TrimSpace(seedB64); s != "" {
		seed, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("LIVE_WALLET_SEED_B64 bukan base64: %w", err)
		}
		if len(seed) != 32 {
			return nil, fmt.Errorf("LIVE_WALLET_SEED_B64 harus 32 byte, dapat %d", len(seed))
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if f := strings.TrimSpace(keyfile); f != "" {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("baca LIVE_WALLET_KEYFILE: %w", err)
		}
		var arr []int
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, fmt.Errorf("LIVE_WALLET_KEYFILE harus JSON array angka (format keypair Solana CLI): %w", err)
		}
		n := len(arr)
		if n != 64 && n != 32 {
			return nil, fmt.Errorf("LIVE_WALLET_KEYFILE harus 64 atau 32 angka, dapat %d", n)
		}
		seed := make([]byte, 32)
		for i := range seed {
			if arr[i] < 0 || arr[i] > 255 {
				return nil, errors.New("LIVE_WALLET_KEYFILE berisi angka di luar 0–255")
			}
			seed[i] = byte(arr[i])
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	return nil, nil // belum dikonfigurasi — bukan error
}

// decodeB64 menerima base64 standar maupun tanpa padding.
func decodeB64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

// compactU16 membaca Solana shortvec (compact-u16): nilai + offset selesai.
func compactU16(b []byte) (int, int, error) {
	v, shift := 0, 0
	for i := 0; i < len(b) && i < 3; i++ {
		v |= int(b[i]&0x7f) << shift
		if b[i]&0x80 == 0 {
			return v, i + 1, nil
		}
		shift += 7
	}
	return 0, 0, errors.New("compact-u16 tidak valid")
}

// base58Decode mengubah string base58 (alfabet Bitcoin/Solana) ke bytes.
func base58Decode(s string) ([]byte, error) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("string base58 kosong")
	}
	var n big.Int
	base := big.NewInt(58)
	for _, c := range s {
		idx := strings.IndexRune(alphabet, c)
		if idx < 0 {
			return nil, fmt.Errorf("karakter base58 tidak valid: %q", c)
		}
		n.Mul(&n, base)
		n.Add(&n, big.NewInt(int64(idx)))
	}
	raw := n.Bytes()
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	out := make([]byte, zeros+len(raw))
	copy(out[zeros:], raw)
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
