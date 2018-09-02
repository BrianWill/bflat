package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Token struct {
	Type    TokenType
	Content string // the token itself, e.g. a number 3.7 is stored here as "3.7"
	Line    int    // first line is line 1
	Column  int    // first character of a line is in column 1
}

const thisWord = "me"
const propertyValueParam = "value"
const IndentSpaces = 4
const directoryPrefix = "bf."
const fileSuffix = ".bf"

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
	Name        ShortName
	Type        TypeAtom
	Value       Expression
	Annotations []AnnotationForm
}

type ImportDef struct {
	Line        int
	Column      int
	Namespace   NSNameFull
	Shortname   NSNameShort
	Exclusions  []string
	Aliases     map[string]string
	Annotations []AnnotationForm
}

type NamespaceDef struct {
	Line        int
	Column      int
	Name        NSNameFull
	Shortname   NSNameShort
	Annotations []AnnotationForm
}

type FuncDef struct {
	Line        int
	Column      int
	Name        ShortName
	ParamTypes  []TypeAtom
	ParamNames  []ShortName
	Return      TypeAtom
	Body        []Statement
	Annotations []AnnotationForm
}

type ClassDef struct {
	Line         int
	Column       int
	Type         TypeAtom
	AccessLevel  AccessLevel
	Supertypes   []TypeAtom
	Fields       []FieldDef
	Methods      []MethodDef
	Constructors []ConstructorDef
	Properties   []PropertyDef
	Annotations  []AnnotationForm
}

type ClassInfo struct {
	Name       ShortName
	Namespace  *Namespace
	Parent     *ClassInfo
	Fields     map[ShortName]FieldInfo
	Properties map[ShortName]PropertyInfo
	Methods    map[ShortName][]*CallableInfo
	Interfaces []*InterfaceInfo
	Params     []Type
}

type StructInfo struct {
	Name       ShortName
	Namespace  *Namespace
	Fields     map[ShortName]FieldInfo
	Properties map[ShortName]PropertyInfo
	Methods    map[ShortName][]*CallableInfo
	Interfaces []*InterfaceInfo
	Params     []Type
}

type InterfaceInfo struct {
	Name      ShortName
	Namespace *Namespace
	Parents   []*InterfaceInfo
	Methods   map[ShortName][]*CallableInfo // an interface can have overloads of the same method name
	Params    []Type
}

type GlobalInfo struct {
	Name      ShortName
	Namespace *Namespace
	Type      Type
}

// nil used to represent void or absence of type
type Type interface {
	Type() // do-nothing marker method
}

type BuiltinType struct {
	Name ShortName
}

type ArrayType struct {
	BaseType Type
}

func (t *ClassInfo) Type()     {}
func (t *StructInfo) Type()    {}
func (t *InterfaceInfo) Type() {}
func (t ArrayType) Type()      {}
func (t BuiltinType) Type()    {}

type CallableInfo struct {
	IsMethod   bool
	Namespace  *Namespace
	ParamNames []ShortName
	ParamTypes []Type
	Return     Type
	Static     Type // class or struct to which this method belongs
}

type Expression interface {
	Expression()
	GetLine() int
	GetColumn() int
}

type IndexingForm struct {
	Line   int
	Column int
	Args   []Expression
}

type CallForm struct {
	Line      int
	Column    int
	Name      ShortName
	Namespace NSNameShort
	Static    TypeAtom
	Args      []Expression
}

type TypeCallForm struct {
	Line     int
	Column   int
	Type     TypeAtom
	SizeFlag bool
	Args     []Expression
}

type VarExpression struct {
	Line      int
	Column    int
	Name      ShortName
	Namespace NSNameShort
}

func (a VarExpression) Expression()    {}
func (a ParsedNumberAtom) Expression() {}
func (a StringAtom) Expression()       {}
func (a IndexingForm) Expression()     {}
func (a CallForm) Expression()         {}
func (a TypeCallForm) Expression()     {}
func (a TypeAtom) Expression()         {}

func (a TypeAtom) GetLine() int {
	return a.Line
}
func (a TypeAtom) GetColumn() int {
	return a.Column
}

func (a IndexingForm) GetLine() int {
	return a.Line
}
func (a IndexingForm) GetColumn() int {
	return a.Column
}

func (a CallForm) GetLine() int {
	return a.Line
}
func (a CallForm) GetColumn() int {
	return a.Column
}

