package ronin

import (
	"context"
	"fmt"
	"github.com/VillanCh/ronin/ronin/bp"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"sync"
)

//// RoninServer is the server API for Ronin service.
//type RoninServer interface {
//	BuildRemoteTCPForward(context.Context, *ForwardRequest) (*ForwardResponse, error)
//	BuildProxyForward(Ronin_BuildProxyForwardServer) error
//}
func ServeForward(lisConn net.Listener, remoteHost string, remotePort uint32) {
	for {
		conn, err := lisConn.Accept()
		if err != nil {
			logrus.Errorf("failed to accept connection: %s", err)
		}
		go func(c net.Conn) {
			defer func() {
				_ = conn.Close()
			}()

			remoteConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort))
			if err != nil {
				logrus.Errorf("failed to connection to remote port: %s", err)
				return
			}
			defer func() {
				_ = remoteConn.Close()
			}()

			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, err := io.Copy(remoteConn, c)
				if err != nil {
					logrus.Errorf("failed to copy stream(->remote): %s", err)
				}
			}()
			go func() {
				defer wg.Done()
				_, err := io.Copy(c, remoteConn)
				if err != nil {
					logrus.Errorf("failed to copy stream(<-remote): %s", err)
				}
			}()
			wg.Wait()
		}(conn)
	}
}

func ToKey(request *bp.ForwardRequest) string {
	return fmt.Sprintf("r[%s:%d]-l[0.0.0.0:%d]", request.TargetHost, request.TargetPort, request.RoninPort)
}

type RoninServer struct {
	bp.RoninServer

	forwardTable sync.Map
}

type ForwardTask struct {
	Request *bp.ForwardRequest
	Cancel  context.CancelFunc
}

func (r *RoninServer) BuildRemoteTCPForward(ctx context.Context, request *bp.ForwardRequest) (*bp.ForwardResponse, error) {
	forwardCtx, cancel := context.WithCancel(context.Background())
	e := r.RegisterForward(request, cancel)
	if e != nil {
		return &bp.ForwardResponse{
			Ok:     false,
			Reason: fmt.Sprint(e),
		}, nil
	}

	if request.RoninPort <= 0 {
		r.ClearForward(request)
		return &bp.ForwardResponse{
			Ok:     false,
			Reason: fmt.Sprintf("not a valid ronin port: %d", request.RoninPort),
		}, nil
	}

	lisConn, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", request.RoninPort))
	if err != nil {
		r.ClearForward(request)
		return &bp.ForwardResponse{
			Ok:     false,
			Reason: fmt.Sprintf("failed to listening on local port[%d]: %s", request.RoninPort, err),
		}, nil
	}

	go func() {
		select {
		case <-forwardCtx.Done():
			_ = lisConn.Close()
		}
		r.ClearForward(request)
	}()

	go func() {
		ServeForward(lisConn, request.TargetHost, request.TargetPort)
		r.ClearForward(request)
	}()

	return &bp.ForwardResponse{
		Ok:     true,
		Reason: "",
	}, nil
}

func (r *RoninServer) BuildProxyForward(server bp.Ronin_BuildProxyForwardServer) error {
	return errors.New("not implemented")
}

func (r *RoninServer) RegisterForward(request *bp.ForwardRequest, cancel context.CancelFunc) error {
	k := ToKey(request)
	_, ok := r.forwardTable.Load(k)
	if ok {
		return errors.Errorf("register failed, existed key: %s", k)
	}
	r.forwardTable.Store(k, &ForwardTask{
		Request: request,
		Cancel:  cancel,
	})
	return nil
}

func (r *RoninServer) ClearForward(request *bp.ForwardRequest) {
	k := ToKey(request)
	task, ok := r.forwardTable.Load(k)
	if !ok {
		return
	}

	ret := task.(*ForwardTask)
	ret.Cancel()
	r.forwardTable.Delete(k)
}
