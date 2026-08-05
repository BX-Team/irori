package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bx-team/irori/internal/ipc"
	"github.com/spf13/cobra"
)

func logsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "logs",
		Short: "show the server console",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := mustConfig(cmd)
			if err != nil {
				return err
			}
			follow, _ := cmd.Flags().GetBool("follow")
			n, _ := cmd.Flags().GetInt("lines")

			socket := cfg.SocketPath()
			if !ipc.Alive(socket) {
				return tailFile(cfg.LogPath(), n)
			}

			conn, err := ipc.Dial(socket)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()
			if err := conn.Send(ipc.Frame{T: ipc.Attach, Tail: n}); err != nil {
				return err
			}

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-stop
				_ = conn.Close()
			}()

			for {
				f, err := conn.Recv()
				if err != nil {
					return nil
				}
				switch f.T {
				case ipc.Log:
					for _, l := range f.Lines {
						fmt.Println(l.Text)
					}
					if f.Line != nil {
						fmt.Println(f.Line.Text)
					}
					if !follow && len(f.Lines) > 0 {
						return nil
					}
				}
			}
		},
	}
	c.Flags().BoolP("follow", "f", false, "follow new lines")
	c.Flags().IntP("lines", "n", 200, "how many trailing lines to show")
	return c
}

func tailFile(path string, n int) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("log is empty: the server has never been started")
			return nil
		}
		return err
	}
	defer f.Close()

	ring := make([]string, 0, n)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if len(ring) == n {
			ring = ring[1:]
		}
		ring = append(ring, sc.Text())
	}
	fmt.Println(strings.Join(ring, "\n"))
	return nil
}
