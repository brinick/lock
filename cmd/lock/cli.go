package main

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/brinick/lock"
)

func createApp() *cli.App {
	app := &cli.App{
		Name:  "lock",
		Usage: "Create/Delete locks",
		Commands: []*cli.Command{
			acquireCmd(),
			deleteCmd(),
		},
	}

	app.EnableBashCompletion = true
	return app
}

func acquireCmd() *cli.Command {
	return &cli.Command{
		Name:  "acquire",
		Usage: "Acquire the lock",
		Flags: []cli.Flag{
			lockdirFlag(),
			locknameFlag(),
			&cli.IntFlag{
				Name:        "poll-interval",
				Aliases:     []string{"i", "lock.poll"},
				Usage:       "Poll interval between lock checks, in secs",
				DefaultText: fmt.Sprintf("%d", lock.DefaultPollTime),
			},

			&cli.IntFlag{
				Name:        "max-wait",
				Usage:       "Maximum time to wait for lock, in secs",
				Aliases:     []string{"w", "lock.max-wait"},
				DefaultText: fmt.Sprintf("%d", lock.DefaultMaxWait),
			},
		},
		Action: func(c *cli.Context) error {
			lck, err := lock.Acquire(&lock.Configuration{
				Dir:          strArg(c, "dir", lock.DefaultDir),
				Name:         strArg(c, "name", lock.DefaultName),
				PollInterval: intArg(c, "poll-interval", lock.DefaultPollTime),
				MaxWait:      intArg(c, "max-wait", lock.DefaultMaxWait),
			})

			if err == nil {
				fmt.Print(lck.ID)
			}

			return err
		},
	}
}

func deleteCmd() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete the lock",
		Flags: []cli.Flag{
			lockdirFlag(),
		},
		Action: func(c *cli.Context) error {
			lockdir := strArg(c, "dir", lock.DefaultDir)
			if c.Args().Len() != 1 {
				return fmt.Errorf("Please give one argument: the UUID of the lock")
			}

			id := c.Args().First()
			lck, err := lock.WithID(id, lockdir)
			if err != nil {
				return fmt.Errorf("Failed to find lock with ID %s, cannot delete", id)
			}

			if err := lck.Remove(); err != nil {
				return fmt.Errorf("Unable to remove lock %s: %v", lck.Path(), err)
			}
			return nil
		},
	}
}

func lockdirFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:        "dir",
		Usage:       "The directory in which to create the lock",
		Aliases:     []string{"d"},
		DefaultText: lock.DefaultDir,
	}
}

func locknameFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:        "name",
		Usage:       "The name to give the lock",
		Aliases:     []string{"n"},
		DefaultText: lock.DefaultName,
	}
}

func intArg(c *cli.Context, name string, default_ int) int {
	if c.IsSet(name) {
		return c.Int(name)
	}
	return default_
}
func strArg(c *cli.Context, name string, default_ string) string {
	val := strings.TrimSpace(c.String(name))
	if len(val) == 0 {
		return default_
	}

	return val
}
