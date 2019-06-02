package ronin

import (
	"github.com/VillanCh/ronin/ronin/bp"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func GetRoninClient(target string, options ...grpc.DialOption) (bp.RoninClient, error) {
	clientConn, err := grpc.Dial(target, options...)
	if err != nil {
		return nil, errors.Errorf("failed to dail: %s", err)
	}
	return bp.NewRoninClient(clientConn), nil
}
