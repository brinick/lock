package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
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
	defaultPollTime := 30
	defaultMaxWait := 3600
	defaultLockDir, _ := os.UserHomeDir()
	defaultLockName := "default-lock"

	return &cli.Command{
		Name:  "acquire",
		Usage: "Acquire the lock",
		Flags: []cli.Flag{
			lockdirFlag(),
			locknameFlag(),
			&cli.IntFlag{
				Name:        "poll-interval",
				Aliases:     []string{"i"},
				Usage:       "Poll interval between lock checks, in secs",
				DefaultText: fmt.Sprintf("%d", defaultPollTime),
			},

			&cli.IntFlag{
				Name:        "max-wait",
				Usage:       "Maximum time to wait for lock, in secs",
				Aliases:     []string{"w"},
				DefaultText: fmt.Sprintf("%d", defaultMaxWait),
			},
		},
		Action: func(c *cli.Context) error {
			lockdir := strArg(c, "dir", defaultLockDir)
			lockname := strArg(c, "name", defaultLockName)
			polltime := intArg(c, "poll-interval", 30)
			maxWait := intArg(c, "max-wait", 3600)

			// Create the lock dir if inexistant
			if err := os.MkdirAll(lockdir, 0774); err != nil {
				return fmt.Errorf("unable to create lock dir %s: %v", lockdir, err)
			}

			req, err := createRequest(lockdir, lockname)
			if err != nil {
				return err
			}

			isTimeOut := timedOut(maxWait)

			for !req.isOldest() {
				if isTimeOut() {
					msg := fmt.Sprintf("Timed out (%ds) waiting to acquire lock", maxWait)
					if err := req.remove(); err != nil {
						msg = fmt.Sprintf(
							" (also failed to remove request %s: %v - please remove manually)",
							req.path,
							err,
						)
					}
					return fmt.Errorf(msg)
				}

				time.Sleep(time.Duration(polltime) * time.Second)
			}

			// first in queue, try and get lock
			for !isTimeOut() {
				lck, err := createLock(lockdir, lockname)
				switch err.(type) {
				case nil:
					// We have the lock:
					// 1. print out the lock token for the client to capture
					// 2. delete the request
					fmt.Println(lck.id())
					return req.remove()
				case lockExistsErr:
					// wait
				default:
					if removeErr := req.remove(); removeErr != nil {
						err = fmt.Errorf(
							"Error creating lock %v, and also failed to remove request %s: %v",
							err,
							req.path,
							removeErr,
						)
					}
					return err
				}
				time.Sleep(time.Duration(polltime) * time.Second)
			}

			return nil
		},
	}
}

func deleteCmd() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "Delete the lock",
		Flags: []cli.Flag{
			lockdirFlag(),
			locknameFlag(),
		},
		Action: func(c *cli.Context) error {
			defaultLockDir, _ := os.UserHomeDir()
			lockdir := strArg(c, "dir", defaultLockDir)
			if c.Args().Len() != 1 {
				return fmt.Errorf("Please give one argument: the UUID of the lock")
			}

			id := c.Args().First()
			match, _ := filepath.Glob(filepath.Join(lockdir, fmt.Sprintf("*__%s__*", id)))
			if len(match) != 1 {
				return fmt.Errorf("Found %d locks to delete", len(match))
			}

			if err := os.Remove(match[0]); err != nil {
				return fmt.Errorf("Unable to remove lock %s: %v", match[0], err)
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
		DefaultText: "$HOME",
	}
}

func locknameFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:        "name",
		Usage:       "The directory in which to create the lock",
		Aliases:     []string{"n"},
		DefaultText: "default-lock",
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

func timedOut(max int) func() bool {
	started := time.Now().Unix()
	return func() bool {
		val := (time.Now().Unix() - started) > int64(max)
		return val
	}
}
