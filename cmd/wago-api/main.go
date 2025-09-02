package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/apridanhm/whatsappApi_GO/internal/app"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	waTypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

/**************** In-memory store pesan masuk ****************/

type InboundPayload struct {
	ChatID    string    `json:"chat_id"`
	SenderID  string    `json:"sender_id"`
	Text      string    `json:"text"`
	Kind      string    `json:"kind"`
	Timestamp time.Time `json:"timestamp"`
}

type StoredMsg struct {
	ID int64 `json:"id"`
	InboundPayload
}

type MsgStore struct {
	mu   sync.RWMutex
	next int64
	buf  []StoredMsg
	cap  int
}

func NewMsgStore(capacity int) *MsgStore {
	return &MsgStore{cap: capacity, buf: make([]StoredMsg, 0, capacity), next: 1}
}
func (s *MsgStore) Add(p InboundPayload) StoredMsg {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := StoredMsg{ID: s.next, InboundPayload: p}
	s.next++
	if len(s.buf) == s.cap {
		copy(s.buf[0:], s.buf[1:])
		s.buf[len(s.buf)-1] = msg
	} else {
		s.buf = append(s.buf, msg)
	}
	return msg
}
func (s *MsgStore) After(afterID int64, limit int, chat string) []StoredMsg {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	res := make([]StoredMsg, 0, limit)
	for i := len(s.buf) - 1; i >= 0 && len(res) < limit; i-- {
		m := s.buf[i]
		if m.ID <= afterID {
			break
		}
		if chat != "" && m.ChatID != chat {
			continue
		}
		res = append(res, m)
	}
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}
	return res
}

/**************** HTTP server ****************/

type Server struct {
	AC            *app.AppClient
	APIKey        string
	WebhookURL    string
	WebhookSecret string
	Store         *MsgStore
}

type sendTextReq struct {
	To   string `json:"to"`   // E164 tanpa '+'
	Text string `json:"text"` // isi pesan
}
type sendOTPReq struct {
	To       string `json:"to"`
	Code     string `json:"code"`
	Template string `json:"template"` // optional, default: "Kode verifikasi kamu: %s"
}

func main() {
	port := getenv("PORT", "8080")
	apiKey := getenv("API_KEY", "")
	dsn := getenv("DSN", "sqlite3://file:session.db?_foreign_keys=on")
	webhookURL := getenv("WEBHOOK_URL", "")
	webhookSecret := getenv("WEBHOOK_SECRET", "")

	if apiKey == "" {
		log.Println("[WARN] API_KEY kosong. Endpoint yang butuh auth akan ditolak (403). Set API_KEY untuk mengaktifkan.")
	}

	container, err := app.NewContainer(dsn)
	if err != nil {
		log.Fatal(err)
	}
	ac, err := app.NewAppClient(container)
	if err != nil {
		log.Fatal(err)
	}

	// Simpan hingga 1000 pesan masuk terbaru di memori
	store := NewMsgStore(1000)

	// Handler default + simpan ke store + forward ke webhook (jika diset)
	defaultHandler := app.DefaultEventHandler(ac)
	ac.Client.AddEventHandler(func(e interface{}) {
		defaultHandler(e)
		if msgEvt, ok := e.(*events.Message); ok {
			if p := toInbound(msgEvt); p != nil {
				store.Add(*p) // simpan pesan masuk
				if webhookURL != "" {
					go forwardToWebhook(webhookURL, webhookSecret, *p)
				}
			}
		}
	})

	ctx := context.Background()
	if err := ac.Start(ctx, func(e interface{}) {}); err != nil {
		log.Fatal(err)
	}

	s := &Server{
		AC:            ac,
		APIKey:        apiKey,
		WebhookURL:    webhookURL,
		WebhookSecret: webhookSecret,
		Store:         store,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/send-text", s.auth(s.sendText))
	mux.HandleFunc("/send-otp", s.auth(s.sendOTP))
	mux.HandleFunc("/messages", s.auth(s.listMessages)) // <— endpoint baru

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      logRequest(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("HTTP API listening on :%s\n", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShutdown)
	ac.Client.Disconnect()
}

/************* Handlers *************/

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.APIKey == "" {
			http.Error(w, "API disabled: set API_KEY", http.StatusForbidden)
			return
		}
		if r.Header.Get("X-API-Key") != s.APIKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) sendText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req sendTextReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.To == "" || req.Text == "" {
		http.Error(w, "to & text wajib diisi", http.StatusBadRequest)
		return
	}
	if !allDigits(req.To) {
		http.Error(w, "format to salah (E164 tanpa +)", http.StatusBadRequest)
		return
	}
	jid := waTypes.NewJID(req.To, waTypes.DefaultUserServer)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := s.AC.Client.SendMessage(ctx, jid, &waProto.Message{Conversation: proto.String(req.Text)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, 200, map[string]any{"message_id": resp.ID})
}

func (s *Server) sendOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req sendOTPReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.To == "" || req.Code == "" {
		http.Error(w, "to & code wajib diisi", http.StatusBadRequest)
		return
	}
	tpl := strings.TrimSpace(req.Template)
	if tpl == "" {
		tpl = "Kode verifikasi kamu: %s"
	}
	text := fmt.Sprintf(tpl, req.Code)
	jid := waTypes.NewJID(req.To, waTypes.DefaultUserServer)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := s.AC.Client.SendMessage(ctx, jid, &waProto.Message{Conversation: proto.String(text)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, 200, map[string]any{"message_id": resp.ID})
}

// GET /messages?after=<id>&limit=<n>&chat=<jid>
func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	afterStr := r.URL.Query().Get("after")
	limitStr := r.URL.Query().Get("limit")
	chat := r.URL.Query().Get("chat")

	var after int64
	if afterStr != "" {
		if v, err := strconv.ParseInt(afterStr, 10, 64); err == nil && v >= 0 {
			after = v
		}
	}
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}
	items := s.Store.After(after, limit, chat)
	writeJSON(w, 200, map[string]any{"items": items})
}

