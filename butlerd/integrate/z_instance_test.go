package integrate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/itchio/butler/butlerd"
	"github.com/itchio/butler/butlerd/messages"
	"github.com/itchio/mitch"
	"github.com/itchio/wharf/state"
	"github.com/pkg/errors"
	"github.com/sourcegraph/jsonrpc2"
)

type ButlerConn struct {
	Ctx            context.Context
	Cancel         context.CancelFunc
	RequestContext *butlerd.RequestContext
	Handler        *handler
}

type ButlerInstance struct {
	Ctx      context.Context
	Cancel   context.CancelFunc
	Address  string
	Secret   string
	Consumer *state.Consumer
	Logf     func(format string, args ...interface{})
	Conn     *ButlerConn

	t    *testing.T
	opts instanceOpts
	// may be nil
	Server mitch.Server
}

type instanceOpts struct {
	mockServer bool
}

type instanceOpt func(o *instanceOpts)

func newInstance(t *testing.T, options ...instanceOpt) *ButlerInstance {
	var opts instanceOpts
	for _, o := range options {
		o(&opts)
	}

	ctx, cancel := context.WithCancel(context.Background())

	logf := t.Logf
	if os.Getenv("LOUD_TESTS") == "1" {
		logf = func(msg string, args ...interface{}) {
			log.Printf(msg, args...)
		}
	}

	consumer := &state.Consumer{
		OnMessage: func(lvl string, msg string) {
			logf("[%s] %s", lvl, msg)
		},
	}

	server, err := mitch.NewServer(ctx, mitch.WithConsumer(consumer))
	must(err)

	args := []string{
		"daemon",
		"--json",
		"--transport", "tcp",
		"--keep-alive",
		"--dbpath", "file::memory:?cache=shared",
		"--destiny-pid", conf.PidString,
		"--destiny-pid", conf.PpidString,
	}
	{
		addressString := fmt.Sprintf("http://%s", server.Address())
		args = append(args, "--address", addressString)
		logf("Using mock server %s", addressString)
	}
	bExec := exec.CommandContext(ctx, conf.ButlerPath, args...)

	stdout, err := bExec.StdoutPipe()
	must(err)

	stderr, err := bExec.StderrPipe()
	must(err)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			consumer.Infof("[%s] %s", "butler stderr", s.Text())
		}
	}()

	must(bExec.Start())

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- bExec.Wait()
	}()

	s := bufio.NewScanner(stdout)
	addrChan := make(chan string)

	var secret string
	go func() {
		defer cancel()

		for s.Scan() {
			line := s.Text()

			im := make(map[string]interface{})
			err := json.Unmarshal([]byte(line), &im)
			if err != nil {
				consumer.Infof("[%s] %s", "butler stdout", line)
				continue
			}

			typ := im["type"].(string)
			switch typ {
			case "butlerd/listen-notification":
				secret = im["secret"].(string)
				tcpBlock := im["tcp"].(map[string]interface{})
				addrChan <- tcpBlock["address"].(string)
			case "log":
				consumer.Infof("[butler] %s", im["message"].(string))
			default:
				must(errors.Errorf("unknown butlerd request: %s", typ))
			}
		}
	}()

	var address string
	select {
	case address = <-addrChan:
		// cool!
	case err := <-waitErr:
		must(err)
	case <-time.After(2 * time.Second):
		must(errors.Errorf("Timed out waiting for butlerd address"))
	}
	must(err)

	bi := &ButlerInstance{
		t:        t,
		opts:     opts,
		Ctx:      ctx,
		Cancel:   cancel,
		Address:  address,
		Secret:   secret,
		Logf:     logf,
		Consumer: consumer,
		Server:   server,
	}
	bi.Connect()
	bi.SetupTmpInstallLocation()

	return bi
}

func (bi *ButlerInstance) Unwrap() (*butlerd.RequestContext, *handler, context.CancelFunc) {
	return bi.Conn.RequestContext, bi.Conn.Handler, bi.Cancel
}

func (bi *ButlerInstance) Disconnect() {
	bi.Conn.Cancel()
	bi.Conn = nil
}

func (bi *ButlerInstance) Connect() (*butlerd.RequestContext, *handler, context.CancelFunc) {
	ctx, cancel := context.WithCancel(bi.Ctx)

	h := newHandler(bi.Consumer)

	messages.Log.Register(h, func(rc *butlerd.RequestContext, params butlerd.LogNotification) {
		bi.Consumer.OnMessage(string(params.Level), params.Message)
	})

	tcpConn, err := net.DialTimeout("tcp", bi.Address, 2*time.Second)
	must(err)

	stream := jsonrpc2.NewBufferedStream(tcpConn, butlerd.LFObjectCodec{})

	jc := jsonrpc2.NewConn(ctx, stream, jsonrpc2.AsyncHandler(h))
	go func() {
		<-ctx.Done()
		jc.Close()
	}()

	rc := &butlerd.RequestContext{
		Conn:     &butlerd.JsonRPC2Conn{Conn: jc},
		Ctx:      ctx,
		Consumer: bi.Consumer,
	}

	_, err = messages.MetaAuthenticate.TestCall(rc, butlerd.MetaAuthenticateParams{
		Secret: bi.Secret,
	})
	must(err)

	bi.Conn = &ButlerConn{
		Ctx:            ctx,
		Cancel:         cancel,
		Handler:        h,
		RequestContext: rc,
	}
	return bi.Unwrap()
}