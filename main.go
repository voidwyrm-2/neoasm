package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/voidwyrm-2/neoasm/generator"
)

//go:embed version.txt
var version string

func _main() error {
	version = strings.TrimSpace(version)

	fVersion := flag.Bool("v", false, "Show the current Neoasm version.")
	fOutpath := flag.String("o", "out", "The output path of the resulting ROM.")

	flag.Parse()

	if *fVersion {
		fmt.Println("Neoasm", version)
		return nil
	}

	args := flag.Args()

	if len(args) == 0 {
		return fmt.Errorf("Expected 'neoasm <file>'")
	}

	path := args[0]
	outpath := *fOutpath

	if filepath.Ext(outpath) != ".rom" {
		outpath += ".rom"
	}

	fIn, err := os.Open(path)
	if err != nil {
		return err
	}

	defer fIn.Close()

	gen := generator.New(path, fIn)

	data, err := gen.Generate()
	if err != nil {
		return err
	}

	err = os.WriteFile(outpath, data, 0o666)
	if err != nil {
		return err
	}

	fmt.Printf("Assembled as '%s' in %d bytes\n", outpath, len(data))

	return nil
}

func main() {
	if err := _main(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
