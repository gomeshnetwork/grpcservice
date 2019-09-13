package grpcservice

import (
	"context"
	"net"

	"github.com/dynamicgo/slf4go"
	"github.com/dynamicgo/xerrors"

	"github.com/dynamicgo/go-config"
)

type builtinProvider struct {
	slf4go.Logger
	config   config.Config
	listener net.Listener
}

func newBuiltinProvider(config config.Config) (Provider, error) {
	laddr := config.Get("laddr").String(":8080")
	listener, err := net.Listen("tcp", laddr)

	if err != nil {
		return nil, xerrors.Wrapf(err, "listen on %s error", laddr)
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
