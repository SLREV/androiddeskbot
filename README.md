# 🌙 Grok Bot · pump.fun night mode — Android Kit

**Full-stack 8-agent trading desk untuk token pump.fun** — backend Go ringan
(murni stdlib), web UI terminal "night mode", dan **APK Android** dari kode yang
sama. Terinspirasi arsitektur *multi-agent consensus*: **nothing gets bought
unless they agree.**

> ⚠️ **Disclaimer**: perangkat lunak ini untuk edukasi & eksperimen (mode
> `paper` secara default). Trading token meme berisiko tinggi — gunakan burner
> wallet, jangan pernah menaruh private key utama. Live mode butuh konfigurasi
> eksplisit dan tidak aktif secara default.

---

## ✨ Fitur

| Lapisan | Isi |
|---|---|
| **8 agents** | `FEED` (new pairs) · `RANK` (hot rank) · `SURGE` (volume spike) · `ALPHA` (top traders) · `COPY` (mirror smart money) · `HOLD` (top-10 holder) · `WHALE` (whale moves) · `DEV` (dev wallet) |
| **Consensus core** | Voting **N-of-8** (default 6) + veto keras dari HOLD/DEV — kandidat dibeli hanya bila cukup banyak agent setuju |
| **Session envelope** | Dialog *"Connect pump.fun"*: window jam (default 22:15 → 08:15), budget ($95), maks %/trade (10%) |
| **Risk** | Take-profit (+25%), hard stop (−6%), **trailing stop** otomatis setelah +10% |
| **Eksekusi** | **Switch `PAPER ⇄ LIVE`** langsung di dashboard (dengan konfirmasi & guard) — `paper` (simulasi, default) / `live` via trade-builder API, pola *"unsigned tx + sign lokal"* (kunci tidak pernah dikirim ke pihak ketiga) |
| **Market feed** | Mock feed (demo, offline) atau **GMGN OpenAPI** live bila `GMGN_API_KEY` diisi |
| **Dashboard** | UI terminal night-mode: equity chart, trade feed, 8 kartu live-load, ring `8 → 1 → EXECUTE`, pipeline, night log, chat perintah — **SSE real-time**, responsif untuk HP/WebView |
| **Android** | `gomobile bind` → AAR + shell WebView ~40 baris Java → **APK ± 6–9 MB** |
| **Auth** | Token perangkat (`?token=…` + cookie HttpOnly) di semua endpoint |

## 🏗 Arsitektur

```
                    ┌────────────────────────────────────────────┐
                    │  web UI (embedded) — SSE real-time         │
                    │  splash → connect → dashboard night mode   │
                    └──────────────┬─────────────────────────────┘
                                   │ http://127.0.0.1:8788/?token=
   ┌───────────────────────────────▼────────────────────────────────┐
   │  pkg/bot  (Go, murni stdlib)                                   │
   │                                                                │
   │  Feed (GMGN / mock) ──► 8 Agents ──► Consensus (6/8 + veto)    │
   │                                │                               │
   │                                ▼                               │
   │  Session (window · budget · max%/trade) ──► Executor            │
   │        │                                    │                  │
   │        └── Risk: TP +25% / SL −6% / trail ◄─┘                  │
   │                                                                │
   │  HTTP + SSE + token auth  ·  Logger  ·  state broadcast        │
   └────────────────────────────────────────────────────────────────┘
        │                                        │
        ▼                                        ▼
   cmd/grokbot (CLI)                android/botlib (gomobile → APK)
```

## 🚀 Quickstart (desktop)

```bash
git clone https://github.com/USERNAME/grokbot-android-kit
cd grokbot-android-kit
go run ./cmd/grokbot            # → buka URL yang tercetak (ada token)
```

Perintah lain:

```bash
go run ./cmd/grokbot --once       # 1 cycle lalu print state
go run ./cmd/grokbot --doctor     # cek konfigurasi
go run ./cmd/grokbot --budget 50  # ganti budget sesi
```

