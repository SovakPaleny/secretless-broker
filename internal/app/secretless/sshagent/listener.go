package sshagent

import (
	"log"
	"net"

	"golang.org/x/crypto/ssh/agent"

	"github.com/conjurinc/secretless/internal/pkg/provider"
	"github.com/conjurinc/secretless/pkg/secretless/config"
)

type Listener struct {
	Config    config.Listener
	Handlers  []config.Handler
	Providers []provider.Provider
	Listener  net.Listener
}

func (l *Listener) Listen() {
	// Serve the first Handler which is attached to this listener
	var selectedHandler *config.Handler
	for _, handler := range l.Handlers {
		listener := handler.Listener
		if listener == "" {
			listener = handler.Name
		}

		if listener == l.Config.Name {
			selectedHandler = &handler
			break
		}
	}

	if selectedHandler == nil {
		log.Fatalf("No ssh-agent handler is available")
	}

	keyring := agent.NewKeyring()

	handler := &Handler{Config: *selectedHandler}
	if err := handler.LoadKeys(keyring); err != nil {
		log.Printf("Failed to load ssh-agent handler keys: ", err)
		return
	}

	for {
		nConn, err := l.Listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection: ", err)
			return
		}

		if err := agent.ServeAgent(keyring, nConn); err != nil {
			log.Printf("Error serving agent : %s", err)
		}
	}
}