package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"
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
	"sub", // sub with one operand negates that operand
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
	"bnot",
	"ref",
	"dr", // deref
	"inc",
	"dec",
	"cat", // concat
	"shl", // shift left
	"shr", // shift right
	"ife", // ?: ternary operator
	"cast",
	"istype", // "is " operator
	"astype", // "as" operator
	"typeof", // get Type
	"sizeof",
	"default",
	// assignment operators
	"as",
	"asadd",
	"asmul",
	"asdiv",
	"assub",
	"asmod",
	"asnot",
	"asand",
	"asor",
	"asbor",
	"asband",
	"asshr",
	"asshl",
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
	'"',
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
	Type        DataType
	Value       Expression
	Annotations []AnnotationForm
}

type NamespaceDef struct {
	Line        int
	Column      int
	Name        string
	Annotations []AnnotationForm
}

type FuncDef struct {
	Line        int
	Column      int
	Name        string
	ParamTypes  []DataType
	ParamNames  []string
	ReturnType  DataType
	Body        []Statement
	Annotations []AnnotationForm
}

type ClassDef struct {
	Line         int
	Column       int
	Type         DataType
	AccessLevel  AccessLevel
	Supertypes   []DataType
	Fields       []FieldDef
	Methods      []MethodDef
	Constructors []ConstructorDef
	Properties   []PropertyDef
	Annotations  []AnnotationForm
}

type ClassInfo struct {
	Name       string
	Namespace  string
	Parent     *ClassInfo
	Interfaces []*InterfaceInfo
}

type StructInfo struct {
	Name       string
	Namespace  string
	Interfaces []*InterfaceInfo
}

type InterfaceInfo struct {
	Name      string
	Namespace string
	Parents   []*InterfaceInfo
}

type GlobalInfo struct {
	Name      string
	Namespace string
}

type CallableInfo struct {
	IsMethod   bool
	ParamNames []string
	ParamTypes []DataType
	ReturnType DataType
}

type Expression interface {
	Expression()
	GetLine() int
	GetColumn() int
}

type CallForm struct {
	Line      int
	Column    int
	Name      string
	Namespace string
	Args      []Expression
}

type VarExpression struct {
	Line      int
	Column    int
	Name      string
	Namespace string
}

func (a VarExpression) Expression()    {}
func (a ParsedNumberAtom) Expression() {}
func (a StringAtom) Expression()       {}
func (a CallForm) Expression()         {}
func (a DataType) Expression()         {}

func (a DataType) GetLine() int {
	return a.Line
}
func (a DataType) GetColumn() int {
	return a.Column
}

func (a CallForm) GetLine() int {
	return a.Line
}
func (a CallForm) GetColumn() int {
	return a.Column
}

func (a StringAtom) GetLine() int {
	return a.Line
}
func (a StringAtom) GetColumn() int {
	return a.Column
}

func (a ParsedNumberAtom) GetLine() int {
	return a.Line
}
func (a ParsedNumberAtom) GetColumn() int {
	return a.Column
}

func (a VarExpression) GetLine() int {
	return a.Line
}
func (a VarExpression) GetColumn() int {
	return a.Column
}

func (a ParenList) GetLine() int {
	return a.Line
}
func (a ParenList) GetColumn() int {
	return a.Column
}

func (a SquareList) GetLine() int {
	return a.Line
}
func (a SquareList) GetColumn() int {
	return a.Column
}

func (a CurlyList) GetLine() int {
	return a.Line
}
func (a CurlyList) GetColumn() int {
	return a.Column
}

func (a AngleList) GetLine() int {
	return a.Line
}
func (a AngleList) GetColumn() int {
	return a.Column
}

func (a Symbol) GetLine() int {
	return a.Line
}
func (a Symbol) GetColumn() int {
	return a.Column
}

func (a SigilAtom) GetLine() int {
	return a.Line
}
func (a SigilAtom) GetColumn() int {
	return a.Column
}

func (a AtomChain) GetLine() int {
	return a.Line
}
func (a AtomChain) GetColumn() int {
	return a.Column
}

func (a NumberAtom) GetLine() int {
	return a.Line
}
func (a NumberAtom) GetColumn() int {
	return a.Column
}

func (a CallForm) Statement()       {}
func (a AssignmentForm) Statement() {}
func (a IfForm) Statement()         {}
func (a SwitchForm) Statement()     {}
func (a VarForm) Statement()        {}
func (a ReturnForm) Statement()     {}
func (a ForForm) Statement()        {}
func (a TryForm) Statement()        {}
func (a ThrowForm) Statement()      {}
func (a ContinueForm) Statement()   {}
func (a BreakForm) Statement()      {}

type IfForm struct {
	Line       int
	Column     int
	Condition  Expression
	Body       []Statement
	ElifConds  []Expression // ElifConds and ElifBodies are parallel
	ElifBodies [][]Statement
	ElseBody   []Statement
}

