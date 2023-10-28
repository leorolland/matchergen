//nolint:unused
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

type Getter struct {
	name       string
	returnType string
}

func (g *Getter) nameAsPrivate() string {
	return string(unicode.ToLower(rune(g.name[0]))) + string(g.name[1:])
}

// File holds a single parsed file and associated data.
type File struct {
	pkg  *Package  // Package to which this file belongs.
	file *ast.File // Parsed AST.
	// These fields are reset for each type being generated.
	typeName string
	getters  []Getter
}

func (f *File) genGetters(node ast.Node) bool {
	fun, isFunc := node.(*ast.FuncDecl)
	if !isFunc {
		return true
	}
	if !fun.Name.IsExported() {
		return true
	}
	if fun.Recv == nil || len(fun.Recv.List) != 1 {
		return true
	}

	var structName string
	receiver := fun.Recv.List[0]
	ptr, isPtr := receiver.Type.(*ast.StarExpr)
	if isPtr {
		structName = ptr.X.(*ast.Ident).Name
	} else {
		// TODO: handle non ptr receiver
	}

	if structName != f.typeName {
		return false
	}

	if len(fun.Type.Results.List) != 1 {
		return false
	}

	f.getters = append(f.getters, Getter{
		name:       fun.Name.String(),
		returnType: types.ExprString(fun.Type.Results.List[0].Type),
	})

	return false
}

type Package struct {
	name  string
	defs  map[*ast.Ident]types.Object
	files []*File
}

type Generator struct {
	buf bytes.Buffer // Accumulated output.
	pkg *Package     // Package we are scanning.
}

func (g *Generator) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&g.buf, format, args...)
}

// parsePackage analyzes the single package constructed from the patterns and tags.
// parsePackage exits if there is an error.
func (g *Generator) parsePackage(patterns []string, tags []string) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		// TODO: Need to think about constants in test files. Maybe write type_string_test.go
		// in a separate pass? For later.
		Tests:      false,
		BuildFlags: []string{fmt.Sprintf("-tags=%s", strings.Join(tags, " "))},
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		log.Fatal(err)
	}
	if len(pkgs) != 1 {
		log.Fatalf("error: %d packages matching %v", len(pkgs), strings.Join(patterns, " "))
	}
	g.addPackage(pkgs[0])
}

// addPackage adds a type checked Package and its syntax files to the generator.
func (g *Generator) addPackage(pkg *packages.Package) {
	fmt.Printf("found package %s\n", pkg)
	g.pkg = &Package{
		name:  pkg.Name,
		defs:  pkg.TypesInfo.Defs,
		files: make([]*File, len(pkg.Syntax)),
	}

	for i, file := range pkg.Syntax {
		g.pkg.files[i] = &File{
			file: file,
			pkg:  g.pkg,
		}
	}
}

func (g *Generator) generate(typeName string) {
	getters := make([]Getter, 0)
	for _, file := range g.pkg.files {
		file.typeName = typeName
		file.getters = nil
		if file.file != nil {
			ast.Inspect(file.file, file.genGetters)
			getters = append(getters, file.getters...)
		}
	}
	fmt.Printf("found getters for struct %s: %s\n", typeName, getters)

	matcherName := fmt.Sprintf("%sMatcher", typeName)
	matcherReceiverName := string(unicode.ToLower(rune(matcherName[0])))

	castedVarName := string(unicode.ToLower(rune(typeName[0]))) + typeName[1:]

	g.Printf("package %s\n\n", "modeltest") // TODO: make it configurable
	g.Printf("type %s struct {\n", matcherName)
	g.Printf("\tcomparator *comparator\n\n")
	for _, getter := range getters {
		g.Printf("\t%s %s\n", getter.nameAsPrivate(), getter.returnType)
	}
	g.Printf("}\n\n")

	g.Printf("func New%s(", matcherName)
	for _, getter := range getters {
		g.Printf("%s %s, ", getter.nameAsPrivate(), getter.returnType)
	}
	g.Printf(") *%s{\n", matcherName)
	g.Printf("\treturn &%s{\n", matcherName)
	for _, getter := range getters {
		g.Printf("\t%s: %s,\n", getter.nameAsPrivate(), getter.nameAsPrivate())
	}
	g.Printf("\t}\n")
	g.Printf("}\n\n")

	g.Printf("func (%s *%s)Matches(arg interface{}) bool {\n", matcherReceiverName, matcherName)
	g.Printf("\t%s.comparator = &comparator{}\n\n", matcherReceiverName)
	g.Printf("\t%s, ok := arg.(*model.%s)\n", castedVarName, typeName)
	g.Printf("\tif !ok {\n")
	g.Printf("\t\t%s.comparator.equal(\"type\", \"*%s.%s\", fmt.Sprintf(\"%%T\", arg))\n",
		matcherReceiverName, g.pkg.name, typeName,
	)
	g.Printf("\t\treturn %s.comparator.matches()\n", matcherReceiverName)
	g.Printf("\t}\n\n")
	for _, getter := range getters {
		g.Printf("\t%s.comparator.equal(%q, %s.%s, %s.%s())\n",
			matcherReceiverName, getter.nameAsPrivate(), matcherReceiverName,
			getter.nameAsPrivate(), castedVarName, getter.name,
		)
	}
	g.Printf("\n\treturn %s.comparator.matches()\n", matcherReceiverName)
	g.Printf("}\n\n")
	g.Printf("func (%s *%s)Got(got interface{}) string {\n", matcherReceiverName, matcherName)
	g.Printf("\treturn getDiff(%s.comparator.got)", matcherReceiverName)
	g.Printf("}\n\n")
	g.Printf("func (%s *%s)String() string {\n", matcherReceiverName, matcherName)
	g.Printf("\treturn getValue(%s.comparator.wanted)", matcherReceiverName)
	g.Printf("}\n\n")
}

// format returns the gofmt-ed contents of the Generator's buffer.
func (g *Generator) format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}

var (
	typeNames = flag.String("type", "", "comma-separated list of type names; must be set")
	output    = flag.String("output", "", "output file name; default srcdir/<type>_matcher.go")
	buildTags = flag.String("tags", "", "comma-separated list of build tags to apply")
)

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of matchergen:\n")
	fmt.Fprintf(os.Stderr, "\tmatchergen [flags] -type T [directory]\n")
	fmt.Fprintf(os.Stderr, "\tmatchergen [flags] -type T files... # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("matchergen: ")
	flag.Usage = Usage
	flag.Parse()
	if len(*typeNames) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	types := strings.Split(*typeNames, ",")
	var tags []string
	if len(*buildTags) > 0 {
		tags = strings.Split(*buildTags, ",")
	}

	// We accept either one directory or a list of files. Which do we have?
	args := flag.Args()
	if len(args) == 0 {
		// Default: process whole package in current directory.
		args = []string{"."}
	}

	// Parse the package once.
	g := Generator{}

	g.parsePackage(args, tags)
	// Run generate for each type.
	for _, typeName := range types {
		g.generate(typeName)
	}

	src := g.format()

	var dir string
	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
	} else {
		if len(tags) != 0 {
			log.Fatal("-tags option applies only to directories, not when files are specified")
		}
		dir = filepath.Dir(args[0])
	}

	// Write to file.
	outputName := *output
	if outputName == "" {
		baseName := fmt.Sprintf("%s_matcher.go", types[0])
		outputName = filepath.Join(dir, strings.ToLower(baseName))
	}

	err := os.WriteFile(outputName, src, 0644)
	if err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return info.IsDir()
}
