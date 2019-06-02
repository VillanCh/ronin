package main

import (
	"context"
	"github.com/VillanCh/ronin/ronin"
	"github.com/VillanCh/ronin/ronin/bp"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	RoninServerTarget = ""
)

func main() {
	app := cli.NewApp()

	/* setting log */
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "log-level,l",
			Value: "info",
		},
		cli.BoolFlag{
			Name: "quiet,q",
		},
		cli.StringFlag{
			Name:        "ronin-server,s",
			Destination: &RoninServerTarget,
		},
	}

	app.Before = func(context *cli.Context) error {
		if context.Bool("quiet") {
			logrus.SetOutput(ioutil.Discard)
			return nil
		}

		level, err := logrus.ParseLevel(context.String("log-level"))
		if err != nil {
			return errors.Errorf("failed to parse %s as log level", context.String("log-level"))
		}

		logrus.SetLevel(level)
		return nil
	}

	/* setting sub command */
	app.Commands = []cli.Command{
		{
			Name: "server",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name: "addr,a",
				},
			},
			Action: func(c *cli.Context) error {
				serverImpl := &ronin.RoninServer{}
				server := grpc.NewServer()
				bp.RegisterRoninServer(server, serverImpl)

				conn, err := net.Listen("tcp", c.String("addr"))
				if err != nil {
					return errors.Errorf("failed to listen in: %s reason: %s", c.String("addr"), err)
				}

				err = server.Serve(conn)
				if err != nil {
					return errors.Errorf("failed to serve: %s", err)
				}

				return nil
			},
		},
		{
			Name: "forward",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "remote-addr,r",
					Usage: "remote address like '1.1.1.1:123'",
				},
				cli.UintFlag{
					Name:  "mirror-port,p",
					Usage: "mirror port: [ronin-server-addr]:[mirror-port]",
				},
			},
			Action: func(c *cli.Context) error {
				if RoninServerTarget == "" {
					return errors.Errorf("no ronin server set")
				}

				rAddr := c.String("remote-addr")
				if !strings.Contains(rAddr, ":") {
					return errors.Errorf("not a valid remote-addr: %s", rAddr)
				}

				results := strings.SplitN(rAddr, ":", 2)
				rHost := results[0]
				rPort, err := strconv.ParseInt(strings.TrimSpace(results[1]), 10, 64)
				if err != nil {
					return errors.Errorf("failed to parse int: %s", results[1])
				}

				client, err := ronin.GetRoninClient(RoninServerTarget, grpc.WithInsecure())
				if err != nil {
					return err
				}

				ctx := context.Background()
				rsp, err := client.BuildRemoteTCPForward(ctx, &bp.ForwardRequest{
					TargetHost: rHost,
					TargetPort: uint32(rPort),
					RoninPort:  uint32(c.Uint("mirror-port")),
				})
				if err != nil {
					return errors.Errorf("failed to execute build remote tcp forward cmd: %s", err)
				}

				if !rsp.Ok {
					return errors.Errorf("failed to build remote tcp forward: %s", rsp.Reason)
				}

				return nil
			},
		},

		{
			Name: "proxy",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "client-proto,c",
					Usage: "client proto [socks5]",
				},
			},
			Action: func(c *cli.Context) error {

				return errors.New("not implemented")
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Errorf("failed to execute action: %s", err)
	}
}
