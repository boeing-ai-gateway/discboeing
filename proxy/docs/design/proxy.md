# Proxy Module

HTTP and SOCKS5 proxy implementation with protocol detection.

## Files

| File | Purpose |
|------|---------|
| `internal/proxy/server.go` | Main server, lifecycle management |
| `internal/proxy/http.go` | HTTP/HTTPS proxy using goproxy |
| `internal/proxy/socks.go` | SOCKS5 proxy using go-socks5 |
| `internal/proxy/detector.go` | Protocol detection logic |

## Protocol Detection

```go
type Protocol int

const (
    ProtocolUnknown Protocol = iota
    ProtocolHTTP
    ProtocolSOCKS5
    ProtocolSOCKS4
)

// Detect reads the first byte(s) to determine protocol
func Detect(conn net.Conn) (Protocol, *PeekedConn, error) {
    // Set a short timeout for detection
    conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    defer conn.SetReadDeadline(time.Time{})

    buf := make([]byte, 1)
    _, err := io.ReadFull(conn, buf)
    if err != nil {
        return ProtocolUnknown, nil, err
    }

    var proto Protocol
    switch buf[0] {
    case 0x05:
        proto = ProtocolSOCKS5
    case 0x04:
        proto = ProtocolSOCKS4
    default:
        // Check if ASCII printable (HTTP methods start with letters)
        if buf[0] >= 'A' && buf[0] <= 'Z' {
            proto = ProtocolHTTP
        } else {
            proto = ProtocolUnknown
        }
    }

    // Wrap connection to replay peeked byte
    peeked := NewPeekedConn(conn, buf)
    return proto, peeked, nil
}

// PeekedConn wraps a connection with pre-read bytes
type PeekedConn struct {
    net.Conn
    reader io.Reader
}

func NewPeekedConn(conn net.Conn, peeked []byte) *PeekedConn {
    return &PeekedConn{
        Conn:   conn,
        reader: io.MultiReader(bytes.NewReader(peeked), conn),
    }
}

func (c *PeekedConn) Read(b []byte) (int, error) {
    return c.reader.Read(b)
}
```

## Server

```go
type Server struct {
    cfg       *config.Config
    listener  net.Listener
    httpProxy *goproxy.ProxyHttpServer
    socksServer *socks5.Server
    injector  *injector.Injector
    filter    *filter.Filter
    logger    *logger.Logger
    certMgr   *cert.Manager
}

func New(cfg *config.Config) (*Server, error) {
    certMgr, err := cert.NewManager(cfg.TLS.CertDir)
    if err != nil {
        return nil, fmt.Errorf("cert manager: %w", err)
    }

    s := &Server{
        cfg:      cfg,
        injector: injector.New(),
        filter:   filter.New(),
        logger:   logger.New(cfg.Logging),
        certMgr:  certMgr,
    }

    s.httpProxy = s.setupHTTPProxy()
    s.socksServer = s.setupSOCKSProxy()

    // Apply initial config
    s.ApplyConfig(cfg)

    return s, nil
}

func (s *Server) ListenAndServe() error {
    addr := fmt.Sprintf(":%d", s.cfg.Proxy.Port)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    s.listener = listener

    s.logger.Info("proxy started", "addr", addr)

    for {
        conn, err := listener.Accept()
        if err != nil {
            if errors.Is(err, net.ErrClosed) {
                return nil
            }
            s.logger.Error("accept error", "error", err)
            continue
        }

        go s.handleConnection(conn)
    }
}

func (s *Server) handleConnection(conn net.Conn) {
    defer conn.Close()

    proto, peeked, err := Detect(conn)
    if err != nil {
        s.logger.Debug("detection failed", "error", err)
        return
    }

    switch proto {
    case ProtocolHTTP:
        s.handleHTTP(peeked)
    case ProtocolSOCKS5:
        s.handleSOCKS(peeked)
    case ProtocolSOCKS4:
        s.logger.Warn("SOCKS4 not supported")
        // Could send SOCKS4 rejection
    default:
        s.logger.Warn("unknown protocol")
    }
}

func (s *Server) ApplyConfig(cfg *config.Config) {
    s.injector.SetRules(cfg.Headers)
    s.filter.SetAllowlist(cfg.Allowlist.Domains, cfg.Allowlist.IPs)
    s.filter.SetEnabled(cfg.Allowlist.Enabled)
}

func (s *Server) Close() error {
    if s.listener != nil {
        return s.listener.Close()
    }
    return nil
}
```

