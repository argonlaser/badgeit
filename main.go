package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/scriptnull/badgeit/contracts"
	"github.com/scriptnull/badgeit/formatters"
)

// VERSION : Tells the version of badgeit
const VERSION = "0.1.0"

func main() {
	// Parse Flags
	fFlag := flag.String("f", "all", "Format for arranging the badges.")
	dFlag := flag.String("d", " ", "Delimiter to be used.")
	sFlag := flag.String("s", "", "Style of the badge.")
	vFlag := flag.Bool("v", false, "Version information.")
	flag.Parse()

	if *vFlag {
		fmt.Println("badgeit version: " + VERSION)
		os.Exit(0)
	}

	// Obtain destination path, if not found, it will be cwd
	args := flag.Args()
	path, err := os.Getwd()
	if len(args) != 0 && len(args[0]) > 0 {
		path = args[0]
	}

	// Check Contract agreement and obtain eligible badges
	badges := contracts.PossibleBadges(path)

	if len(badges) == 0 {
		fmt.Println("0 badges detected.")
		os.Exit(0)
	}

	// Get Suitable Formatter
	formatter, err := formatters.NewFormatter(
		formatters.FormatterOption{
			Badges:    badges,
			Delimiter: *dFlag,
			Type:      *fFlag,
			Style:     *sFlag,
		},
	)
	if err != nil {
		fmt.Fprint(os.Stderr, "Invalid -f option. \n")
	}

	result := formatter.Format()
	fmt.Fprintf(os.Stdout, "%s\n", result)
	fmt.Println("")
	fmt.Println(len(badges), " badges detected.")
	os.Exit(0)
}
