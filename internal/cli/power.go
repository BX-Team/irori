package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bx-team/irori/internal/config"
	"github.com/bx-team/irori/internal/daemon"
	"github.com/bx-team/irori/internal/ipc"
	"github.com/bx-team/irori/internal/models"
	"github.com/spf13/cobra"
)

func daemonCmd() *cobra.Command {
	c := &cobra.Command{
		Use:    "daemon",
		Short:  "internal supervisor for the server process",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			noStart, _ := cmd.Flags().GetBool("no-start")
			if dir == "" {
				return errors.New("--dir is required")
			}
			return daemon.Run(daemon.Options{Dir: dir, NoStart: noStart})
		},
	}
	c.Flags().Bool("no-start", false, "do not start the server immediately")
	return c
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "start the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			socket := cfg.SocketPath()
			if ipc.Alive(socket) {
				f, err := ipc.Query(socket, ipc.Frame{T: ipc.Start})
				if err != nil {
					return err
				}
				if f.T == ipc.Error {
					return errors.New(f.Err)
				}
				printStatus(cfg, f.Status)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := daemon.Spawn(ctx, cfg.Dir()); err != nil {
				return err
			}
			fmt.Println("server is starting, follow the console with: irori logs -f")
			return nil
		},
	}
}

func stopCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "stop",
		Short: "stop the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wait, _ := cmd.Flags().GetBool("wait")
			return control(cmd, ipc.Stop, wait)
		},
	}
	c.Flags().Bool("wait", true, "wait until the server has actually stopped")
	return c
}

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "restart the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			if !ipc.Alive(cfg.SocketPath()) {
				ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
				defer cancel()
				return daemon.Spawn(ctx, cfg.Dir())
			}
			return control(cmd, ipc.Restart, false)
		},
	}
}

func killCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill",
		Short: "kill the server process (SIGKILL)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return control(cmd, ipc.Kill, true)
		},
	}
}

func control(cmd *cobra.Command, action string, wait bool) error {
	cfg, err := mustConfig(cmd)
	if err != nil {
		return err
	}
	socket := cfg.SocketPath()
	if !ipc.Alive(socket) {
		fmt.Println("server is not running")
		return nil
	}
	f, err := ipc.Query(socket, ipc.Frame{T: action})
	if err != nil {
		return err
	}
	if f.T == ipc.Error {
		return errors.New(f.Err)
	}
	if !wait {
		printStatus(cfg, f.Status)
		return nil
	}

	deadline := time.Now().Add(time.Duration(cfg.Runtime.StopTimeoutSec+30) * time.Second)
	for time.Now().Before(deadline) {
		if !ipc.Alive(socket) {
			fmt.Println("server stopped")
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return errors.New("server did not stop within the allotted time")
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "show the server status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			socket := cfg.SocketPath()
			if !ipc.Alive(socket) {
				printStatus(cfg, nil)
				return nil
			}
			f, err := ipc.Query(socket, ipc.Frame{T: ipc.GetState})
			if err != nil {
				return err
			}
			printStatus(cfg, f.Status)
			return nil
		},
	}
}

func sendCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "cmd <command...>",
		Aliases: []string{"sendcmd", "send"},
		Short:   "send a command to the server console",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			socket := cfg.SocketPath()
			if !ipc.Alive(socket) {
				return errors.New("server is not running")
			}
			conn, err := ipc.Dial(socket)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()
			return conn.Send(ipc.Frame{T: ipc.Input, Data: strings.Join(args, " ")})
		},
	}
}

func printStatus(cfg *config.Config, st *models.Status) {
	name := cfg.Server.Name
	meta := cfg.Server.Type.Display()
	if cfg.Server.MCVersion != "" {
		meta += " " + cfg.Server.MCVersion
	}

	if st == nil {
		fmt.Printf("%s (%s)\nstate:   OFFLINE\n", name, meta)
		return
	}
	fmt.Printf("%s (%s)\nstate:   %s\n", name, meta, st.State.Label())
	if st.PID > 0 {
		fmt.Printf("pid:     %d\n", st.PID)
	}
	if st.State.IsUp() {
		fmt.Printf("uptime:  %s\n", time.Duration(st.Stats.UptimeSec)*time.Second)
		fmt.Printf("cpu/ram: %.1f%% · %.0f MiB\n", st.Stats.CPU, st.Stats.RSSMB)
		fmt.Printf("players: %d/%d\n", st.Stats.Players, st.Stats.MaxPlayer)
	}
	if st.LastErr != "" {
		fmt.Printf("error:   %s\n", st.LastErr)
	}
}
