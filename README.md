WhatsappApi_GO — WhatsApp API for Go

WhatsappApi_GO adalah HTTP API ringan di atas whatsmeow
 (client WhatsApp Web Multi-Device untuk Go). Cocok buat kirim OTP, notifikasi transaksi, auto-reply sederhana, sampai integrasi CRM — tanpa Docker, tanpa layanan pihak ketiga.

⚠️ Catatan: ini memakai protokol WhatsApp Web (bukan WhatsApp Cloud API resmi). Gunakan pada akun Anda sendiri dan patuhi ketentuan layanan WhatsApp.

Fitur Utama

Login via QR Code → sesi tersimpan di SQLite (session.db)

HTTP API:

POST /send-text — kirim teks

POST /send-otp — kirim OTP dengan template

GET /messages — baca pesan masuk (in-memory store)

GET /health — health check

Webhook (opsional) untuk push pesan masuk, dengan HMAC signature

Auth sederhana pakai header X-API-Key

Satu binary, no Docker. Nyala di laptop/VM/VPS.



ARSITEKTUR SINGKAT

<img width="578" height="357" alt="image" src="https://github.com/user-attachments/assets/1824f931-13c5-4754-afdb-d02fd2328f02" />


Cara Start

# dari root project
$ go mod tidy
$ go mod vendor
$ go build -mod=vendor -o bin/wago-api ./cmd/wago-api
$ go build -mod=vendor -o bin/wago-listen ./cmd/wago-listen


Pairing (sekali saja)

Pilih salah satu:

# Opsi A: pairing via listener
./bin/wago-listen
# → scan QR dari WhatsApp > Linked devices

# Opsi B: pairing dari API (QR muncul saat start jika belum ada session.db)
API_KEY=supersecret \
PORT=8080 \
DSN='sqlite3://file:session.db?_foreign_keys=on' \
./bin/wago-api



Jalankan API
API_KEY=supersecret \
PORT=8080 \
DSN='sqlite3://file:session.db?_foreign_keys=on' \
./bin/wago-api


Cek:
curl http://localhost:8080/health

Contoh .env:
PORT=8080
API_KEY=supersecretkey-ubah
DSN=sqlite3://file:session.db?_foreign_keys=on
# WEBHOOK_URL=https://yourapp/hook
# WEBHOOK_SECRET=whsec_123abc


Kirim teks
curl -X POST http://localhost:8080/send-text \
  -H 'X-API-Key: supersecret' \
  -H 'Content-Type: application/json' \
  -d '{"to":"62812XXXXXXX","text":"Halo dari API 🎯"}'

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

Deploy singkat (Linux + systemd)

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





  

  

  