/************* Utils *************/

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		type srw struct{ http.ResponseWriter; code int }
		rec := &srw{ResponseWriter: w, code: 200}
		rec.ResponseWriter = struct {
			http.ResponseWriter
		}{rec.ResponseWriter}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) >= 7
}
func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// Konversi events.Message -> payload inbound sederhana
func toInbound(e *events.Message) *InboundPayload {
	get := func() (string, string) {
		if t := e.Message.GetConversation(); t != "" {
			return t, "conversation"
		}
		if x := e.Message.GetExtendedTextMessage(); x != nil && x.GetText() != "" {
			return x.GetText(), "extended_text"
		}
		if im := e.Message.GetImageMessage(); im != nil {
			if im.GetCaption() != "" {
				return im.GetCaption(), "image_caption"
			}
			return "[image]", "image"
		}
		if vm := e.Message.GetVideoMessage(); vm != nil {
			if vm.GetCaption() != "" {
				return vm.GetCaption(), "video_caption"
			}
			return "[video]", "video"
		}
		if rm := e.Message.GetReactionMessage(); rm != nil {
			return "reaction: " + rm.GetText(), "reaction"
		}
		if e.Message.GetStickerMessage() != nil {
			return "[sticker]", "sticker"
		}
		if e.Message.GetAudioMessage() != nil {
			return "[audio]", "audio"
		}
		if dm := e.Message.GetDocumentMessage(); dm != nil {
			if dm.GetCaption() != "" {
				return dm.GetCaption(), "doc_caption"
			}
			return "[document]", "document"
		}
		return "", "unknown"
	}
	text, kind := get()
	if text == "" {
		return nil
	}
	return &InboundPayload{
		ChatID:    e.Info.Chat.String(),
		SenderID:  e.Info.Sender.String(),
		Text:      text,
		Kind:      kind,
		Timestamp: e.Info.Timestamp,
	}
}

func forwardToWebhook(url, secret string, p InboundPayload) {
	body, _ := json.Marshal(p)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		h := hmac.New(sha256.New, []byte(secret))
		h.Write(body)
		sig := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Wago-Signature", sig)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("[webhook] err:", err)
		return
	}
	_ = resp.Body.Close()
}
