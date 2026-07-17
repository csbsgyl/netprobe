package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/csbsgyl/netprobe/internal/check"
	"github.com/csbsgyl/netprobe/internal/protocol"
)

// These values may be replaced by release builds, for example:
// go build -ldflags "-X main.defaultServer=https://probe.example.com -X main.version=v1.0.0".
var (
	defaultServer = "https://v4.tcptest.cn"
	version       = "dev"
)

const (
	exitPass          = 0
	exitFailed        = 1
	exitUsage         = 2
	exitRuntimeError  = 3
	exitIndeterminate = 4
)

type options struct {
	server  string
	json    bool
	timeout time.Duration
}

var errFlagParsing = errors.New("flag parsing failed")

type errorOutput struct {
	Error protocol.APIError `json:"error"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(parent context.Context, args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		if !errors.Is(err, errFlagParsing) || opts.json {
			writeError(stderr, opts.json, "invalid_configuration", err)
		}
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()
	client, err := check.NewClient(
		opts.server,
		check.WithClientInfo("netcheck", version),
		check.WithUDPTimeout(udpBudget(opts.timeout)),
	)
	if err != nil {
		writeError(stderr, opts.json, "invalid_configuration", err)
		return exitUsage
	}

	result, err := client.Run(ctx)
	if err != nil {
		code := "probe_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			code = "timeout"
		} else if errors.Is(err, context.Canceled) {
			code = "canceled"
		}
		writeError(stderr, opts.json, code, err)
		return exitRuntimeError
	}
	if opts.json {
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(result); err != nil {
			writeError(stderr, true, "output_failed", err)
			return exitRuntimeError
		}
	} else {
		writeHuman(stdout, result)
	}

	switch result.Verdict {
	case protocol.VerdictPass:
		return exitPass
	case protocol.VerdictFail:
		return exitFailed
	default:
		return exitIndeterminate
	}
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{server: defaultServer, json: jsonModeRequested(args), timeout: 10 * time.Second}
	flags := flag.NewFlagSet("netcheck", flag.ContinueOnError)
	flagOutput := stderr
	if opts.json {
		flagOutput = io.Discard
	}
	flags.SetOutput(flagOutput)
	flags.StringVar(&opts.server, "server", opts.server, "netprobe server URL")
	flags.BoolVar(&opts.json, "json", opts.json, "write the result as JSON")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "overall timeout (for example 10s)")
	flags.Usage = func() {
		fmt.Fprintln(flagOutput, "Usage: netcheck [--server URL] [--json] [--timeout DURATION]")
		fmt.Fprintln(flagOutput)
		fmt.Fprintln(flagOutput, "Exit codes: 0 pass, 1 fail, 2 usage, 3 runtime error, 4 indeterminate.")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, err
		}
		return opts, fmt.Errorf("%w: %v", errFlagParsing, err)
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return opts, fmt.Errorf("unexpected positional argument %q", flags.Arg(0))
	}
	if strings.TrimSpace(opts.server) == "" {
		return opts, errors.New("--server must not be empty")
	}
	if opts.timeout < time.Second {
		return opts, errors.New("--timeout must be at least 1s")
	}
	return opts, nil
}

func jsonModeRequested(args []string) bool {
	requested := false
	for _, arg := range args {
		switch arg {
		case "--json", "--json=true", "--json=1", "--json=t", "--json=T", "--json=TRUE", "--json=True":
			requested = true
		case "--json=false", "--json=0", "--json=f", "--json=F", "--json=FALSE", "--json=False":
			requested = false
		}
	}
	return requested
}

func udpBudget(overall time.Duration) time.Duration {
	budget := overall * 3 / 5
	if budget > 5*time.Second {
		return 5 * time.Second
	}
	return budget
}

func writeError(writer io.Writer, asJSON bool, code string, err error) {
	if asJSON {
		_ = json.NewEncoder(writer).Encode(errorOutput{
			Error: protocol.APIError{Code: code, Message: err.Error()},
		})
		return
	}
	fmt.Fprintf(writer, "netcheck: %v\n", err)
}

func writeHuman(writer io.Writer, result protocol.CompleteSessionResponse) {
	fmt.Fprintln(writer, "TCPT Network Check")
	fmt.Fprintln(writer)
	writeField(writer, "Status", strings.ToUpper(result.Verdict))
	if result.Summary != "" {
		writeField(writer, "Summary", result.Summary)
	}
	if result.PublicIP != "" {
		publicAddress := result.PublicIP
		if result.PublicPort > 0 {
			publicAddress = net.JoinHostPort(result.PublicIP, fmt.Sprint(result.PublicPort))
		}
		writeField(writer, "Public address", publicAddress)
	}
	writeField(writer, "UDP reachable", yesNo(result.UDPReachable))
	writeField(writer, "Alternate callback", yesNo(result.AlternatePortReachable))
	if result.MappingBehavior != "" {
		writeField(writer, "Mapping behavior", result.MappingBehavior)
	}
	if result.FilteringBehavior != "" {
		writeField(writer, "Filtering behavior", result.FilteringBehavior)
	}
	if result.LegacyNAT != "" {
		writeField(writer, "Legacy NAT estimate", result.LegacyNAT)
	}
	if len(result.Checks) > 0 {
		fmt.Fprintln(writer)
		for _, item := range result.Checks {
			line := fmt.Sprintf("[%s] %s", strings.ToUpper(item.Status), item.Name)
			if item.Detail != "" {
				line += ": " + item.Detail
			}
			fmt.Fprintln(writer, line)
		}
	}
	if result.SessionID != "" {
		fmt.Fprintln(writer)
		writeField(writer, "Report ID", result.SessionID)
	}
}

func writeField(writer io.Writer, label, value string) {
	fmt.Fprintf(writer, "%-20s %s\n", label+":", value)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
