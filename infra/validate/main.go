package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const usage = `slvalidate — validate ScottyLabs registry manifests and recipes against their JSON Schemas.

Usage:
  slvalidate manifests [root]   validate every mcp-servers/**/manifest.yaml
  slvalidate recipes   [root]   validate every recipes/**/*.yaml
  slvalidate all       [root]   validate both

root defaults to the current directory. Exits non-zero if any file is invalid.`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, usage)
		return fmt.Errorf("missing command")
	}
	kind := args[0]
	root := "."
	if len(args) > 1 {
		root = args[1]
	}

	switch kind {
	case "manifests":
		return checkManifests(root)
	case "recipes":
		return checkRecipes(root)
	case "all":
		if err := checkManifests(root); err != nil {
			return err
		}
		return checkRecipes(root)
	case "checklist":
		return checkSecurity(root)
	case "-h", "--help", "help":
		fmt.Println(usage)
		return nil
	default:
		fmt.Fprintln(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", kind)
	}
}

func checkSecurity(root string) error {
	problems, err := RunChecklist(root)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "✗ "+p)
		}
		return fmt.Errorf("security checklist: %d problem(s)", len(problems))
	}
	fmt.Println("security checklist: passed")
	return nil
}

func checkManifests(root string) error {
	return checkKind(root, "manifest", filepath.Join(root, "mcp-servers", "manifest.schema.json"), findManifests)
}

func checkRecipes(root string) error {
	return checkKind(root, "recipe", filepath.Join(root, "recipes", "recipe.schema.json"), findRecipes)
}

// checkKind loads a schema, finds the files via find, validates them all, prints a per-file report,
// and returns an error if any file is invalid.
func checkKind(root, label, schemaPath string, find func(string) ([]string, error)) error {
	schema, err := loadSchema(schemaPath)
	if err != nil {
		return err
	}
	files, err := find(root)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Printf("%s: no files to validate\n", label)
		return nil
	}
	problems := validateAll(schema, files)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		return fmt.Errorf("%d of %d %s file(s) invalid", len(problems), len(files), label)
	}
	fmt.Printf("%s: %d file(s) valid\n", label, len(files))
	return nil
}
