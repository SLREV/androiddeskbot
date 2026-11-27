# 🚀 Panduan Push & Pull dari Terminal / CMD

> ZIP ini **sudah termasuk repo git** (riwayat commit lengkap) — jadi push cukup
> 2 perintah. Bila folder `.git` tidak ada (mis. di-upload lewat web), pakai
> "Cara B — dari nol" di bawah.

## Siapkan sekali saja

1. **Install Git** — Windows: unduh di https://git-scm.com/download/win
   (Next–Next–Finish), atau `winget install Git.Git`. Linux/Mac biasanya sudah ada.
2. **Buat repo kosong** di https://github.com/new — beri nama (mis.
   `grokbot-android-kit`), **jangan** centang README/.gitignore/license
   (biar push pertama tidak bentrok).
3. **Token akses** (untuk login): GitHub → *Settings → Developer settings →
   Personal access tokens → Tokens (classic) → Generate new token* → centang
   **`repo`** → salin tokennya.
   *(Di Windows modern cukup login browser via jendela Git Credential Manager —
   token hanya cadangan bila diminta manual.)*

## PUSH (Cara A — ZIP ini sudah ber-`.git`) ⭐

Buka **CMD** (Windows) atau Terminal (Linux/Mac):

```cmd
cd jalan\ke\grokbot-android-kit
git remote add origin https://github.com/USERNAME/REPO-ANDA.git
git push -u origin main
```

Saat diminta kredensial:
- **Username** = nama akun GitHub Anda
- **Password** = **PASTE TOKEN** tadi (bukan password login GitHub!)

Selesai — kode Anda sudah di GitHub. 🎉

## PUSH (Cara B — dari nol, bila `.git` tidak ada)

```cmd
cd jalan\ke\grokbot-android-kit
git init -b main
git add .
git commit -m "Grok Bot Night Desk v1.0.0"
git remote add origin https://github.com/USERNAME/REPO-ANDA.git
git push -u origin main
```

## PULL (ambil update terbaru dari GitHub)

```cmd
cd jalan\ke\grokbot-android-kit
git pull origin main
```

## Perintah sering dipakai

| Perintah | Fungsi |
|---|---|
| `git status` | Lihat perubahan yang belum di-commit |
| `git add . && git commit -m "pesan"` | Simpan perubahan lokal |
| `git push` | Kirim commit baru (setelah push pertama) |
| `git pull` | Tarik commit terbaru |
| `git log --oneline` | Lihat riwayat commit |

## Setelah push — APK terbangun otomatis

Buka tab **Actions** di repo → workflow **build-apk** berjalan otomatis
(~5–8 menit) → klik run terbaru → **Artifacts** → unduh **`grokbot-debug-apk`**
(berisi `app-debug.apk`, siap install di HP). Setiap `git push` = APK baru.

## Troubleshooting singkat

| Gejala | Solusi |
|---|---|
| `remote origin already exists` | `git remote set-url origin URL-BARU` |
| `rejected (non-fast-forward)` | Repo dibuat dengan README → `git pull --rebase origin main` lalu `git push` |
| `403` / auth gagal | Pakai **token** sebagai password (lihat langkah 3) |
| `gomobile: missing golang.org/x/mobile dependency` | Sudah teratasi di commit terbaru (go.mod memuat `tool` gomobile+gobind) — pastikan pakai kode terbaru; lokal: `go install tool` |
| Salah ketik file ter-commit | `git rm --cached nama-file` lalu commit ulang |

## Catatan keamanan

- **Jangan** pernah commit file `.env` yang berisi kunci rahasia — pakai
  `.env.example` sebagai template.
- Gunakan **burner wallet** untuk trading live.
