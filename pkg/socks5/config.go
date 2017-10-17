package socks5

import (
	"fmt"

	"gitlab.com/tabjy/groundhog/pkg/util"
)

var (
	ErrInvalidListenHost = fmt.Errorf("invalid listen host")
	ErrInvalidListenPort = fmt.Errorf("invalid listen port")
)

type Config struct {
	Host string
	Port int
}

func GenerateDefaultConfig() (*Config, error) {
	port, err := util.GetFreePort()
	if err != nil {
		return nil, err
	}

	return &Config{
		Host: "127.0.0.1",
		Port: port,
	}, nil
}
