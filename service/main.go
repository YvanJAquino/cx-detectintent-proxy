// Created with Strapit
package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cx "cloud.google.com/go/dialogflow/cx/apiv3"
	"cloud.google.com/go/dialogflow/cx/apiv3/cxpb"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	PORT = os.Getenv("PORT")
)

func main() {

	notifications := make(chan os.Signal, 1)
	signal.Notify(notifications, syscall.SIGINT, syscall.SIGTERM)

	parent := context.Background()

	sessions, err := cx.NewSessionsClient(parent)
	if err != nil {
		log.Fatal(err)
	}
	handlers := &SessionsHandlers{
		SessionsClient: sessions,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/detect-intent", handlers.HandleRequest)
	server := &http.Server{
		Addr:        ":" + PORT,
		Handler:     mux,
		BaseContext: func(l net.Listener) context.Context { return parent },
	}

	go func() {
		log.Fatal(server.ListenAndServe())
	}()
	<-notifications
	shutCtx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	log.Fatal(server.Shutdown(shutCtx))
}

func requestUnmarshaler(r io.ReadCloser) (*cxpb.DetectIntentRequest, error) {
	var dir cxpb.DetectIntentRequest
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pbUnmarshaler := &protojson.UnmarshalOptions{
		AllowPartial:   true,
		DiscardUnknown: true,
	}
	err = pbUnmarshaler.Unmarshal(b, &dir)
	if err != nil {
		return nil, err
	}
	r.Close()
	return &dir, nil
}

func responseMarshaler(w io.Writer, res *cxpb.DetectIntentResponse) error {
	m := protojson.MarshalOptions{Indent: "\t"}
	b, err := m.Marshal(res)
	if err != nil {
		log.Println(err)
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(b))
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

type SessionsHandlers struct {
	*cx.SessionsClient
}

func (h SessionsHandlers) HandleRequest(w http.ResponseWriter, r *http.Request) {
	dir, err := requestUnmarshaler(r.Body)
	if err != nil {
		log.Println(err)
		return
	}
	diresp, err := h.SessionsClient.DetectIntent(r.Context(), dir)
	if err != nil {
		log.Println(err)
		return
	}
	err = responseMarshaler(w, diresp)
	if err != nil {
		log.Println(err)
		return
	}

}