Di dashboard: klik **Connect** (menyalakan sesi), tekan **▶ 1 Pass** untuk
menjalankan cycle manual, atau biarkan loop berjalan otomatis tiap 20 detik.
Gunakan **switch `PAPER / LIVE`** di header untuk berpindah mode eksekusi
kapan pun (mode live butuh konfigurasi dulu — lihat tabel di bawah). Chat
perintah: `pass · session on/off · paper · live · stop · resume · reset · status · help`.

## ⚙️ Konfigurasi

Salin `.env.example` → `.env` (atau pakai environment variable):

| Variabel | Default | Fungsi |
|---|---|---|
| `SESSION_START` / `SESSION_END` | `22:15` / `08:15` | Jendela night mode (bisa overnight) |
| `SESSION_BUDGET_USD` | `95` | Envelope budget sesi |
| `MAX_TRADE_PCT` | `10` | Maks % budget per trade |
| `TAKE_PCT` / `STOP_PCT` | `25` / `6` | TP / SL (+ trail otomatis) |
| `CONSENSUS_MIN` | `6` | Minimal agent yang harus setuju (dari 8) |
| `GMGN_API_KEY` | *(kosong)* | Isi untuk feed live GMGN — kosong = mock demo |
| `DESK_EXECUTION_MODE` | `paper` | `live` hanya setelah **4 variabel live** di bawah diisi |
| `CYCLE_SECS` | `20` | Interval cycle otomatis |
| `LIVE_TRADE_ENDPOINT` | *(kosong)* | Trade-builder API — mengembalikan transaksi **unsigned** |
| `LIVE_RPC_ENDPOINT` | *(kosong)* | RPC Solana untuk `sendTransaction` (mis. `https://api.mainnet-beta.solana.com`) |
| `LIVE_WALLET_PUBKEY` | *(kosong)* | Public key wallet sesi (base58) |
| `LIVE_WALLET_KEYFILE` | *(kosong)* | File keypair Solana CLI (`~/.config/solana/id.json`) — **atau** pakai `LIVE_WALLET_SEED_B64` (seed 32-byte base64) |

## 📱 Build APK Android

### Cara 0 (disarankan) — GitHub Actions, otomatis 🤖

Repo sudah menyertakan workflow **`build-apk.yml`**: setiap push ke `main`/
`master`, tag `v*`, atau manual (*Actions → build-apk → Run workflow*), CI akan:

1. `gomobile bind` → `android/app/libs/botlib.aar`
2. `gradle assembleDebug` → **`app-debug.apk`**
3. Upload hasil sebagai **artifact** `grokbot-debug-apk` — tinggal unduh dari
   halaman run Actions (tab Artifacts). Tanpa Android Studio lokal sama sekali.

### Cara 1 — build lokal

Prasyarat: Go ≥ 1.21, Android Studio (atau SDK+NDK), JDK 17.

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init

