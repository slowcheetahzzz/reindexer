package cproto

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restream/reindexer/bindings"
	"github.com/restream/reindexer/test/helpers"
)

func BenchmarkGetConn(b *testing.B) {
	srv1 := helpers.TestServer{T: nil, RpcPort: "6651", HttpPort: "9951", DbName: "cproto"}
	if err := srv1.Run(); err != nil {
		panic(err)
	}
	defer srv1.Stop()

	binding := NetCProto{}
	u, _ := url.Parse(fmt.Sprintf("cproto://127.0.0.1:%s/%s_%s", srv1.RpcPort, srv1.DbName, srv1.RpcPort))
	dsn := []url.URL{*u}
	err := binding.Init(dsn, bindings.OptionConnect{CreateDBIfMissing: true})
	if err != nil {
		panic(err)
	}

	b.Run("getConn", func(b *testing.B) {
		var conn *connection
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			conn, err = binding.getConn(ctx)
			if err != nil {
				panic(err)
			}
		}

		_ = conn
	})

}

func TestCprotoPool(t *testing.T) {
	t.Run("success connection", func(t *testing.T) {
		t.Skip("think about mock login")
		serv, addr, err := runTestServer()
		require.NoError(t, err)
		defer serv.Close()

		c := new(NetCProto)
		err = c.Init([]url.URL{*addr})
		require.NoError(t, err)

		assert.Equal(t, defConnPoolSize, len(serv.conns))
	})

	t.Run("rotate connections on each getConn", func(t *testing.T) {
		srv1 := helpers.TestServer{T: t, RpcPort: "6661", HttpPort: "9961", DbName: "cproto"}
		dsn := fmt.Sprintf("cproto://127.0.0.1:%s/%s_%s", srv1.RpcPort, srv1.DbName, srv1.RpcPort)

		err := srv1.Run()
		require.NoError(t, err)
		defer srv1.Stop()

		u, err := url.Parse(dsn)
		require.NoError(t, err)
		c := new(NetCProto)
		err = c.Init([]url.URL{*u})
		require.NoError(t, err)

		conns := make(map[*connection]bool)
		for i := 0; i < defConnPoolSize; i++ {
			conn, err := c.getConn(context.Background())
			require.NoError(t, err)
			if _, ok := conns[conn]; ok {
				t.Fatalf("getConn not rotate conn")
			}
			conns[conn] = true
		}

		// return anew from the pool
		conn, err := c.getConn(context.Background())
		require.NoError(t, err)
		if _, ok := conns[conn]; !ok {
			t.Fatalf("getConn not rotate conn")
		}
	})

}

func runTestServer() (s *testServer, addr *url.URL, err error) {
	startPort := 40000
	var l net.Listener
	var port int
	for port = startPort; port < startPort+10; port++ {
		if l, err = net.Listen("tcp", fmt.Sprintf(":%d", port)); err == nil {
			break
		}
	}
	if err != nil {
		return
	}
	s = &testServer{l: l}
	go s.acceptLoop()
	addr, _ = url.Parse(fmt.Sprintf("cproto://127.0.0.1:%d", port))
	return
}

type testServer struct {
	l     net.Listener
	conns []net.Conn
}

func (s *testServer) acceptLoop() {
	for {
		conn, err := s.l.Accept()
		if err != nil {
			return
		}
		s.conns = append(s.conns, conn)
	}
}

func (s *testServer) Close() {
	s.l.Close()
}
