# WAGO — WhatsApp API for Go (tanpa Docker)

WAGO adalah **HTTP API ringan** di atas [whatsmeow](https://github.com/tulir/whatsmeow) (klien WhatsApp Web Multi-Device untuk Go). Cocok buat **kirim OTP**, notifikasi transaksi, auto-reply sederhana, sampai integrasi CRM — **tanpa Docker**, **tanpa layanan pihak ketiga**.

> ⚠️ **Catatan**: WAGO memakai protokol **WhatsApp Web** (bukan WhatsApp Cloud API resmi). Gunakan pada akun Anda sendiri dan patuhi Ketentuan Layanan WhatsApp.

---

## Daftar Isi
- [Fitur Utama](#fitur-utama)
- [Arsitektur](#arsitektur)
- [Quick Start](#quick-start)
- [Konfigurasi (ENV)](#konfigurasi-env)
- [API Reference](#api-reference)
  - [`POST /send-text`](#post-send-text)
  - [`POST /send-otp`](#post-send-otp)
  - [`GET /messages`](#get-messages)
  - [`GET /health`](#get-health)
- [Webhook (Push Inbound)](#webhook-push-inbound)
- [Deploy (Linux + systemd)](#deploy-linux--systemd)
- [Tips & Praktik Baik](#tips--praktik-baik)
- [Troubleshooting](#troubleshooting)
- [Lisensi](#lisensi)

---

## Fitur Utama

- Login via **QR Code** → sesi tersimpan di **SQLite** (`session.db`)
- HTTP API siap pakai:
  - `POST /send-text` — kirim teks
  - `POST /send-otp` — kirim OTP dengan template
  - `GET /messages` — baca pesan masuk (**in-memory store**)
  - `GET /health` — health check sederhana
- **Webhook** (opsional) untuk push pesan masuk, dengan **HMAC signature**
- Auth sederhana via header `X-API-Key`
- Satu binary, no Docker. **Jalan di laptop/VM/VPS**

---

## Arsitektur

```mermaid
flowchart LR
  WA[WhatsApp Server] <--> WM[whatsmeow (Go client)]
  WM -->|events pesan| API[WAGO HTTP API]
  API -->|pull| WEB[Web App]
  API -->|push (webhook)| WH[Webhook App]
