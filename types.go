package main

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
)

type Token struct {
	Type    TokenType
	Content string // the token itself, e.g. a number 3.7 is stored here as "3.7"
	Line    int    // first line is line 1
	Column  int    // first character of a line is in column 1
}

const IndentSpaces = 4

// not including nil, true, false, and the operators
var reservedWords = []string{
	"func",
	"efunc",
	"meth",
	"emeth",
	"struct",
	"estruct",
	"interface",
	"einterface",
	"var",
	"evar",
	"const",
	"econst",
	"if",
	"for",
	"while",
	"switch",
	"select",
	"return",
}

var operatorWords = []string{
	"add",
	"sub",
	"mul",
	"div",
	"mod",
	"eq",
	"neq",
	"not",
	"lt",
	"gt",
	"lte",
	"gte",
	"len",
	"or",
	"and",
	"xor",
	"bor",
	"band",
	"bxor",
	"inc",
	"dec",
	// assignment operators
	"as",
	"asadd",
	"asmul",
	"asdiv",
	"assub",
	"asinc",
	"asdec",
}

var sigils = []rune{
	'.',
	',',
	'~',
	'!',
	'@',
	'#',
	'$',
	'%',
	'^',
	'&',
	'*',
	'-',
	'+',
	'=',
	'/',
	'\\',
	':',
	';',
	'\'',
}

var debug = fmt.Println // alias for debug printing
var itoa = strconv.Itoa

type TokenType uint

// token types
const (
	Word TokenType = iota
	Newline
	Spaces
	OpenParen
	CloseParen
	OpenSquare
	CloseSquare
	NoClose // passed to readAtom to indicate that we're not expecting an end delimiter
	OpenCurly
	CloseCurly
	OpenAngle
	CloseAngle
	NumberLiteral
	StringLiteral
	Sigil
)

type Statement interface {
	Statement()
}

type GlobalDef struct {
	Line        int
	Column      int
	Name        string
	Type        []TypeSpec
	Val         Expression
	Annotations []AnnotationForm
}

type FuncDef struct {
	Line        int
	Column      int
	Name        string
	Params      []ParamDef
	ReturnType  TypeSpec
	Body        []Statement
	Annotations []AnnotationForm
}

type ClassDef struct {
	Line         int
	Column       int
	Name         string
	AccessLevel  AccessLevel
	Parent       string
	Interfaces   []string
	Fields       []FieldDef
	Methods      []MethodDef
	Constructors []ConstructorDef
	Properties   []PropertyDef
	Annotations  []AnnotationForm
}

type Expression interface {
	Expression()
}

type CallForm struct {
	Line      int
	Column    int
	Name      string
	Class     string
	Namespace []string
	Args      []Expression
}

type VarExpression struct {
	Line      int
	Column    int
	Name      string
	Class     string
	Namespace []string
}

func (a VarExpression) Expression()    {}
func (a ParsedNumberAtom) Expression() {}
func (a StringAtom) Expression()       {}
func (a CallForm) Expression()         {}

func (a CallForm) Statement() {}
func (a IfForm) Statement()   {}

type IfForm struct {
	Line       int
	Column     int
	Condition  Expression
	Body       []Statement
	ElifConds  []Expression // ElifConds and ElifBodies are parallel
	ElifBodies [][]Statement
	ElseBody   []Statement
}

type ParamDef struct {
	Name string
	Type TypeSpec
}

type TypeSpec struct {
	Line       int
	Column     int
	Name       string
	TypeParams []TypeSpec
	Namespace  []string
}

type AssignmentForm struct {
}

type AnnotationForm struct {
	Line      int
	Column    int
	Name      string
	Class     string
	Namespace []string
	Args      []Expression
}

type FieldDef struct {
	Line        int
	Column      int
	Name        string
	Type        TypeSpec
	AccessLevel AccessLevel
	Annotations []AnnotationForm
	Value       Expression
}

type StructDef struct {
	Line         int
	Column       int
	Name         string
	AccessLevel  AccessLevel
	Interfaces   []string
	Fields       []FieldDef
	Methods      []MethodDef
	Constructors []ConstructorDef
	Properties   []PropertyDef
	Annotations  []AnnotationForm
}

type MethodDef struct {
}

type ConstructorDef struct {
}

type PropertyDef struct {
}

type Atom interface {
	Atom()
}

type AccessLevel int

const (
	PublicAccess AccessLevel = iota
	PrivateAccess
	ProtectedAccess
)

type ParenList struct {
	Atoms  []Atom
	Line   int
	Column int
}

type SquareList struct {
	Atoms  []Atom
	Line   int
	Column int
}

type CurlyList struct {
	Atoms  []Atom
	Line   int
	Column int
}

type AngleList struct {
	Atoms  []Atom
	Line   int
	Column int
}

type AtomChain struct {
	Atoms  []Atom
	Line   int
	Column int
}

type Symbol struct {
	Content string
	Line    int
	Column  int
}

type NumberAtom struct {
	Content string
	Line    int
	Column  int
}

type ParsedNumberAtom struct {
	Value  string
	Line   int
	Column int
}

type StringAtom struct {
	Content string // includes enclosing quote marks
	Line    int
	Column  int
}

type SigilAtom struct {
	Content string
	Line    int
	Column  int
}

func (a ParenList) Atom()  {}
func (a SquareList) Atom() {}
func (a CurlyList) Atom()  {}
func (a AngleList) Atom()  {}
func (a AtomChain) Atom()  {}
func (a Symbol) Atom()     {}
func (a NumberAtom) Atom() {}
func (a StringAtom) Atom() {}
func (a SigilAtom) Atom()  {}

type TopDefs struct {
	classes []ClassDef
	structs []StructDef
	funcs   []FuncDef
	globals []GlobalDef
}

func main() {
	// if len(os.Args) != 2 {
	//     fmt.Println("Must specify a .bf file.")
	//     return
	// }
	// inputFilename := os.Args[1]
	inputFilename := "example.bf"

	data, err := ioutil.ReadFile(inputFilename)
	if err != nil {
		fmt.Println(err)
		return
	}

	data = append(data, '\n')

	start := time.Now()
	tokens, err := lex(string(data))
	if err != nil {
		fmt.Println(err)
		return
	}

	atoms, err := read(tokens)
	if err != nil {
		fmt.Println(err)
		return
	}

	topDefs, err := parse(atoms)
	if err != nil {
		fmt.Println(err)
		return

	}

	fmt.Println("TOP DEFS")
	spew.Dump(topDefs)

	debug("Time: ", time.Since(start))
	return

	// topDefs, err := parse(atoms)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// code, err := compile(topDefs)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// outputFilename := inputFilename + ".go"
	// err = ioutil.WriteFile(outputFilename, []byte(code), os.ModePerm)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// err = exec.Command("go", "fmt", outputFilename).Run()
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// cmd := exec.Command("go", "run", outputFilename)
	// cmd.Stdin = os.Stdin
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// err = cmd.Run()
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
}
