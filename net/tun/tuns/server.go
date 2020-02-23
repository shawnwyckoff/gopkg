package tuns

import (
	"fmt"
	"github.com/shawnwyckoff/commpkg/apputil/logger"
	"github.com/shawnwyckoff/commpkg/net/listeners"
	"github.com/shawnwyckoff/commpkg/net/mux"
	"github.com/shawnwyckoff/commpkg/net/smux"
	"github.com/shawnwyckoff/commpkg/sys/ios"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

type ServerOption struct {
	Listen              string
	Target              string
	ScavengeTTL         int
	AutoExpireSeconds   int
	MuxBufferSize       int
	MuxKeepAliveSeconds int
}

// A pool for stream copying
var xmitBuf sync.Pool

// handle multiplex-ed connection
func handleMux(conn net.Conn, config *ServerOption, lg logger.Logger) {
	// check if target is unix domain socket
	var isUnix bool
	if _, _, err := net.SplitHostPort(config.Target); err != nil {
		isUnix = true
	}
	lg.Infof("smux on connection: %s -> %s", conn.LocalAddr(), conn.RemoteAddr())

	// stream multiplex
	var muxer mux.Mux

	mux, err := smux.NewSmuxServer(conn, config.MuxBufferSize, config.MuxKeepAliveSeconds)
	if err != nil {
		lg.Erro(err)
		return
	}
	defer mux.Close()
	muxer = mux

	for {
		stream, err := muxer.Accept()
		if err != nil {
			lg.Erro(err)
			return
		}

		go func(p1 io.ReadWriteCloser) {
			var p2 net.Conn
			var err error
			if !isUnix {
				p2, err = net.Dial("tcp", config.Target)
			} else {
				p2, err = net.Dial("unix", config.Target)
			}

			if err != nil {
				lg.Erro(err)
				p1.Close()
				return
			}
			handleClient(p1, p2, lg)
		}(stream)
	}
}

func handleClient(p1 io.ReadWriteCloser, p2 net.Conn, lg logger.Logger) {
	defer p1.Close()
	defer p2.Close()

	if s1, ok := p1.(mux.Stream); ok {
		lg.Infof("stream opened in: %s out: %s", fmt.Sprint(s1.RemoteAddr(), "(", s1.ID(), ")"), p2.RemoteAddr())
		defer lg.Infof("stream closed in: %s out: %s", fmt.Sprint(s1.RemoteAddr(), "(", s1.ID(), ")"), p2.RemoteAddr())
	}

	// start tunnel & wait for tunnel termination
	streamCopy := func(dst io.Writer, src io.ReadCloser) chan struct{} {
		die := make(chan struct{})
		go func() {
			buf := xmitBuf.Get().([]byte)
			if _, err := ios.CopyBuffer(dst, src, buf); err != nil {
				if s1, ok := p1.(mux.Stream); ok {
					// verbose error handling
					cause := err
					if e, ok := err.(interface{ Cause() error }); ok {
						cause = e.Cause()
					}

					if smux.ConvertInternalError(cause) == mux.ErrInvalidProtocol {
						lg.Errof("smux error %s in: %s out: %s", err.Error(), fmt.Sprint(s1.RemoteAddr(), "(", s1.ID(), ")"), p2.RemoteAddr())

					}
				}
			}
			xmitBuf.Put(buf)
			close(die)
		}()
		return die
	}

	select {
	case <-streamCopy(p1, p2):
	case <-streamCopy(p2, p1):
	}
}

func ServeWait(l listeners.Listener, lg logger.Logger, config ServerOption) error {
	rand.Seed(int64(time.Now().Nanosecond()))

	xmitBuf.New = func() interface{} {
		return make([]byte, 4096)
	}

	// main loop
	var wg sync.WaitGroup
	loop := func(lis net.Listener) {
		defer wg.Done()

		for {
			if conn, err := lis.Accept(); err == nil {
				lg.Infof("remote address: %s", conn.RemoteAddr())
				go handleMux(conn, &config, lg)
			} else {
				lg.Erro(err)
			}
		}
	}

	lis, err := l.Listen(config.Listen)
	if err != nil {
		return err
	}
	wg.Add(1)
	go loop(lis)
	wg.Wait()
	return nil
}
