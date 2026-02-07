package main

import (
    "context"
    "fmt"
    "os"

    "github.com/andy/timesink/internal/app"
    "github.com/andy/timesink/internal/cli"
)

func main() {
    // If the user asked for help, avoid initializing the full app (which may prompt)
    skipInit := false
    for _, a := range os.Args[1:] {
        if a == "-h" || a == "--help" || a == "help" {
            skipInit = true
            break
        }
    }

    if !skipInit {
        ctx := context.Background()
        a, err := app.New(ctx)
        if err != nil {
            fmt.Fprintf(os.Stderr, "failed to initialize app: %v\n", err)
            os.Exit(1)
        }
        defer a.Close()
        cli.SetApp(a)
    }

    if err := cli.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
