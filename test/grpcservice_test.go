package test

import (
	context "context"
	"testing"

	"github.com/dynamicgo/go-config"
	"github.com/dynamicgo/slf4go"
	"github.com/gomeshnetwork/gomesh"
	"github.com/gomeshnetwork/grpcservice"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

//go:generate protoc --proto_path=./ --go_out=plugins=grpc,paths=source_relative:. grpcservice_test.proto

var logger = slf4go.Get("test")

type TestServerImpl struct {
}

func (server *TestServerImpl) Hello(ctx context.Context, in *TestRequest) (*TestResponse, error) {
	logger.DebugF("call .....")
	return &TestResponse{
		Message: in.Message,
	}, nil
}

func (server *TestServerImpl) GrpcHandler(s *grpc.Server) error {
	logger.DebugF("bind server .....")
	RegisterTestServer(s, server)
	return nil
}

func TestEcho(t *testing.T) {
	mesh := gomesh.New()
	grpcService := grpcservice.New(mesh)

	grpcService.Local("grpc.local", func(config config.Config) (grpcservice.Service, error) {
		return &TestServerImpl{}, nil
	})

	grpcService.Remote("grpc.remote", func(conn *grpc.ClientConn) (gomesh.Service, error) {
		return NewTestClient(conn), nil
	})

	err := mesh.Start()

	require.NoError(t, err)

	var client TestClient

	mesh.ServiceByName("grpc.remote", &client)

	require.NotNil(t, client)

	resp, err := client.Hello(context.Background(), &TestRequest{
		Message: "hello",
	})

	require.NoError(t, err)

	require.Equal(t, resp.Message, "hello")
}
