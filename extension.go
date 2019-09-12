package grpcservice

import (
	"context"
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

// Register .
type Register interface {
	Client
	Local(name string, creator CreatorF)
	Remote(name string, connector ConnectorF)
}

// Client .
type Client interface {
	Dial(ctx context.Context, url string, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error)
}

type registerImpl struct {
	slf4go.Logger
	provider string // provider serivce name
	local    map[string]CreatorF
	remote   map[string]ConnectorF
	server   *grpc.Server
	config   config.Config
	servces  []Service
}

// New .
func New(name string) Register {

	providerName := "grpcservice.default"

	localservice.Register(providerName, func(config config.Config) (gomesh.Service, error) {
		return newBuiltinProvider(config)
	})

	return WithProvider(name, providerName)
}

// WithProvider create new grpc service extension with provider service name
func WithProvider(name string, provider string) Register {
	impl := &registerImpl{
		Logger:   slf4go.Get("mxwservice"),
		local:    make(map[string]CreatorF),
		remote:   make(map[string]ConnectorF),
		provider: provider,
	}

	gomesh.Builder().RegisterExtension(impl)

	localservice.Register(name, func(config config.Config) (gomesh.Service, error) {
		impl.config = config
		return impl, nil
	})

	return impl
}

func (extension *registerImpl) Start() error {

	go func() {
		for {
			if err := extension.server.Serve(extension); err != nil {
				extension.ErrorF("grpc serve err %s", err)
			}

			time.Sleep(extension.config.Get("backoff").Duration(time.Second * 5))
		}
	}()

	return nil
}

func (extension *registerImpl) Name() string {
	return "gomesh.extension.mxwservice"
}

// Accept waits for and returns the next connection to the listener.
func (extension *registerImpl) Accept() (net.Conn, error) {
	for {
		provider := extension.getProvider()

		if provider != nil {
			return provider.Listener().Accept()
		}

		time.Sleep(time.Second)
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (extension *registerImpl) Close() error {
	provider := extension.getProvider()

	if provider != nil {
		return provider.Listener().Close()
	}

	return nil
}

func fakeLocalAddr() net.Addr {
	localIP := net.ParseIP("127.0.0.1")
	return &net.TCPAddr{IP: localIP, Port: 0}
}

// Addr returns the listener's network address.
func (extension *registerImpl) Addr() net.Addr {
	return fakeLocalAddr()
}

func (extension *registerImpl) Begin(config config.Config, builder gomesh.MeshBuilder) error {

	for name := range extension.local {
		builder.RegisterService(extension.Name(), name)
	}

	for name := range extension.remote {
		builder.RegisterService(extension.Name(), name)
	}

	extension.server = grpc.NewServer()

	return nil
}

func (extension *registerImpl) CreateSerivce(serviceName string, config config.Config) (gomesh.Service, error) {
	f, ok := extension.local[serviceName]

	if ok {
		service, err := f(config)

		if err != nil {
			return nil, err
		}

		grpcService, ok := service.(Service)

		if ok {
			extension.servces = append(extension.servces, grpcService)
		}

		return service, nil

	}

	f2, ok := extension.remote[serviceName]

	if ok {
		remote := config.Get("remote").String("")
		conn, err := extension.Dial(context.Background(), remote, grpc.WithInsecure())

		if err != nil {
			return nil, err
		}

		return f2(conn)
	}

	return nil, xerrors.Wrapf(gomesh.ErrNotFound, "service %s not found", serviceName)
}

func (extension *registerImpl) getProvider() Provider {
	var provider Provider
	gomesh.Builder().FindService(extension.provider, &provider)

	return provider
}

func (extension *registerImpl) dialOption(ctx context.Context) grpc.DialOption {
	return grpc.WithDialer(func(remote string, timeout time.Duration) (net.Conn, error) {

		extension.DebugF("grpc dial to %s", remote)

		provider := extension.getProvider()

		if provider == nil {
			extension.DebugF("grpc provider not exists ...")
			return nil, xerrors.Errorf("grpc provider not valid")
		}

		subCtx, subCtxCancel := context.WithTimeout(ctx, timeout)
		defer subCtxCancel()

		conn, err := provider.Connect(subCtx, remote)

		if err != nil {
			extension.ErrorF("grpc dial to %s error: %s", remote, err)
			return nil, err
		}

		return conn, nil
	})
}

func (extension *registerImpl) Dial(ctx context.Context, url string, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {

	dialOpsPrepended := append([]grpc.DialOption{extension.dialOption(ctx)}, dialOpts...)

	return grpc.DialContext(ctx, url, dialOpsPrepended...)
}

func (extension *registerImpl) End() error {

	for _, service := range extension.servces {
		if err := service.GrpcHandler(extension.server); err != nil {
			return err
		}
	}

	return nil
}

func (extension *registerImpl) Local(name string, creator CreatorF) {
	extension.local[name] = creator
}

func (extension *registerImpl) Remote(name string, connector ConnectorF) {
	extension.remote[name] = connector
}
