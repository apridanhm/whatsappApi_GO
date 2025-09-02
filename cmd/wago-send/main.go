package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/apridanhm/whatsappApi_GO/internal/app"
)

func main() {
	dsn := flag.String("dsn", "sqlite3://file:session.db?_foreign_keys=on", "SQL DSN (sqlite3/postgres)")
	to := flag.String("to", "", "Nomor tujuan E164 tanpa + (mis. 62812xxxxxx)")
	text := flag.String("text", "", "Isi pesan")
	flag.Parse()
	if *to == "" || *text == "" {
		panic("wajib --to dan --text")
	}

	container, err := app.NewContainer(*dsn)
	if err != nil {
		panic(err)
	}
	client, err := app.NewAppClient(container)
	if err != nil {
		panic(err)
	}

	if err := client.Start(context.Background(), func(e interface{}) {}); err != nil {
		panic(err)
	}
	defer client.Client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	id, err := client.SendText(ctx, *to, *text)
	if err != nil {
		panic(err)
	}
	fmt.Println("Terkirim, id:", id)
}
