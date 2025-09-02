package app

import (
	"context"
	"fmt"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var debugLogUnknown = false

func DefaultEventHandler(ac *AppClient) func(evt interface{}) {
	startedAt := time.Now()

	getText := func(m *waProto.Message) (string, bool, string) {
		if t := m.GetConversation(); t != "" {
			return t, true, "conversation"
		}
		if x := m.GetExtendedTextMessage(); x != nil && x.GetText() != "" {
			return x.GetText(), true, "extended_text"
		}
		if im := m.GetImageMessage(); im != nil {
			if im.GetCaption() != "" {
				return im.GetCaption(), true, "image_caption"
			}
			return "[image]", false, "image"
		}
		if vm := m.GetVideoMessage(); vm != nil {
			if vm.GetCaption() != "" {
				return vm.GetCaption(), true, "video_caption"
			}
			return "[video]", false, "video"
		}
		if rm := m.GetReactionMessage(); rm != nil {
			return "reaction: " + rm.GetText(), true, "reaction"
		}
		if m.GetStickerMessage() != nil {
			return "[sticker]", false, "sticker"
		}
		if m.GetAudioMessage() != nil {
			return "[audio]", false, "audio"
		}
		if dm := m.GetDocumentMessage(); dm != nil {
			if dm.GetCaption() != "" {
				return dm.GetCaption(), true, "doc_caption"
			}
			return "[document]", false, "document"
		}
		if bm := m.GetButtonsResponseMessage(); bm != nil && bm.GetSelectedDisplayText() != "" {
			return bm.GetSelectedDisplayText(), true, "buttons_response"
		}
		if lm := m.GetListResponseMessage(); lm != nil && lm.GetTitle() != "" {
			return lm.GetTitle(), true, "list_response"
		}
		return "", false, "unknown"
	}

	return func(evt interface{}) {
		switch e := evt.(type) {
		case *events.PairSuccess:
			fmt.Println("✅ Pairing sukses, logged in sebagai:", ac.Client.Store.ID)

		case *events.Connected:
			fmt.Println("🔌 Connected")

		case *events.Disconnected:
			fmt.Println("🔌 Disconnected")

		case *events.Message:
			// Filter initial sync & pesan dari diri sendiri (opsional)
			if e.Info.Timestamp.Before(startedAt.Add(-2 * time.Second)) {
				return
			}
			if e.Info.IsFromMe {
				return
			}

			text, isText, kind := getText(e.Message)
			from := e.Info.Sender.String()

			if text == "" {
				if debugLogUnknown {
					fmt.Printf("📩 %s: [unknown kind=%s]\n", from, kind)
				}
				return
			}

			fmt.Printf("📩 %s: %s\n", from, text)

			if isText {
				switch text {
				case "!ping":
					_, _ = ac.Client.SendMessage(context.Background(), e.Info.Chat, &waProto.Message{
						Conversation: proto.String("pong"),
					})
				case "!id":
					reply := fmt.Sprintf("Chat: %s\nSender: %s", e.Info.Chat.String(), from)
					_, _ = ac.Client.SendMessage(context.Background(), e.Info.Chat, &waProto.Message{
						Conversation: proto.String(reply),
					})
				}
			}
		}
	}
}
