WAGO — WhatsApp API for Go (tanpa Docker)

WAGO adalah HTTP API ringan di atas whatsmeow
 (client WhatsApp Web Multi-Device untuk Go). Cocok buat kirim OTP, notifikasi transaksi, auto-reply sederhana, sampai integrasi CRM — tanpa Docker, tanpa layanan pihak ketiga.

⚠️ Catatan: ini memakai protokol WhatsApp Web (bukan WhatsApp Cloud API resmi). Gunakan pada akun Anda sendiri dan patuhi ketentuan layanan WhatsApp.

✨ Fitur Utama

Login via QR Code → sesi tersimpan di SQLite (session.db)

HTTP API:

POST /send-text — kirim teks

POST /send-otp — kirim OTP dengan template

GET /messages — baca pesan masuk (in-memory store)

GET /health — health check

Webhook (opsional) untuk push pesan masuk, dengan HMAC signature

Auth sederhana pakai header X-API-Key

Satu binary, no Docker. Nyala di laptop/VM/VPS.

🧱 Arsitektur Singkat
┌──────────────┐     events/msg     ┌───────────────┐
│  WhatsApp    │◄──────────────────►│  whatsmeow    │
│  Server      │                    │  (Go client)  │
└──────────────┘                    └──────┬────────┘
                                           │
                              ┌────────────▼────────────┐
                              │  WAGO HTTP API (Go)     │
                              │  /send-text /messages   │
                              └──────┬─────────┬────────┘
                                     │         │
                               (push) │   (pull│REST)
                                     ▼         ▼
                               Webhook App    Web App

🚀 Quick Start
0) Prasyarat

Go 1.21+ (disarankan 1.22/1.23)

Toolchain C (untuk github.com/mattn/go-sqlite3)

macOS: xcode-select --install

Ubuntu/Debian: sudo apt-get install -y build-essential

1) Instal dep & build
# dari root project
go mod tidy
# opsi: vendor (build offline)
go mod vendor

# build (tanpa Docker)
go build -mod=vendor -o bin/wago-api ./cmd/wago-api
go build -mod=vendor -o bin/wago-listen ./cmd/wago-listen

2) Pairing (sekali saja)

Pilih salah satu:

# Opsi A: pairing via listener
./bin/wago-listen
# → scan QR dari WhatsApp > Linked devices

# Opsi B: pairing dari API (QR muncul saat start jika belum ada session.db)
API_KEY=supersecret \
PORT=8080 \
DSN='sqlite3://file:session.db?_foreign_keys=on' \
./bin/wago-api


File sesi disimpan di session.db. Jangan commit file ini ke Git.

3) Jalankan API
API_KEY=supersecret \
PORT=8080 \
DSN='sqlite3://file:session.db?_foreign_keys=on' \
./bin/wago-api


Cek:

curl http://localhost:8080/health
# {"ok":true}

🔐 Environment Variables
Var	Wajib	Contoh	Keterangan
API_KEY	Ya	supersecretkey-ubah-ke-random	Kunci untuk header X-API-Key
PORT	Tidak	8080	Port HTTP
DSN	Ya	sqlite3://file:session.db?_foreign_keys=on	DSN SQL untuk whatsmeow (SQLite/Postgres)
WEBHOOK_URL	Ops	https://appkamu.example.com/hook	Endpoint untuk push pesan masuk
WEBHOOK_SECRET	Ops	whsec_xxx	Kunci HMAC untuk menandatangani payload webhook

Contoh .env:

PORT=8080
API_KEY=supersecretkey-ubah
DSN=sqlite3://file:session.db?_foreign_keys=on
# WEBHOOK_URL=https://yourapp/hook
# WEBHOOK_SECRET=whsec_123abc


Program tidak otomatis membaca .env. Pakai export/source, atau tambahkan github.com/joho/godotenv jika ingin auto-load.

🧪 Cara Pakai API
Kirim teks
curl -X POST http://localhost:8080/send-text \
  -H 'X-API-Key: supersecret' \
  -H 'Content-Type: application/json' \
  -d '{"to":"62812XXXXXXX","text":"Halo dari API 🎯"}'
# → {"message_id":"..."}


Format to: E.164 tanpa tanda + (contoh: 62812xxxxxx).

Kirim OTP (dengan template)
curl -X POST http://localhost:8080/send-otp \
  -H 'X-API-Key: supersecret' \
  -H 'Content-Type: application/json' \
  -d '{"to":"62812XXXXXXX","code":"654321","template":"[MyApp] OTP: %s (5 menit)"}'

