package grpcservice

import (
	"context"
	"net"

	"github.com/dynamicgo/slf4go"

	"github.com/dynamicgo/go-config"
)

type builtinProvider struct {
	slf4go.Logger
	config   config.Config
	listener net.Listener
}

func newBuiltinProvider(config config.Config) (Provider, error) {

	listener, err := net.Listen("tcp", config.Get("laddr").String(":8080"))

	if err != nil {
		return nil, err
	}

	return &builtinProvider{
		Logger:   slf4go.Get("grpservice.default"),
		listener: listener,
	}, nil
}

func (provider *builtinProvider) Listener() net.Listener {
	return provider.listener
}

func (provider *builtinProvider) Connect(ctx context.Context, remote string) (net.Conn, error) {
	provider.DebugF("try connect to %s", remote)
	return net.Dial("tcp", remote)
}
