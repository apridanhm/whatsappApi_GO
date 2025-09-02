package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/apridanhm/whatsappApi_GO/internal/app"
)

func main() {
	dsn := flag.String("dsn", "sqlite3://file:session.db?_foreign_keys=on", "SQL DSN (sqlite3/postgres)")
	flag.Parse()

	container, err := app.NewContainer(*dsn)
	if err != nil {
		panic(err)
	}
	client, err := app.NewAppClient(container)
	if err != nil {
		panic(err)
	}

	fmt.Println("Menjalankan listener…")
	if err := client.Start(context.Background(), app.DefaultEventHandler(client)); err != nil {
		panic(err)
	}
	client.GracefulWait()
}