func (a TypeCallForm) GetLine() int {
	return a.Line
}
func (a TypeCallForm) GetColumn() int {
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
	CatchTypes  []TypeAtom // CaseValues and Casebodies are parallel
	CatchBodies [][]Statement
	FinallyBody []Statement
}

type TypeAtom struct {
	Line      int
	Column    int
	Name      ShortName
	Namespace NSNameShort
	Params    []TypeAtom
}

type Target interface {
	Target()
}

type AssignmentForm struct {
	Line   int
	Column int
	Target Target
	Value  Expression
}

func (a VarExpression) Target() {}
func (a IndexingForm) Target()  {}

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
	Target ShortName
	Type   TypeAtom
	Value  Expression
}

type AnnotationForm struct {
	Line      int
	Column    int
	Name      ShortName
	Class     string
	Namespace NSNameShort
	Args      []Expression
}

type FieldDef struct {
	Line        int
	Column      int
	Name        ShortName
	Type        TypeAtom
	AccessLevel AccessLevel
	Annotations []AnnotationForm
	Value       Expression
	IsStatic    bool
}

type FieldInfo struct {
	Name        ShortName
	Type        Type
	AccessLevel AccessLevel
	Static      Type
}

type StructDef struct {
	Line         int
	Column       int
	Type         TypeAtom
	AccessLevel  AccessLevel
	Interfaces   []TypeAtom
	Fields       []FieldDef
	Methods      []MethodDef
	Constructors []ConstructorDef
	Properties   []PropertyDef
	Annotations  []AnnotationForm
}

type InterfaceDef struct {
	Line              int
	Column            int
	Type              TypeAtom
	AccessLevel       AccessLevel
	ParentInterfaces  []TypeAtom
	MethodNames       []ShortName
	MethodParams      [][]TypeAtom
	MethodReturnTypes []TypeAtom
	MethodAnnotations [][]AnnotationForm
	Annotations       []AnnotationForm
}

type MethodDef struct {
	Line        int
	Column      int
	Name        ShortName
	ParamTypes  []TypeAtom
	ParamNames  []ShortName
	IsStatic    bool
	Return      TypeAtom
	Body        []Statement
	Annotations []AnnotationForm
}

type ConstructorDef struct {
	Line        int
	Column      int
	ParamTypes  []TypeAtom
	ParamNames  []ShortName
	Body        []Statement
	Annotations []AnnotationForm
}

type PropertyDef struct {
	Line        int
	Column      int
	Name        ShortName
	Type        TypeAtom
	IsManual    bool
	IsStatic    bool
	GetBody     []Statement
	SetBody     []Statement
	HasGetter   bool
	HasSetter   bool
	Annotations []AnnotationForm
	AccessLevel AccessLevel
}

type PropertyInfo struct {
	Name        ShortName
	Type        Type
	HasGetter   bool
	HasSetter   bool
	AccessLevel AccessLevel
	Static      Type
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
	Classes    []ClassDef
	Structs    []StructDef
	Interfaces []InterfaceDef
	Funcs      []FuncDef
	Globals    []GlobalDef
	Imports    []ImportDef
}

const GlobalsClass = "_Globals"
const FuncsClass = "_Funcs"

type NSNameFull string
type NSNameShort string // for namespace names with dots, the part after the last dot (otherwise same as NSNameFull)
type NSNameCS string
type ShortName string // unqualified name

type Namespace struct {
	Name      NSNameFull
	ShortName NSNameShort
	CSName    NSNameCS
	Imports   map[NSNameShort]*Namespace

	Classes      map[ShortName]*ClassInfo
	Structs      map[ShortName]*StructInfo
	Interfaces   map[ShortName]*InterfaceInfo
	Constructors map[ShortName][]*CallableInfo
	Globals      map[ShortName]*GlobalInfo
	Funcs        map[ShortName][]*CallableInfo
	Methods      map[ShortName][]*CallableInfo
}

type TypeInfo interface {
	TypeInfo()
}

func (a *ClassInfo) TypeInfo()     {}
func (a *StructInfo) TypeInfo()    {}
func (a *InterfaceInfo) TypeInfo() {}

const StrLengthWord = "len"

var AnyType = BuiltinType{
	Name: "Any",
}

var StrType = BuiltinType{
	Name: "Str",
}

var BoolType = BuiltinType{
	Name: "Bool",
}

var IntType = BuiltinType{
	Name: "I",
}

var LongType = BuiltinType{
	Name: "II",
}

var FloatType = BuiltinType{
	Name: "F",
}

var DoubleType = BuiltinType{
	Name: "FF",
}