## HTTP Proxy

The HTTP proxy records two layers of traffic when request recording is enabled:
- the HTTP request/response handshake as JSONL metadata
- a separate framed binary stream for post-upgrade traffic such as WebSockets after a `101 Switching Protocols` response

Upgraded stream chunks are captured on both directions of the proxied `io.ReadWriteCloser` and queued to a background writer. This preserves observed direction and ordering without blocking the active network path when disk writes are slow.

```go
func (s *Server) setupHTTPProxy() *goproxy.ProxyHttpServer {
    proxy := goproxy.NewProxyHttpServer()
    proxy.Verbose = false

    // Set up MITM with our CA
    ca, _ := s.certMgr.GetOrCreateCA()
    goproxy.GoproxyCa = *ca
    goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(ca)}
    goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(ca)}

    // Handle CONNECT requests (HTTPS)
    proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
        // Filter check
        if !s.filter.AllowHost(host) {
            s.logger.Info("blocked", "host", host, "reason", "filter")
            return goproxy.RejectConnect, host
        }
        return goproxy.MitmConnect, host
    })

    // Handle all requests (after MITM decryption for HTTPS)
    proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
        // Filter check (for plain HTTP)
        if !s.filter.AllowHost(req.Host) {
            s.logger.Info("blocked", "host", req.Host, "reason", "filter")
            return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusForbidden, "Blocked by proxy")
        }

        // Inject headers
        s.injector.Apply(req)

        // Log request
        s.logger.LogRequest(req)

        return req, nil
    })

    // Log responses
    proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
        if resp != nil {
            s.logger.LogResponse(resp, ctx.Req)
        }
        return resp
    })

    return proxy
}

func (s *Server) handleHTTP(conn *PeekedConn) {
    // goproxy expects to handle the full HTTP connection
    // We need to wrap it in a listener that returns this single connection
    singleConnListener := &singleConnListener{conn: conn}
    s.httpProxy.Serve(singleConnListener)
}

// singleConnListener is a net.Listener that returns one connection then closes
type singleConnListener struct {
    conn net.Conn
    once sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
    var conn net.Conn
    l.once.Do(func() {
        conn = l.conn
    })
    if conn != nil {
        return conn, nil
    }
    return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error   { return nil }
func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }
```

## SOCKS5 Proxy

```go
func (s *Server) setupSOCKSProxy() *socks5.Server {
    return socks5.NewServer(
        socks5.WithRule(&filterRule{filter: s.filter, logger: s.logger}),
        socks5.WithLogger(&socksLogger{logger: s.logger}),
        // No authentication
        socks5.WithAuthMethods([]socks5.Authenticator{
            socks5.NoAuthAuthenticator{},
        }),
    )
}

func (s *Server) handleSOCKS(conn *PeekedConn) {
    // go-socks5 handles the connection directly
    if err := s.socksServer.ServeConn(conn); err != nil {
        s.logger.Debug("socks error", "error", err)
    }
}

// filterRule implements socks5.Rule for allowlist filtering
type filterRule struct {
    filter *filter.Filter
    logger *logger.Logger
}

func (r *filterRule) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
    var host string
    if req.DestAddr.FQDN != "" {
        host = req.DestAddr.FQDN
    } else {
        host = req.DestAddr.IP.String()
    }

    allowed := r.filter.AllowHost(host)
    if !allowed {
        r.logger.Info("blocked", "host", host, "port", req.DestAddr.Port, "reason", "filter")
    } else {
        r.logger.Info("socks connect", "host", host, "port", req.DestAddr.Port)
    }

    return ctx, allowed
}

// socksLogger adapts our logger to socks5.Logger interface
type socksLogger struct {
    logger *logger.Logger
}

func (l *socksLogger) Errorf(format string, args ...interface{}) {
    l.logger.Error(fmt.Sprintf(format, args...))
}
```
