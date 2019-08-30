package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	config_v1 "github.com/cyberark/secretless-broker/pkg/secretless/config/v1"
)

func v1DbExample() *config_v1.Config {
	return &config_v1.Config{
		Listeners: []config_v1.Listener{
			{
				Address:  "0.0.0.0:2345",
				Name:     "test-db-listener",
				Protocol: "pg",
			},
		},
		Handlers: []config_v1.Handler{
			{
				Name:         "test-db-handler",
				ListenerName: "test-db-listener",
				Credentials: []config_v1.StoredSecret{
					{
						Name:     "TestSecret1",
						Provider: "conjur",
						ID:       "some-id-1",
					},
					{
						Name:     "TestSecret2",
						Provider: "literal",
						ID:       "some-id-2",
					},
				},
			},
		},
	}
}

func v1HttpExample() *config_v1.Config {
	return &config_v1.Config{
		Listeners: []config_v1.Listener{
			{
				Address:  "0.0.0.0:2345",
				Name:     "test-http-listener",
				Protocol: "http",
			},
		},
		Handlers: []config_v1.Handler{
			{
				Name:         "test-http-handler",
				Type:         "http/aws",
				ListenerName: "test-http-listener",
				Match:        []string{"^http://aws*", "amzn.com"},
				Credentials: []config_v1.StoredSecret{
					{
						Name:     "TestSecret1",
						Provider: "conjur",
						ID:       "some-id-1",
					},
					{
						Name:     "TestSecret2",
						Provider: "literal",
						ID:       "some-id-2",
					},
				},
			},
		},
	}
}

func TestV1HttpHandlerConversion(t *testing.T) {
	t.Run("ConnectorConfig field maps correctly", func(t *testing.T) {
		v1Cfg := v1HttpExample()
		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, "test-http-handler", v2Cfg.Services[0].Name)
		assert.Equal(t,
			`authenticateURLsMatching:
- ^http://aws*
- amzn.com
`, string(v2Cfg.Services[0].ConnectorConfig))
	})

	t.Run("Connector field maps correctly", func(t *testing.T) {
		v1Cfg := v1HttpExample()
		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, "test-http-handler", v2Cfg.Services[0].Name)
		assert.Equal(t, "aws", v2Cfg.Services[0].Connector)
	})

	t.Run("Separate v2 Service created for every Handler associated to a Listener", func(t *testing.T) {
		v1Cfg := v1HttpExample()
		otherHandler := v1Cfg.Handlers[0]
		otherHandler.Name = "test-http-handler-other"
		otherHandler.Credentials = nil
		otherHandler.Match = []string{"not-amzn.com"}
		v1Cfg.Handlers = append(v1Cfg.Handlers, otherHandler)

		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		// Service count
		assert.Equal(t, 2, len(v2Cfg.Services))

		// Name
		assert.Equal(t, "test-http-handler", v2Cfg.Services[0].Name)
		assert.Equal(t, "test-http-handler-other", v2Cfg.Services[1].Name)

		// ListenOn
		assert.Equal(t, "tcp://0.0.0.0:2345", v2Cfg.Services[0].ListenOn)
		assert.Equal(t, "tcp://0.0.0.0:2345", v2Cfg.Services[1].ListenOn)

		// Credentials
		assert.Equal(t, []*Credential{
			{
				Name: "TestSecret1",
				From: "conjur",
				Get:  "some-id-1",
			},
			{
				Name: "TestSecret2",
				From: "literal",
				Get:  "some-id-2",
			},
		}, v2Cfg.Services[0].Credentials)
		assert.Equal(t, []*Credential{}, v2Cfg.Services[1].Credentials)

		// ConnectorConfig
		assert.Equal(t,
			`authenticateURLsMatching:
- ^http://aws*
- amzn.com
`, string(v2Cfg.Services[0].ConnectorConfig))
		assert.Equal(t,
			`authenticateURLsMatching:
- not-amzn.com
`, string(v2Cfg.Services[1].ConnectorConfig))
	})
}

func TestV1ValidationConversion(t *testing.T) {
	t.Run("V1 Config validation fails and reports no handler or listener errors", func(t *testing.T) {
		v1Cfg := v1HttpExample()
		v1Cfg.Handlers = []config_v1.Handler{}
		v1Cfg.Listeners = []config_v1.Listener{}
		_, err := newV2Config(v1Cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Listeners: cannot be blank")
		assert.Contains(t, err.Error(), "Handlers: cannot be blank")
	})

	t.Run("V1 Config validation fails and reports un-associated handler or listener errors", func(t *testing.T) {
		v1Cfg := v1HttpExample()
		v1Cfg.Handlers[0].ListenerName = "xyz"
		_, err := newV2Config(v1Cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Listeners: (0: has no associated handler.)")
		assert.Contains(t, err.Error(), "Handlers: (0: has no associated listener.)")
	})
}
func TestV1AddressSocketConversion(t *testing.T) {

	t.Run("Address maps to TCP listenOn", func(t *testing.T) {
		v1Cfg := v1DbExample()
		v2, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, "tcp://0.0.0.0:2345", v2.Services[0].ListenOn)
	})

	t.Run("Socket maps to Unix listenOn", func(t *testing.T) {
		v1Cfg := v1DbExample()
		v1Cfg.Listeners[0].Socket = "/some/socket/path"
		v1Cfg.Listeners[0].Address = ""
		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, "unix:///some/socket/path", v2Cfg.Services[0].ListenOn)
	})

	t.Run("Empty Socket and Address returns error", func(t *testing.T) {
		v1Cfg := v1DbExample()
		v1Cfg.Listeners[0].Socket = ""
		v1Cfg.Listeners[0].Address = ""
		_, err := newV2Config(v1Cfg)
		assert.Error(t, err)
	})

	t.Run("Both Socket and Address returns error", func(t *testing.T) {
		v1Cfg := v1DbExample()
		v1Cfg.Listeners[0].Socket = "0.0.0.0:5432"
		v1Cfg.Listeners[0].Address = "/some/socket/path"
		_, err := newV2Config(v1Cfg)
		assert.Error(t, err)
	})
}

func TestV1StoredSecretConversion(t *testing.T) {
	t.Run("Handler Credentials map to Service Credentials", func(t *testing.T) {
		v1Cfg := v1DbExample()
		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, []*Credential{
			{
				Name: "TestSecret1",
				From: "conjur",
				Get:  "some-id-1",
			},
			{
				Name: "TestSecret2",
				From: "literal",
				Get:  "some-id-2",
			},
		}, v2Cfg.Services[0].Credentials)
	})
}

func TestV1HandlersConversion(t *testing.T) {
	t.Run("V2 Service assumes the name of the first Handler matching Listener", func(t *testing.T) {
		v1Cfg := v1HttpExample()
		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, "test-http-handler", v2Cfg.Services[0].Name)
	})

	t.Run("V2 Service assumes first Handler matching Listener", func(t *testing.T) {
		v1Cfg := v1DbExample()
		otherHandler := v1Cfg.Handlers[0]
		otherHandler.Name = "test-db-handler-other"
		otherHandler.Credentials = nil
		v1Cfg.Handlers = append(v1Cfg.Handlers, otherHandler)

		v2Cfg, err := newV2Config(v1Cfg)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, []*Credential{
			{
				Name: "TestSecret1",
				From: "conjur",
				Get:  "some-id-1",
			},
			{
				Name: "TestSecret2",
				From: "literal",
				Get:  "some-id-2",
			},
		}, v2Cfg.Services[0].Credentials)

	})
}