Baca pesan masuk (pull)
# semua pesan terbaru (max 100 default)
curl -H 'X-API-Key: supersecret' http://localhost:8080/messages

# filter per chat_id
curl -H 'X-API-Key: supersecret' \
  'http://localhost:8080/messages?chat=62812xxxxxx@s.whatsapp.net'

# pagination: ambil setelah id tertentu
curl -H 'X-API-Key: supersecret' \
  'http://localhost:8080/messages?after=10&limit=50'


Respons item:

{
  "id": 11,
  "chat_id": "62812xxxxxx@s.whatsapp.net",
  "sender_id": "62812xxxxxx@s.whatsapp.net",
  "text": "halo",
  "kind": "conversation",
  "timestamp": "2025-09-01T09:20:31Z"
}


Store ini in-memory (hilang saat restart). Simpan sendiri ke DB kalau butuh persist.

🔔 Webhook (push)

Jika WEBHOOK_URL diset, setiap pesan teks masuk akan dikirim ke URL tersebut:

Request

POST {WEBHOOK_URL}
Content-Type: application/json
X-Wago-Signature: {hex(hmac_sha256(body, WEBHOOK_SECRET))}


Body

{
  "chat_id": "62812xxxxxx@s.whatsapp.net",
  "sender_id": "62812xxxxxx@s.whatsapp.net",
  "text": "halo",
  "kind": "conversation",
  "timestamp": "2025-09-01T09:20:31Z"
}


Verifikasi HMAC (Node.js contoh)

import crypto from "node:crypto";

function isValid(req, secret) {
  const sig = req.headers["x-wago-signature"];
  const body = JSON.stringify(req.body);
  const h = crypto.createHmac("sha256", secret).update(body).digest("hex");
  return crypto.timingSafeEqual(Buffer.from(sig), Buffer.from(h));
}

🛠️ Deploy singkat (Linux + systemd)

Buat user & folder

sudo useradd -r -s /usr/sbin/nologin wago || true
sudo mkdir -p /srv/wago/bin /srv/wago/data
sudo chown -R wago:wago /srv/wago


Env

sudo tee /etc/wago.env >/dev/null <<'EOF'
API_KEY=supersecret-ubah
PORT=8080
DSN=sqlite3://file:/srv/wago/data/session.db?_foreign_keys=on
# WEBHOOK_URL=
# WEBHOOK_SECRET=
EOF
sudo chmod 600 /etc/wago.env


Service

# /etc/systemd/system/wago.service
[Unit]
Description=WAGO API (WhatsApp via whatsmeow)
After=network-online.target
Wants=network-online.target

[Service]
User=wago
Group=wago
EnvironmentFile=/etc/wago.env
WorkingDirectory=/srv/wago
ExecStart=/srv/wago/bin/wago-api
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target


Start

sudo systemctl daemon-reload
sudo systemctl enable --now wago.service
sudo journalctl -u wago -f


Pertama kali tanpa session.db, QR akan tercetak di log — scan dari HP (Linked devices), lalu status Connected.

🧰 Tips & Praktik Baik

Satu proses per session.db (jangan jalankan dua instance untuk sesi yang sama).

Jangan commit: session.db, .env, bin/ (set di .gitignore).

API_KEY panjang & acak (openssl rand -hex 32) — rotasi berkala.

Reverse proxy + HTTPS (Caddy/Nginx) sangat disarankan.

Build Linux dari Mac? Lebih mudah build di server. Jika perlu cross-compile CGO:

pakai Zig: CC="zig cc -target x86_64-linux-gnu" / aarch64-linux-gnu saat go build.

🧪 Troubleshooting
Gejala	Penyebab umum	Solusi
401 unauthorized	X-API-Key ≠ API_KEY server	Samakan kunci, restart server
403 API disabled: set API_KEY	Server start tanpa API_KEY	Set env & start ulang
Disconnected	Salah DSN, sesi invalid, 2 proses	Pastikan DSN ke session.db benar, hanya 1 proses, re-pair jika perlu
unknown driver "sqlite3"	CGO/gcc belum ada	Install toolchain C (build-essential / Xcode CLI)
Pesan tidak terkirim	Format to salah / blocked	Pakai E.164 tanpa +, tes ke nomor sendiri
