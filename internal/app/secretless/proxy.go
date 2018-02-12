package secretless

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/conjurinc/secretless/internal/app/secretless/http"
	"github.com/conjurinc/secretless/internal/app/secretless/pg"
	"github.com/conjurinc/secretless/internal/app/secretless/ssh"
	"github.com/conjurinc/secretless/internal/app/secretless/sshagent"
	"github.com/conjurinc/secretless/pkg/secretless/config"
)

// Listener is an interface for listening in an abstract way.
type Listener interface {
	Listen()
}

// Proxy is the main struct of Secretless.
type Proxy struct {
	Config config.Config
}

// Listen runs the listen loop for a specific Listener.
func (p *Proxy) Listen(listenerConfig config.Listener, wg sync.WaitGroup) {
	var l net.Listener
	var err error

	if listenerConfig.Address != "" {
		l, err = net.Listen("tcp", listenerConfig.Address)
	} else {
		l, err = net.Listen("unix", listenerConfig.Socket)

		// https://stackoverflow.com/questions/16681944/how-to-reliably-unlink-a-unix-domain-socket-in-go-programming-language
		// Handle common process-killing signals so we can gracefully shut down:
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM)
		go func(c chan os.Signal) {
			// Wait for a SIGINT or SIGKILL:
			sig := <-c
			log.Printf("Caught signal %s: shutting down.", sig)
			// Stop listening (and unlink the socket if unix type):
			l.Close()
			// And we're done:
			os.Exit(0)
		}(sigc)
	}
	if err == nil {
		log.Printf("%s listener '%s' listening at: %s", listenerConfig.Protocol, listenerConfig.Name, l.Addr())

		protocol := listenerConfig.Protocol
		if protocol == "" {
			protocol = listenerConfig.Name
		}

		var listener Listener
		switch protocol {
		case "pg":
			listener = &pg.Listener{Config: listenerConfig, Listener: l, Handlers: p.Config.Handlers}
		case "http":
			listener = &http.Listener{Config: listenerConfig, Listener: l, Handlers: p.Config.Handlers}
		case "ssh":
			listener = &ssh.Listener{Config: listenerConfig, Listener: l, Handlers: p.Config.Handlers}
		case "ssh-agent":
			listener = &sshagent.Listener{Config: listenerConfig, Listener: l, Handlers: p.Config.Handlers}
		default:
			panic(fmt.Sprintf("Unrecognized protocol '%s' on listener '%s'", protocol, listenerConfig.Name))
		}
		go func() {
			defer wg.Done()
			listener.Listen()
		}()
	} else {
		log.Fatal(err)
	}
}

// Run is the main entrypoint to the secretless program.
func (p *Proxy) Run() {
	var wg sync.WaitGroup
	wg.Add(len(p.Config.Listeners))
	for _, config := range p.Config.Listeners {
		p.Listen(config, wg)
	}
	wg.Wait()
}