./scripts/build-android.sh
# → android/app/build/outputs/apk/debug/app-debug.apk  (± 6–9 MB)
```

Cara kerjanya: `gomobile bind` membungkus `android/botlib` (Start/Stop server Go)
menjadi `botlib.aar`; `MainActivity.java` (shell WebView ~40 baris) menjalankan
server lalu memuat dashboard dari `127.0.0.1`. Tidak ada dependensi AndroidX —
APK tetap ringan. Token login tersimpan di folder privat aplikasi.

**Alternatif tanpa APK:** build `GOOS=linux GOARCH=arm64` (lihat
`scripts/build-all.sh`) → jalankan di **Termux**, buka di Chrome; atau buka
dashboard dari PC lalu *Add to Home screen* (PWA).

## 🎨 Icon & avatar (PNG, bisa ditukar)

Semua ikon dan avatar adalah **file PNG** — tidak ada SVG inline. Struktur:

| File (`pkg/bot/web/icons/`) | Dipakai untuk |
|---|---|
| `chief.png` | Logo utama (splash, header, chat, core) |
| `launcher.png` | Ikon launcher APK + favicon |
| `feed·rank·surge·alpha·copy·hold·whale·dev.png` | Avatar 8 agent |

**Cara menukar:** letakkan PNG pengganti (nama sama, bebas ukuran) di
`<DESK_DATA_DIR>/icons/` (tanpa rebuild) atau `pkg/bot/web/icons/` (rebuild),
lalu muat ulang halaman. Bila file tidak ada, UI otomatis memakai PNG bawaan
yang di-embed di binary — tidak pernah kosong. Ikon launcher Android ada di
`android/app/src/main/res/mipmap-*/ic_launcher.png`.

## ⚡ Trading live sungguhan — kapan & di mana menggantinya

**Kapan:** kapan pun Anda siap — punya trade-builder API dan wallet Solana.
Selama konfigurasi belum lengkap, live **tertahan dengan aman** (paper tetap jalan).

**Di mana (hanya 2 tempat, tanpa ubah kode):**

1. **`.env`** (atau environment variable) — isi 4 variabel:
   `LIVE_TRADE_ENDPOINT`, `LIVE_RPC_ENDPOINT`, `LIVE_WALLET_PUBKEY`,
   dan `LIVE_WALLET_KEYFILE` (file keypair Solana CLI) **atau**
   `LIVE_WALLET_SEED_B64`. Cek hasilnya: `./grokbot --doctor` → baris `live:`.
2. **Switch di aplikasi** — geser `PAPER → LIVE` di header (atau chat `live`),
   konfirmasi dialog ⚠, selesai.

**Kode yang menyentuh signing** (hanya bila ingin kustom):
`pkg/bot/executor.go` → fungsi **`SignAndSend`** (decode tx → sign ed25519
lokal → `sendTransaction` ke RPC) dan `loadLiveKey` (pemuatan kunci).
Private key tidak pernah keluar dari proses bot.

## 🔐 Keamanan & pola "tanpa private key"

Tidak ada transaksi Solana yang bisa terjadi tanpa tanda tangan — kunci selalu
ada di **satu** lapisan. Kit ini memakai pola yang disarankan:

1. **Paper mode (default)** — tidak ada kunci sama sekali, simulasi penuh.
2. **Live mode** — `LIVE_TRADE_ENDPOINT` (trade-builder) mengembalikan transaksi
   **belum ditandatangani**; `SignAndSend` di `executor.go` **sudah terimplementasi**:
   signing **lokal** dengan ed25519 (stdlib Go) lalu kirim raw tx ke
   `LIVE_RPC_ENDPOINT`. API pihak ketiga tidak pernah melihat private key.
3. Jangan pernah menaruh private key di konfigurasi yang ikut ter-commit.
   Gunakan **burner wallet** khusus bot dengan dana yang siap hilang.

## 📂 Struktur repo

```
├── cmd/grokbot/            CLI utama
├── pkg/bot/                core: agents, consensus, session, risk, executor,
│   └── web/index.html      server HTTP+SSE, UI night mode (embedded)
├── android/
│   ├── botlib/             wrapper gomobile (Start/Stop/Version)
│   └── app/                shell WebView (Java + Gradle)
├── scripts/                run-dev · build-android · build-all
├── .env.example            template konfigurasi
└── .github/workflows/ci.yml  gofmt + vet + build + smoke test
```

## 🗺 Roadmap

- [x] 8 agent + consensus core + session envelope + TP/SL/trail
- [x] Dashboard night mode (SSE) + chat perintah
- [x] Android kit (gomobile + WebView)
- [ ] GMGN live endpoint mapping per docs terbaru
- [x] `SignAndSend` live — ed25519 lokal (stdlib) + JSON-RPC `sendTransaction`; siap pakai setelah 4 env live diisi
- [ ] Foreground service Android untuk sesi overnight
- [ ] Copy-trade scoring (wallet-score GMGN)

## License

MIT — lihat [LICENSE](LICENSE).
