package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/KamogeloMahlake/pawchive_downloader/internal/archive"
	"github.com/KamogeloMahlake/pawchive_downloader/internal/clients"
	"github.com/KamogeloMahlake/pawchive_downloader/internal/storage"
	"github.com/KamogeloMahlake/pawchive_downloader/internal/viewer"
)

const (
	defaultBaseURL = "https://pawchive.pw/api/v1"
	defaultDataURL = "https://n4.pawchive.pw/data/"
	defaultDir     = "pawchive_data"
)

func usage() {
	fmt.Fprint(os.Stderr, `pawchive_downloader - archive creators from Pawchive

Usage:
  pawchive_downloader <command> [flags] [arguments]

Commands:
  creator  <service> <id>            show a creator's profile (no download)
  download <service> <id>            download all posts and files of a creator
  update   [<service> <id>]          fetch new and edited posts; with no arguments,
                                     update every creator already downloaded
  list     [<service> <id>]          list downloaded creators, or one creator's posts
  show     <service> <id> <post-id>  read one downloaded post in the terminal
  serve                              browse everything in your web browser

Flags (any command; can go before or after the arguments):
  -dir      data folder              (default "pawchive_data", env PAWCHIVE_DIR)
  -base     API base URL             (env PAWCHIVE_BASE_URL)
  -data-url file server URL          (env PAWCHIVE_DATA_URL)
  -workers  parallel downloads       (default 4)
  -no-files save post text only, skip files        (download/update)
  -addr     address for serve        (default 127.0.0.1:8080)

Examples:
  pawchive_downloader download fanbox 3773642
  pawchive_downloader update
  pawchive_downloader show fanbox 3773642 1234567
  pawchive_downloader serve
`)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type options struct {
	dir, base, dataURL, addr string
	workers                  int
	noFiles                  bool
}

func newFlagSet(name string, o *options) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&o.dir, "dir", getenv("PAWCHIVE_DIR", defaultDir), "data folder")
	fs.StringVar(&o.base, "base", getenv("PAWCHIVE_BASE_URL", defaultBaseURL), "API base URL")
	fs.StringVar(&o.dataURL, "data-url", getenv("PAWCHIVE_DATA_URL", defaultDataURL), "file server URL")
	fs.StringVar(&o.addr, "addr", "127.0.0.1:8080", "listen address for serve")
	fs.IntVar(&o.workers, "workers", 4, "parallel downloads")
	fs.BoolVar(&o.noFiles, "no-files", false, "skip downloading files")
	return fs
}

func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "interrupted; progress was saved, run the same command again to continue")
			os.Exit(130)
		}
		log.SetFlags(0)
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage()
		return nil
	}
	cmd, rest := args[0], args[1:]

	var o options
	fs := newFlagSet(cmd, &o)
	pos, err := parseInterspersed(fs, rest)
	if err != nil {
		return err
	}

	need := func(n int, what string) error {
		if len(pos) != n {
			return fmt.Errorf("usage: pawchive_downloader %s %s", cmd, what)
		}
		return nil
	}

	st := storage.NewStore(o.dir)
	client := clients.NewClient(strings.TrimRight(o.base, "/"))
	syncer := &archive.Syncer{
		Client:    client,
		Store:     st,
		DataUrl:   o.dataURL,
		Workers:   o.workers,
		Skipfiles: o.noFiles,
		Log:       func(f string, a ...any) { fmt.Printf(f+"\n", a...) },
	}

	switch cmd {
	case "creator":
		if err := need(2, "<service> <id>"); err != nil {
			return err
		}
		c, err := client.GetCreator(ctx, pos[0], pos[1])
		if err != nil {
			return err
		}
		fmt.Printf("%+v\n", c)
		return nil

	case "download":
		if err := need(2, "<service> <id>"); err != nil {
			return err
		}
		rep, err := syncer.Run(ctx, pos[0], pos[1])
		printReport(pos[0], pos[1], rep)
		return err

	case "update":
		switch len(pos) {
		case 2:
			// Updating a creator you never downloaded is almost always a typo.
			if _, err := st.LoadIndex(pos[0], pos[1]); err != nil {
				return fmt.Errorf("%w (use the download command first)", err)
			}
			rep, err := syncer.Run(ctx, pos[0], pos[1])
			printReport(pos[0], pos[1], rep)
			return err
		case 0:
			list, err := st.ListCreators()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Println("nothing downloaded yet")
				return nil
			}
			var failed int
			for _, ix := range list {
				rep, err := syncer.Run(ctx, ix.Service, ix.UserID)
				printReport(ix.Service, ix.UserID, rep)
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					failed++
					log.Printf("error updating %s/%s: %v", ix.Service, ix.UserID, err)
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d creator(s) failed to update", failed)
			}
			return nil
		default:
			return errors.New("usage: pawchive_downloader update [<service> <id>]")
		}

	case "list":
		switch len(pos) {
		case 0:
			return viewer.PrintCreators(os.Stdout, st)
		case 2:
			return viewer.PrintPosts(os.Stdout, st, pos[0], pos[1])
		default:
			return errors.New("usage: pawchive_downloader list [<service> <id>]")
		}

	case "show":
		if err := need(3, "<service> <id> <post-id>"); err != nil {
			return err
		}
		return viewer.PrintPost(os.Stdout, st, pos[0], pos[1], pos[2])

	case "serve":
		if len(pos) != 0 {
			return errors.New("usage: pawchive_downloader serve [-addr host:port]")
		}
		return viewer.Serve(ctx, st, o.addr, os.Stdout)

	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func printReport(service, id string, r archive.Report) {
	fmt.Printf("\n%s/%s: %d new, %d updated, %d unchanged", service, id, r.New, r.Updated, r.Unchanged)
	if r.MissingRemote > 0 {
		fmt.Printf(", %d no longer on server", r.MissingRemote)
	}
	fmt.Printf("; files: %d saved, %d failed\n", r.FilesOK, r.FilesFailed)
	if r.FilesFailed > 0 {
		fmt.Println("some files failed; run the same command again to retry them")
	}
}
