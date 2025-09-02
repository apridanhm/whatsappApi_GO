package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waTypes "go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// AppClient membungkus whatsmeow.Client dan util
type AppClient struct {
	Client      *whatsmeow.Client
	DeviceStore *store.Device
}

func NewAppClient(container *sqlstore.Container) (*AppClient, error) {
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}
	cli := whatsmeow.NewClient(deviceStore, waLog.Noop)
	ac := &AppClient{Client: cli, DeviceStore: deviceStore}
	return ac, nil
}

func (ac *AppClient) Start(ctx context.Context, onEvent func(evt interface{})) error {
	ac.Client.AddEventHandler(func(e interface{}) { onEvent(e) })

	if ac.Client.Store.ID == nil {
		qrChan, _ := ac.Client.GetQRChannel(ctx)
		if err := ac.Client.Connect(); err != nil {
			return err
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan QR berikut dari WhatsApp di HP kamu:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				fmt.Println("-- Jika sudah di-scan, tunggu sampai status READY --")
			} else {
				fmt.Println("Auth event:", evt.Event)
			}
		}
	} else {
		if err := ac.Client.Connect(); err != nil {
			return err
		}
	}
	return nil
}

func (ac *AppClient) GracefulWait() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	ac.Client.Disconnect()
}

// SendText mengirim pesan teks sederhana
func (ac *AppClient) SendText(ctx context.Context, phoneE164, text string) (waTypes.MessageID, error) {
	jid := waTypes.NewJID(phoneE164, waTypes.DefaultUserServer)
	msg := &waProto.Message{Conversation: proto.String(text)}
	resp, err := ac.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}
