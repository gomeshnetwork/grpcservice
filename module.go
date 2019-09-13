package grpcservice

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dynamicgo/go-config"
	"github.com/dynamicgo/slf4go"
	"github.com/dynamicgo/xerrors"
	"github.com/gomeshnetwork/gomesh"
	"github.com/gomeshnetwork/localservice"
	"google.golang.org/grpc"
)

// Provider .
type Provider interface {
	gomesh.Service
	Listener() net.Listener
	Connect(ctx context.Context, remote string) (net.Conn, error)
}

// Service .
type Service interface {
	gomesh.Service
	GrpcHandler(server *grpc.Server) error
}

// CreatorF .
type CreatorF func(config config.Config) (Service, error)

// ConnectorF .
type ConnectorF func(conn *grpc.ClientConn) (gomesh.Service, error)

// GrpcService .
type GrpcService interface {
	Client
	Local(name string, creator CreatorF)
	Remote(name string, connector ConnectorF)
}

// Client .
type Client interface {
	Dial(ctx context.Context, url string, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error)
}

type grpcServiceImpl struct {
	slf4go.Logger
	provider string
	local    map[string]CreatorF
	remote   map[string]ConnectorF
	server   *grpc.Server
	servces  []Service
	builder  gomesh.ModuleBuilder
	mesh     gomesh.Mesh
}

// Option .
type Option func(impl *grpcServiceImpl)

// WithProvider .
func WithProvider(name string) Option {
	return func(impl *grpcServiceImpl) {
		impl.provider = name
	}
}

// New create new gomesh grpcservice module instance
func New(mesh gomesh.Mesh, options ...Option) GrpcService {
	impl := &grpcServiceImpl{
		Logger: slf4go.Get("gomesh.grpc"),
		local:  make(map[string]CreatorF),
		remote: make(map[string]ConnectorF),
		mesh:   mesh,
	}

	for _, option := range options {
		option(impl)
	}

	if impl.provider == "" {
		localService := localservice.New(mesh)

		impl.provider = "gomesh.module.grpc.provider.default"

		localService.Register(impl.provider, func(config config.Config) (gomesh.Service, error) {
			return newBuiltinProvider(config)
		})
	}

	impl.builder = mesh.Module(impl)

	return impl
}

func (module *grpcServiceImpl) Config(config config.Config) {
	module.server = grpc.NewServer()
}

func (module *grpcServiceImpl) Start() error {

	go func() {
		provider := module.getProvider()

		for {
			if err := module.server.Serve(provider.Listener()); err != nil {
				module.ErrorF("grpc server err: %s", err)
			}
		}

	}()

	return nil
}

func (module *grpcServiceImpl) Name() string {
	return "gomesh.module.grpc"
}

func (module *grpcServiceImpl) BeginCreateService() error {
	return nil
}

func (module *grpcServiceImpl) CreateService(name string, config config.Config) (gomesh.Service, error) {
	f, ok := module.local[name]

	if ok {
		return f(config)
	}

	f2, ok := module.remote[name]

	if ok {
		remote := config.Get("remote").String("127.0.0.1:8080")

		conn, err := module.Dial(context.Background(), remote, grpc.WithInsecure())

		if err != nil {
			return nil, err
		}

		return f2(conn)
	}

	return nil, xerrors.Wrapf(gomesh.ErrNotFound, "service %s not found", name)
}

func (module *grpcServiceImpl) EndCreateService() error {
	return nil
}

func (module *grpcServiceImpl) BeginSetupService() error {
	return nil
}

func (module *grpcServiceImpl) SetupService(service gomesh.Service) error {

	grpcService, ok := service.(Service)

	if ok {
		return grpcService.GrpcHandler(module.server)
	}

	return nil

}

func (module *grpcServiceImpl) EndSetupService() error {
	return nil
}

func (module *grpcServiceImpl) BeginStartService() error {
	return nil
}

func (module *grpcServiceImpl) StartService(service gomesh.Service) error {
	return nil
}

func (module *grpcServiceImpl) EndStarService() error {
	return nil
}

func (module *grpcServiceImpl) getProvider() Provider {
	var provider Provider

	for {
		ok := module.mesh.ServiceByName(module.provider, &provider)

		if provider != nil {
			break
		}

		if !ok {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		panic(fmt.Sprintf("expect provider %s", module.provider))
	}

	return provider
}

type providerError struct{}

func (providerError) Error() string   { return "grpc provider not valid" }
func (providerError) Temporary() bool { return true }

func (module *grpcServiceImpl) dialOption(ctx context.Context) grpc.DialOption {
	return grpc.WithDialer(func(remote string, timeout time.Duration) (net.Conn, error) {

		module.DebugF("grpc dial to %s", remote)

		provider := module.getProvider()

		subCtx, subCtxCancel := context.WithTimeout(ctx, timeout)
		defer subCtxCancel()

		conn, err := provider.Connect(subCtx, remote)

		if err != nil {
			module.ErrorF("grpc dial to %s error: %s", remote, err)
			return nil, err
		}

		module.DebugF("dial success with conn %s -> %s", conn.LocalAddr(), conn.RemoteAddr())

		return conn, nil
	})
}

func (module *grpcServiceImpl) Dial(ctx context.Context, url string, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialOpsPrepended := append([]grpc.DialOption{
		module.dialOption(ctx),
		grpc.FailOnNonTempDialError(true),
	}, dialOpts...)

	return grpc.DialContext(ctx, url, dialOpsPrepended...)
}

func (module *grpcServiceImpl) Local(name string, creator CreatorF) {
	module.local[name] = creator
	module.builder.RegisterService(name)
}

func (module *grpcServiceImpl) Remote(name string, connector ConnectorF) {
	module.remote[name] = connector
	module.builder.RegisterService(name)
}
