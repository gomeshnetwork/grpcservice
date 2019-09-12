package main

import (
	"github.com/dynamicgo/slf4go"

	"github.com/dynamicgo/go-config"
	"github.com/gomeshnetwork/gomesh"
	"github.com/gomeshnetwork/gomesh/app"
	"github.com/gomeshnetwork/grpcservice"
	"google.golang.org/grpc"
)

var logger = slf4go.Get("test")

type grpcClient struct {
}

type grpcServer struct {
	Client grpcservice.Client `inject:"test.grpc"`
}

func (s *grpcServer) GrpcHandler(server *grpc.Server) error {
	logger.DebugF("bind grpc server %p", s.Client)
	return nil
}

func main() {
	grpcService := grpcservice.New("test.grpc")

	grpcService.Local("test.grpc.server", func(config config.Config) (grpcservice.Service, error) {
		return &grpcServer{}, nil
	})

	grpcService.Remote("test.grpc.client", func(conn *grpc.ClientConn) (gomesh.Service, error) {
		return &grpcClient{}, nil
	})

	app.Run("test")
}