type SwitchForm struct {
	Line        int
	Column      int
	Value       Expression
	CaseValues  []Expression // CaseValues and Casebodies are parallel
	CaseBodies  [][]Statement
	DefaultBody []Statement
}

type TryForm struct {
	Line        int
	Column      int
	Body        []Statement
	CatchTypes  []DataType // CaseValues and Casebodies are parallel
	CatchBodies [][]Statement
	FinallyBody []Statement
}

type DataType struct {
	Line       int
	Column     int
	Name       string
	TypeParams []DataType
	Namespace  string
}

type AssignmentForm struct {
	Line   int
	Column int
	Target VarExpression
	Value  Expression
}

type ReturnForm struct {
	Line   int
	Column int
	Value  Expression
}

type ThrowForm struct {
	Line   int
	Column int
	Value  Expression
}

type BreakForm struct {
	Line   int
	Column int
	Label  string
}

type ContinueForm struct {
	Line   int
	Column int
	Label  string
}

type ForForm struct {
	Line      int
	Column    int
	Condition Expression
	Body      []Statement
}

type VarForm struct {
	Line   int
	Column int
	Target string
	Type   DataType
	Value  Expression
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
	Type        DataType
	AccessLevel AccessLevel
	Annotations []AnnotationForm
	Value       Expression
}

type StructDef struct {
	Line         int
	Column       int
	Type         DataType
	AccessLevel  AccessLevel
	Interfaces   []DataType
	Fields       []FieldDef
	Methods      []MethodDef
	Constructors []ConstructorDef
	Properties   []PropertyDef
	Annotations  []AnnotationForm
}

type InterfaceDef struct {
	Line              int
	Column            int
	Type              DataType
	AccessLevel       AccessLevel
	ParentInterfaces  []DataType
	MethodNames       []string
	MethodParams      [][]DataType
	MethodReturnTypes []DataType
	MethodAnnotations [][]AnnotationForm
	Annotations       []AnnotationForm
}

type MethodDef struct {
	Line        int
	Column      int
	Name        string
	ParamTypes  []DataType
	ParamNames  []string
	ReturnType  DataType
	Body        []Statement
	Annotations []AnnotationForm
}

type ConstructorDef struct {
	Line        int
	Column      int
	ParamTypes  []DataType
	ParamNames  []string
	Body        []Statement
	Annotations []AnnotationForm
}

type PropertyDef struct {
	Line        int
	Column      int
	Name        string
	Type        DataType
	GetBody     []Statement
	SetBody     []Statement
	Annotations []AnnotationForm
}

type Atom interface {
	Atom()
	GetLine() int
	GetColumn() int
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
	IntegerPart    string
	FractionalPart string
	Line           int
	Column         int
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
	Namespace  NamespaceDef
	Classes    []ClassDef
	Structs    []StructDef
	Interfaces []InterfaceDef
	Funcs      []FuncDef
	Globals    []GlobalDef
}

const GlobalsClass = "_Globals"
const FuncsClass = "_Funcs"

type Namespace struct {
	Name       string
	Classes    map[string]*ClassInfo
	Structs    map[string]*StructInfo
	Interfaces map[string]*InterfaceInfo
	Funcs      map[string][]*CallableInfo
	Methods    map[string][]*CallableInfo
	Globals    map[string]*GlobalInfo
	FullNames  map[string]string // unqualifieid name -> fully qualified name
}

var StrType = DataType{
	Name: "Str",
}

var BoolType = DataType{
	Name: "Bool",
}

var IntType = DataType{
	Name: "I",
}

var LongIntType = DataType{
	Name: "II",
}

var FloatType = DataType{
	Name: "F",
}

var DoubleType = DataType{
	Name: "FF",
}

var ByteType = DataType{
	Name: "Byte",
}

var SignedByteType = DataType{
	Name: "SByte",
}

var OperatorSymbols = map[string]string{
	"add":  " + ",
	"sub":  " - ",
	"mul":  " * ",
	"div":  " / ",
	"mod":  " % ",
	"and":  " && ",
	"or":   " || ",
	"band": " & ",
	"bor":  " | ",
	"bxor": " ^ ",
	"cat":  " + ",
	"eq":   " == ",
	"neq":  " != ",
	"gt":   " > ",
	"lt":   " < ",
	"gte":  " >= ",
	"lte":  " <= ",
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

	code, err := compile(topDefs)
	if err != nil {
		fmt.Println(err)
		return
	}

	debug("Time: ", time.Since(start))

	outputFilename := inputFilename + ".cs"
	err = ioutil.WriteFile(outputFilename, []byte(code), os.ModePerm)
	if err != nil {
		fmt.Println(err)
		return
	}

	// todo: use https://github.com/dotnet/codeformatter to format the code
}

func msg(line int, column int, s string) error {
	return errors.New("Line " + strconv.Itoa(line) + ", column " +
		strconv.Itoa(column) + ": " + s)
}