var ByteType = BuiltinType{
	Name: "B",
}

var SignedByteType = BuiltinType{
	Name: "SB",
}

var OperatorSymbols = map[ShortName]string{
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
	debugMode := true
	var directory string
	var namespace NSNameFull
	if debugMode {
		directory = "."
		namespace = "something.test"
	} else {
		if len(os.Args) < 2 {
			fmt.Println("Must specify a namespace (short name) and directory.")
			return
		}
		namespace = NSNameFull(os.Args[1]) // expecting the full namespace name

		directory = "."
		if len(os.Args) == 3 {
			directory = os.Args[2]
		}

		if len(os.Args) > 3 {
			fmt.Println("Too many program arguments. Expecting 2 program arguments at most.")
			return
		}
	}

	start := time.Now()

	nsFileLookup := map[NSNameFull][]string{}
	err := buildNamespaceFileLookup(directory, nsFileLookup)
	if err != nil {
		fmt.Println(errors.New("Cannot find or read files of namespace: " + string(namespace) + err.Error()))
		return
	}
	err = compileNamespace(namespace, nsFileLookup, map[NSNameFull]*Namespace{})
	if err != nil {
		fmt.Println(err)
		return
	}

	debug("Time: ", time.Since(start))
}

func msg(line int, column int, s string) error {
	return errors.New("Line " + strconv.Itoa(line) + ", column " +
		strconv.Itoa(column) + ": " + s)
}

// namespace is expected on first line with no leading whitespace
func fileReadNamespace(file string) (NSNameFull, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	data = append(data, '\n', '\n')
	firstNewline := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			firstNewline = i
			break
		}
		if data[i] == '\r' && data[i+1] == '\n' {
			firstNewline = i
			break
		}
	}
	str := strings.Trim(string(data[:firstNewline]), " \t")
	if !isFullNamespace(str) {
		return "", errors.New("First line of source file does not begin with properly formed namespace name.")
	}
	return NSNameFull(str), nil
}

func isFullNamespace(ns string) bool {
	data := []byte(ns)
	componentStart := true
	for i := 0; i < len(data); i++ {
		if data[i] == '.' {
			componentStart = false
			continue
		}
		if componentStart {
			// is not lowercase alpha
			if data[i] < 'a' && data[i] > 'z' {
				return false
			}
			componentStart = false
		} else {
			if data[i] == ' ' || data[i] == '\r' || data[i] == '\t' || data[i] == '\n' {
				return true
			}
			if data[i] >= 'a' && data[i] <= 'z' {
				continue
			}
			if data[i] >= 'A' && data[i] <= 'Z' {
				continue
			}
			if data[i] >= '0' && data[i] <= '9' {
				continue
			}
			if data[i] == '.' {
				componentStart = true
				continue
			}
		}
	}
	return true
}

func buildNamespaceFileLookup(dir string, nsFileLookup map[NSNameFull][]string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	nsNames := map[NSNameShort]NSNameFull{}
	for _, file := range files {
		name := file.Name()
		if !file.IsDir() && strings.Index(name, "_") == -1 && strings.HasSuffix(name, fileSuffix) {
			if len(name) == len(fileSuffix) {
				continue
			}
			nsName, err := fileReadNamespace(name)
			if err != nil {
				return err
			}
			shortName := getNSNameShort(nsName)
			if name[:len(name)-len(fileSuffix)] != string(shortName) {
				return errors.New("Source file has wrong name for its declared namespace: " + name)
			}
			if len(nsFileLookup[nsName]) != 0 {
				return errors.New("Found more than one set of source files for namespace: " + string(nsName))
			}
			nsFileLookup[nsName] = append(nsFileLookup[nsName], dir+"/"+name)
			nsNames[shortName] = nsName
		}
	}

	for _, file := range files {
		name := file.Name()
		idx := strings.Index(name, "_")
		if !file.IsDir() && idx != -1 && strings.HasSuffix(name, fileSuffix) {
			if nsName, ok := nsNames[NSNameShort(name[idx:])]; ok {
				nsFileLookup[nsName] = append(nsFileLookup[nsName], dir+"/"+name)
			} else {
				return errors.New("Source file has no main source file of matching name: " + name)
			}
		}
	}

	// recurse into directories starting with special directory prefix
	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), directoryPrefix) {
			err := buildNamespaceFileLookup(dir+"/"+file.Name(), nsFileLookup)
			if err != nil {
				return nil
			}
		}
	}
	return nil
}
