package container

import (
	"crypto/tls"
	"github.com/sharovik/devbot/internal/client"
	"github.com/sharovik/devbot/internal/config"
	"github.com/sharovik/devbot/internal/log"
	"net/http"
	"time"
)

//Main container object
type Main struct {
	Config      config.Config
	SlackClient client.SlackClientInterface
}

//C container variable
var C Main

//Init initialise container
func (container Main) Init() Main {
	container.Config = config.Init()

	_ = log.Init(log.Config(container.Config))

	netTransport := &http.Transport{
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	slackClient := client.SlackClient{
		Client: &http.Client{
			Timeout:   time.Duration(5) * time.Second,
			Transport: netTransport,
		},
		BaseURL:    container.Config.SlackConfig.BaseURL,
		OAuthToken: container.Config.SlackConfig.OAuthToken,
	}

	container.SlackClient = slackClient

	return container
}