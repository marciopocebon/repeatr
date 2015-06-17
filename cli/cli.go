package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/codegangsta/cli"
)

func Main(args []string, journal, output io.Writer) {
	App := cli.NewApp()

	App.Name = "repeatr"
	App.Usage = "Run it. Run it again."
	App.Version = "0.0.1"

	App.Writer = journal

	App.Commands = []cli.Command{
		RunCommandPattern(journal),
		ScanCommandPattern(journal, output),
	}

	// Reporting "no help topic for 'zyx'" and exiting with a *zero* is... silly.
	// A failure to hit a command should be an error.  Like, if a bash script does `repeatr somethingimportant`, there's no way it shouldn't *stop* when that's not there.
	App.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Fprintf(ctx.App.Writer, "'%s %v' is not a repeatr subcommand\n", ctx.App.Name, command)
		os.Exit(int(EXIT_BADARGS))
	}

	// Invoking version as a subcommand should also fly.
	App.Commands = append(App.Commands,
		cli.Command{
			Name:  "version",
			Usage: "Shows the version of repeatr",
			Action: func(ctx *cli.Context) {
				cli.ShowVersion(ctx)
			},
		},
	)

	App.Run(args)
}
