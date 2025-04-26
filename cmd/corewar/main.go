package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"go.creack.net/corewar/cli"
	"go.creack.net/corewar/vm"
)

func dump(vm []byte, pc uint32) {
	zz := make([]byte, 32)
	for i := 0; i < len(vm); {
		b := vm[i]
		if i%32 == 0 {
			if bytes.Equal(vm[i:i+32], zz) {
				fmt.Printf("\n*")
				for ; i < len(vm) && bytes.Equal(vm[i:i+32], zz); i += 32 {
				}
				continue
			}
			fmt.Printf("\n0x%04X:", i)
		}
		if i == int(pc) {
			fmt.Printf("\033[7m")
		}
		fmt.Printf(" %02x", b)
		if i == int(pc) {
			fmt.Printf("\033[27m")
		}
		i++
	}
	fmt.Printf("\n")
}

func run(ctx context.Context, cw *vm.Corewar) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-cw.Messages:
				if !ok {
					return
				}
				switch msg.Type {
				case vm.MsgDebug:
				// case vm.MsgLive, vm.MsgLiveMiss:
				case vm.MsgGameOver:
					fmt.Printf("%s\n", msg.Message)
					fmt.Printf("Cycles: %d in %s.\n", cw.Cycle, time.Since(start))
					return
				default:
					fmt.Println(strings.TrimSuffix(msg.Message, "\n"))
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			break
		case <-done:
			break
		default:
		}
		if err := cw.Round(); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to execute round: %w", err)
		}
	}
	close(cw.Messages)
	select {
	case <-done:
	case <-ctx.Done():
	}

	return nil
}

func main() {
	ctx := context.Background()

	cfg, _, err := cli.ParseConfig()
	if err != nil {
		log.Fatalf("Failed to parse CLI config: %s.", err)
	}

	cw := vm.NewCorewar(cfg)
	if err := cw.Round(); err != nil {
		log.Fatalf("Failed to execute first round: %s.", err)
	}

	if err := run(ctx, cw); err != nil {
		log.Fatal("Fail:", err.Error())
		return
	}
}
