package main

import (
	"errors"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
)

func codeGen(topDefs *TopDefs, ns *Namespace) (string, error) {
	code := "namespace " + string(ns.CSName) + " {\n\n"

	c, err := compileGlobals(topDefs.Globals, ns, "\t")
	if err != nil {
		return "", err
	}
	code += c

	code += "public class " + FuncsClass + " {\n"
	for _, fn := range topDefs.Funcs {
		c, err := compileFunc(fn, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c
	}
	code += "}\n\n"

	for _, classDef := range topDefs.Classes {
		c, err := compileClass(classDef, ns, "")
		if err != nil {
			return "", err
		}
		code += c
	}

	for _, interfaceDef := range topDefs.Interfaces {
		c, err := compileInterface(interfaceDef, ns, "")
		if err != nil {
			return "", err
		}
		code += c
	}

	code += "}"

	return code, nil
}

func getNSNameShort(namespace NSNameFull) NSNameShort {
	return NSNameShort(namespace[strings.LastIndex(string(namespace), ".")+1:])
}

func compileNamespace(namespace NSNameFull, nsFileLookup map[NSNameFull][]string, namespaces map[NSNameFull]*Namespace) error {
	if _, ok := namespaces[namespace]; ok {
		return errors.New("Recursive import depedency: " + string(namespace))
	}

	topDefs := &TopDefs{
		Classes: []ClassDef{},
		Structs: []StructDef{},
		Funcs:   []FuncDef{},
		Globals: []GlobalDef{},
		Imports: []ImportDef{},
	}

	if len(nsFileLookup[namespace]) == 0 {
		return errors.New("No source files found for namespace: " + string(namespace))
	}

	for i, file := range nsFileLookup[namespace] {

		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		data = append(data, '\n', '\n', '\n', '\n')

		// find first blank line
		var blankIdx int
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' && data[i+1] == '\n' {
				blankIdx = i
				break
			}
			if data[i] == '\r' && data[i+1] == '\n' && data[i+2] == '\r' && data[i+3] == '\n' {
				blankIdx = i
				break
			}
		}
		if blankIdx == 0 {
			return errors.New("Expecting blank line in file after namespace name: " + file)
		}

		data = data[blankIdx:]

		tokens, err := lex(string(data))
		if err != nil {
			return err
		}

		atoms, err := read(tokens)
		if err != nil {
			return err
		}

		err = parse(atoms, topDefs, i == 0)
		if err != nil {
			return err
		}

	}

	ns, err := createNamespace(topDefs, namespace, nsFileLookup, namespaces)
	if err != nil {
		return err
	}
	namespaces[namespace] = ns

	code, err := codeGen(topDefs, ns)
	if err != nil {
		return err
	}

	outputFilename := string(namespace) + ".cs"
	err = ioutil.WriteFile(outputFilename, []byte(code), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

// returns indexed type and true if an indexable type
func IsIndexableType(t Type) (Type, bool) {
	switch t := t.(type) {
	case ArrayType:
		return t.BaseType, true
	case *ClassInfo:
		// todo
		return nil, false
	case *StructInfo:
		// todo
		return nil, false
	case *InterfaceInfo:
		return nil, false
	case BuiltinType:
		return nil, false
	default:
		return nil, false
	}
}

// only first param matters,
// assumes len(sigs) >= 2
// we can assume that all sigs have at least one param
func ClosestMatchingSignature(sigs []*CallableInfo, ns *Namespace, line int, column int) (*CallableInfo, error) {
	funcCalls := []*CallableInfo{}
	interfaceCalls := []*CallableInfo{}
	classCalls := []*CallableInfo{}
	structCalls := []*CallableInfo{}
	firstParamClass := []*ClassInfo{}
	allFuncs := true
	for _, sig := range sigs {
		if sig.IsMethod {
			allFuncs = false
		} else {
			funcCalls = append(funcCalls, sig)
			continue
		}
		switch ti := sig.ParamTypes[0].(type) {
		case *InterfaceInfo:
			interfaceCalls = append(interfaceCalls, sig)
		case *ClassInfo:
			classCalls = append(classCalls, sig)
			firstParamClass = append(firstParamClass, ti)
		case *StructInfo:
			structCalls = append(structCalls, sig)
		}
	}
	if allFuncs {
		return nil, msg(line, column, "Call ambiguously matches multiple functions.")
	} else if len(funcCalls) > 0 {
		return nil, msg(line, column, "Call ambiguously matches both one or more functions and one or more methods.")
	}
	if len(classCalls) > 0 {
		// we can assume all classes are related
		// we want to find lowest in hierarchy
		winnerClass := firstParamClass[0]
		winnerIdx := 0
		for i := 1; i < len(classCalls); i++ {
			other := firstParamClass[i]
			if winnerClass == other {
				return nil, msg(line, column, "Call ambiguously matches multiple overloads of method.")
			}
			if !IsDescendent(winnerClass, other) {
				winnerClass = other
				winnerIdx = i
			}
		}
		return classCalls[winnerIdx], nil
	}
	if len(structCalls) == 1 {
		return structCalls[0], nil
	}
	return nil, msg(line, column, "Call ambiguously matches multiple methods.")
}

func compileExpression(expr Expression, ns *Namespace, expectedType Type,
	locals map[ShortName]Type) (code string, dt Type, err error) {
	switch expr := expr.(type) {
	case VarExpression:
		global := ns.GetGlobal(expr.Name, expr.Namespace)
		if global != nil {
			dt = global.Type
			code = string(global.Namespace.CSName) + "." + GlobalsClass + "." + string(global.Name)
			return
		} else {
			var ok bool
			dt, ok = locals[expr.Name]
			if !ok {
				return "", nil, msg(expr.Line, expr.Column, "No variable found of name: "+string(expr.Name))
			}
			if expr.Name == thisWord {
				code = "this"
			} else {
				code = string(expr.Name)
			}
			return
		}
	case ParsedNumberAtom:
		if expectedType == nil {
			if expr.FractionalPart == "" {
				val, err := strconv.Atoi(expr.IntegerPart)
				if err != nil {
					return "", nil, msg(expr.Line, expr.Column, "Expecting Int number literal.")
				}
				if val > math.MaxInt32 || val < math.MinInt32 {
					return "", nil, msg(expr.Line, expr.Column, "Expecting Int number literal but magnitude is too great.")
				}
				code = expr.IntegerPart
				dt = IntType
			} else {
				code = "(double) " + expr.IntegerPart + "." + expr.FractionalPart
				dt = DoubleType
			}
		} else if expectedType == IntType {
			if expr.FractionalPart != "" {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Int literal, but got floating-point.")
			}
			val, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Int number literal.")
			}
			if val > math.MaxInt32 || val < math.MinInt32 {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Int number literal but magnitude is too great.")
			}
			code = expr.IntegerPart
			dt = IntType
		} else if expectedType == LongType {
			if expr.FractionalPart != "" {
				return "", nil, msg(expr.Line, expr.Column, "Expecting II literal, but got floating-point.")
			}
			_, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", nil, msg(expr.Line, expr.Column, "Expecting II number literal, but value is not integer or out of range.")
			}
			code = "(long) " + expr.IntegerPart
			dt = LongType
		} else if expectedType == FloatType {
			if expr.FractionalPart == "" {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Float literal, but got integer.")
			}
			code = "(float) " + expr.IntegerPart + "." + expr.FractionalPart
			dt = FloatType
		} else if expectedType == DoubleType {
			// todo check within double range
			code = "(double) " + expr.IntegerPart + "." + expr.FractionalPart
			dt = DoubleType
		} else if expectedType == ByteType {
			if expr.FractionalPart != "" {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Byte literal, but got floating-point.")
			}
			val, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Byte number literal.")
			}
			if val > math.MaxUint8 || val < 0 {
				return "", nil, msg(expr.Line, expr.Column, "Expecting Byte number literal but value is out of range.")
			}
			code = "(byte) " + expr.IntegerPart
			dt = ByteType
		} else if expectedType == SignedByteType {
			if expr.FractionalPart != "" {
				return "", nil, msg(expr.Line, expr.Column, "Expecting SByte literal, but got floating-point.")
			}
			val, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", nil, msg(expr.Line, expr.Column, "Expecting SByte number literal.")
			}
			if val > math.MaxInt8 || val < math.MinInt8 {
				return "", nil, msg(expr.Line, expr.Column, "Expecting SByte number literal but value is out of range.")
			}
			code = "(sbyte) " + expr.IntegerPart
			dt = SignedByteType
		} else {
			return "", nil, msg(expr.Line, expr.Column, "Non-number type given as expected type for a number literal.")
		}
	case StringAtom:
		code = "\"" + escapeString(expr.Content[1:len(expr.Content)-1]) + "\""
		dt = StrType
	case CallForm:
		code, dt, err = compileCallForm(expr, ns, expectedType, locals)
		if err != nil {
			return "", nil, err
		}
	case TypeCallForm:
		code, dt, err = compileTypeCallForm(expr, ns, expectedType, locals)
		if err != nil {
			return "", nil, err
		}
	case IndexingForm:
		code, dt, err = compileIndexingForm(expr, ns, false, locals)
		if err != nil {
			return "", nil, err
		}
	default:
		return "", nil, errors.New("Unexpected non-expression: line " +
			itoa(expr.GetLine()) + " column " + itoa(expr.GetColumn()))
	}
	if expectedType != nil && !IsSubType(dt, expectedType) {
		return "", nil, msg(expr.GetLine(), expr.GetColumn(), "Expression has wrong type.")
	}
	return
}

func compileGlobals(globals []GlobalDef, ns *Namespace, indent string) (string, error) {
	code := "public class " + GlobalsClass + " {\n"
	for _, g := range globals {
		globalInfo := ns.Globals[g.Name]
		code += indent + "public " + compileType(globalInfo.Type) + " " + string(g.Name)
		if g.Value != nil {
			c, returnedType, err := compileExpression(g.Value, ns, globalInfo.Type, map[ShortName]Type{})
			if err != nil {
				return "", err
			}
			if !IsSubType(returnedType, globalInfo.Type) {
				return "", msg(g.Line, g.Column, "Initial value of global does not match the declared type.")
			}
			code += " = " + c
		}
		code += ";\n"
	}
	code += "}\n\n"
	return code, nil
}

func escapeString(str string) string {
	src := []rune(str)
	dest := make([]rune, 0, len(src))
	for i := 0; i < len(src); {
		switch src[i] {
		case '\n':
			dest = append(dest, '\\', 'n')
			i++
		case '\r':
			dest = append(dest, '\\', 'n')
			i += 2 // skip over the \n that follows the \r
		case '\t':
			dest = append(dest, '\\', 't')
			i++
		case '"':
			dest = append(dest, '\\', '"')
			i++
		default:
			dest = append(dest, src[i])
			i++
		}
	}
	return string(dest)
}

// assumes a valid data type. Accepts Struct but not a StructDefinition
func compileType(t Type) string {
	switch t := t.(type) {
	case *ClassInfo:
		return string(t.Namespace.CSName) + "." + string(t.Name)
	case *StructInfo:
		return string(t.Namespace.CSName) + "." + string(t.Name)
	case *InterfaceInfo:
		return string(t.Namespace.CSName) + "." + string(t.Name)
	case ArrayType:
		return compileType(t.BaseType) + "[]"
	case BuiltinType:
		switch t.Name {
		case "I":
			return "int"
		case "F":
			return "float"
		case "B":
			return "byte"
		case "SB":
			return "sbyte"
		case "Bool":
			return "bool"
		case "Str":
			return "string"
		case "Any":
			return "object"
		}
	}
	panic("should never reach here")
	return ""
}

func compileIfForm(s IfForm, returnType Type,
	ns *Namespace, locals map[ShortName]Type, insideLoop bool, indent string) (string, error) {
	c, conditionType, err := compileExpression(s.Condition, ns, BoolType, locals)
	if err != nil {
		return "", err
	}
	if conditionType != BoolType {
		return "", msg(s.Line, s.Column, "The 'if' condition must return a boolean.")
	}
	code := "if (" + c + ") {\n"
	c, err = compileBody(s.Body, returnType, ns, locals, insideLoop, false, indent+"\t")
	if err != nil {
		return "", nil
	}
	code += c + "\n}"
	for i, elif := range s.ElifConds {
		c, conditionType, err := compileExpression(elif, ns, BoolType, locals)
		if err != nil {
			return "", err
		}
		if conditionType != BoolType {
			return "", msg(s.Line, s.Column, "Elif condition expression does not return a boolean.")
		}
		code += " else if (" + c + ") {\n"
		c, err = compileBody(s.ElifBodies[i], returnType, ns, locals, insideLoop, false, indent+"\t")
		if err != nil {
			return "", err
		}
		code += c + "}"
	}
	if len(s.ElseBody) > 0 {
		c, err := compileBody(s.ElseBody, returnType, ns, locals, insideLoop, false, indent+"\t")
		if err != nil {
			return "", err
		}
		code += " else {\n" + c + "}"
	}
	return code + "\n", nil
}

func compileBody(statements []Statement, returnType Type,
	ns *Namespace, locals map[ShortName]Type, insideLoop bool,
	requiresReturn bool, indent string) (string, error) {
	code := ""
	if requiresReturn {
		// len(statments) will not be 0
		if st, ok := statements[len(statements)-1].(ReturnForm); !ok {
			return "", msg(st.Line, st.Column, "this function must end with a return statement.")
		}
	}
	for _, s := range statements {
		var c string
		var err error
		switch f := s.(type) {
		case IfForm:
			newLocals := map[ShortName]Type{}
			for k, v := range locals {
				newLocals[k] = v
			}
			c, err = compileIfForm(f, returnType, ns, newLocals, insideLoop, indent)
		case AssignmentForm:
			c, err = compileAssignment(f, ns, locals, indent)
		case ReturnForm:
			c, err = compileReturn(f, returnType, ns, locals, indent)
		case BreakForm:
			if insideLoop {
				c += indent + "break; \n"
			} else {
				err = msg(f.Line, f.Column, "cannot have break statement outside a loop.")
			}
		case ContinueForm:
			if insideLoop {
				c += indent + "continue; \n"
			} else {
				err = msg(f.Line, f.Column, "cannot have continue statement outside a loop.")
			}
		case CallForm:
			c, _, err = compileCallForm(f, ns, nil, locals)
			c = indent + c + ";\n"
		case VarForm:
			if locals[f.Target] != nil {
				return "", msg(f.Line, f.Column, "Local variable of same name already exists in this scope.")
			}
			var typeStr string
			var t Type
			if f.Type.Name != "" {
				t = ns.GetType(f.Type)
				if t == nil {
					return "", msg(f.Line, f.Column, "Var form specifies unknown type.")
				}
				typeStr = compileType(t) + " "
			}
			valStr := ""
			var exprType Type
			if f.Value != nil {
				valStr, exprType, err = compileExpression(f.Value, ns, t, locals)
				if err != nil {
					return "", err
				}
				if t == nil {
					typeStr = compileType(exprType) + " "
				} else if !IsSubType(exprType, t) {
					return "", msg(f.Line, f.Column, "Initial value in var statement is wrong type.")
				}
			}
			if valStr == "" {
				c = indent + typeStr + string(f.Target) + ";\n"
			} else {
				c = indent + typeStr + string(f.Target) + " = " + valStr + ";\n"
			}
			if t == nil {
				locals[f.Target] = exprType
			} else {
				locals[f.Target] = t
			}
		}
		if err != nil {
			return "", err
		}
		code += c
	}
	return code, nil
}

func compileIndexingForm(f IndexingForm, ns *Namespace, isTarget bool,
	locals map[ShortName]Type) (code string, dt Type, err error) {

	last := f.Args[len(f.Args)-1]
	static := false
	if ta, ok := last.(TypeAtom); ok {
		dt = ns.GetType(ta)
		if dt == nil {
			err = msg(f.Line, f.Column, "Indexing form references unknown type.")
			return
		}
		code = compileType(dt)
		static = true
	} else {
		code, dt, err = compileExpression(last, ns, nil, locals)
		if err != nil {
			return
		}
	}
	for i := len(f.Args) - 2; i >= 0; i-- {
		expr := f.Args[i]
		if indexedType, ok := IsIndexableType(dt); ok {
			var c string
			var argType Type
			c, argType, err = compileExpression(expr, ns, nil, locals)
			if err != nil {
				return
			}
			if !IsInteger(argType) {
				err = msg(f.Line, f.Column, "Expecting integer for array index in indexing form.")
				return
			}
			code += "[" + c + "]"
			dt = indexedType
		} else {
			if varExpr, ok := expr.(VarExpression); ok {
				if varExpr.Namespace != "" {
					err = msg(varExpr.Line, varExpr.Column, "Improper name in indexing form.")
					return
				}
				dt, ok, err = GetFieldOrPropertyType(varExpr.Name, dt, isTarget, static)
				if err != nil {
					err = msg(varExpr.Line, varExpr.Column, err.Error())
					return
				}
				if !ok {
					err = msg(varExpr.Line, varExpr.Column, "No field called '"+string(varExpr.Name)+"' in indexing form.")
					return
				}
				code += "." + string(varExpr.Name)
			} else {
				err = msg(varExpr.Line, varExpr.Column, "Improper name in indexing form.")
				return
			}
		}
	}
	return code, dt, nil
}

func compileAssignment(f AssignmentForm, ns *Namespace, locals map[ShortName]Type,
	indent string) (code string, err error) {
	var dt Type
	switch target := f.Target.(type) {
	case VarExpression:
		if target.Namespace == "" {
			var ok bool
			dt, ok = locals[target.Name] // local name takes precedence over unqualified global name
			if ok {
				code = string(target.Name)
				break
			}
		}
		globalInfo := ns.GetGlobal(target.Name, target.Namespace)
		if globalInfo == nil {
			return "", msg(f.Line, f.Column, "Assignment to non-existent variable.")
		}
		code = string(globalInfo.Namespace.CSName) + "." + string(globalInfo.Name)
	case IndexingForm:
		code, dt, err = compileIndexingForm(target, ns, true, locals)
		if err != nil {
			return "", err
		}
	}

	code += " = "
	exprStr, exprType, err := compileExpression(f.Value, ns, dt, locals)
	if err != nil {
		return "", err
	}
	if !IsSubType(exprType, dt) {
		return "", msg(f.Line, f.Column, "Assignment value is wrong type.")
	}
	return indent + code + exprStr + ";\n", nil
}

func compileReturn(f ReturnForm, returnType Type, ns *Namespace, locals map[ShortName]Type, indent string) (string, error) {
	code := indent + "return "
	c, exprType, err := compileExpression(f.Value, ns, returnType, locals)
	if err != nil {
		return "", err
	}
	if !IsSubType(exprType, returnType) {
		return "", msg(f.Line, f.Column, "Return value is wrong type.")
	}
	code += c + ";\n"
	return code, nil
}

func compileFunc(f FuncDef, ns *Namespace, indent string) (string, error) {
	code := indent + "public static "
	returnType := ns.GetType(f.Return)
	if returnType == nil {
		code += "void "
	} else {
		code += compileType(returnType) + " "
	}
	code += string(f.Name) + "("
	locals := map[ShortName]Type{}
	for i, paramName := range f.ParamNames {
		paramType := ns.GetType(f.ParamTypes[i])
		if paramType == nil {
			return "", msg(f.ParamTypes[i].Line, f.ParamTypes[i].Column, "Function has unknown parameter type.")
		}
		locals[paramName] = paramType
		code += string(paramName) + " " + compileType(paramType)
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, returnType, ns, locals, false, returnType != nil, indent+"\t")
	if err != nil {
		return "", err
	}
	code += body + indent + "}\n"
	return code, nil
}

func compileMethod(f MethodDef, class Type, ns *Namespace, indent string) (string, error) {
	code := indent + "public "
	if f.IsStatic {
		code += "static "
	}
	returnType := ns.GetType(f.Return)
	if returnType == nil {
		code += "void "
	} else {
		code += compileType(returnType) + " "
	}
	code += string(f.Name) + "("
	locals := map[ShortName]Type{thisWord: class}
	for i, paramName := range f.ParamNames {
		paramType := ns.GetType(f.ParamTypes[i])
		if paramType == nil {
			return "", msg(f.ParamTypes[i].Line, f.ParamTypes[i].Column, "Function has unknown parameter type.")
		}
		locals[paramName] = paramType
		code += string(paramName) + " " + compileType(paramType)
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, returnType, ns, locals, false, returnType != nil, indent+"\t")
	if err != nil {
		return "", err
	}
	code += body + indent + "}\n"
	return code, nil
}

// type should be a class or struct
func compileConstructor(f ConstructorDef, t Type, ns *Namespace, indent string) (string, error) {
	var name string
	switch t := t.(type) {
	case *ClassInfo:
		name = string(t.Name)
	case *StructInfo:
		name = string(t.Name)
	}

	code := indent + "public " + name + "("
	locals := map[ShortName]Type{thisWord: t}
	for i, paramName := range f.ParamNames {
		paramType := ns.GetType(f.ParamTypes[i])
		if paramType == nil {
			return "", msg(f.ParamTypes[i].Line, f.ParamTypes[i].Column, "Function has unknown parameter type.")
		}
		locals[paramName] = paramType
		code += string(paramName) + " " + compileType(paramType)
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, t, ns, locals, false, false, indent+"\t")
	if err != nil {
		return "", err
	}
	code += body + indent + "}"
	return code, nil
}

func compileField(f FieldDef, ns *Namespace, indent string) (string, error) {
	code := indent
	switch f.AccessLevel {
	case PublicAccess:
		code += "public "
	case PrivateAccess:
		code += "private "
	case ProtectedAccess:
		code += "protected "
	}
	if f.IsStatic {
		code += "static "
	}

	t := ns.GetType(f.Type)
	if t == nil {
		return "", msg(f.Line, f.Column, "Field has unknown type.")
	}

	typeStr := compileType(t)
	if f.Value != nil {
		exprStr, _, err := compileExpression(f.Value, ns, t, nil)
		if err != nil {
			return "", err
		}
		code += typeStr + " " + string(f.Name) + " = " + exprStr + ";"
	} else {
		code += typeStr + " " + string(f.Name) + ";"
	}
	return code, nil
}

func compileProperty(p PropertyDef, containingType Type, ns *Namespace, indent string) (string, error) {
	var code string
	t := ns.GetType(p.Type)
	if t == nil {
		return "", msg(p.Line, p.Column, "Property has unknown type.")
	}

	if p.IsManual {
		if len(p.GetBody) == 0 {
			return "", msg(p.Line, p.Column, "Property is manual (no auto-backing field) but is missing explicit getter.")
		}

		if len(p.SetBody) == 0 {
			return "", msg(p.Line, p.Column, "Property is manual (no auto-backing field) but is missing explicit settter.")
		}
	} else {
		code += indent
		switch p.AccessLevel {
		case PublicAccess:
			code += "public "
		case PrivateAccess:
			code += "private "
		case ProtectedAccess:
			code += "protected "
		}
		if p.IsStatic {
			code += "static "
		}
		code += compileType(t) + " " + string(p.Name) + "_;\n"

	}

	code += indent
	switch p.AccessLevel {
	case PublicAccess:
		code += "public "
	case PrivateAccess:
		code += "private "
	case ProtectedAccess:
		code += "protected "
	}
	if p.IsStatic {
		code += "static "
	}

	code += compileType(t) + " " + string(p.Name) + " {\n"

	if len(p.GetBody) > 0 {
		code += indent + "\tget {\n"
		c, err := compileBody(p.GetBody, t, ns, map[ShortName]Type{thisWord: containingType}, false, true, indent+"\t\t")
		if err != nil {
			return "", err
		}
		code += c + indent + "\t}\n"
	} else {
		code += indent + "\tget {return " + string(p.Name) + "_;}\n"
	}

	if len(p.SetBody) > 0 {
		code += indent + "\tset {\n"
		c, err := compileBody(p.SetBody, nil, ns, map[ShortName]Type{thisWord: containingType, propertyValueParam: t}, false, false, indent+"\t\t")
		if err != nil {
			return "", err
		}
		code += c + indent + "\t}\n"
	} else {
		code += indent + "\tset {this." + string(p.Name) + "_ = " + propertyValueParam + ";}\n"
	}
	code += indent + "}\n"

	return code, nil
}

func compileClass(f ClassDef, ns *Namespace, indent string) (string, error) {
	var code string
	switch f.AccessLevel {
	case PublicAccess:
		code = "public class "
	case PrivateAccess:
		code = "private class "
	case ProtectedAccess:
		code = "protected class "
	}
	if f.Type.Namespace != "" {
		return "", msg(f.Line, f.Column, "Class name in its definition should not be qualified by namespace.")
	}

	classInfo := ns.GetClass(f.Type.Name, f.Type.Namespace)
	if classInfo == nil {
		panic("Internal error: cannot find ClassInfo when compiling class.")
	}

	code += string(f.Type.Name)
	if len(classInfo.Interfaces) > 0 || classInfo.Parent != nil {
		code += " : "
	}
	if classInfo.Parent != nil {
		code += compileType(classInfo.Parent)
	}
	if len(classInfo.Interfaces) > 0 {
		code += ", "
	}
	for i, inter := range classInfo.Interfaces {
		code += compileType(inter)
		if i < len(classInfo.Interfaces)-1 {
			code += ", "
		}
	}
	code += " {\n"
	for _, fieldDef := range f.Fields {
		c, err := compileField(fieldDef, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	if len(f.Fields) > 0 {
		code += "\n"
	}
	for _, propertyDef := range f.Properties {
		c, err := compileProperty(propertyDef, classInfo, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, constructorDef := range f.Constructors {
		c, err := compileConstructor(constructorDef, classInfo, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for i, methodDef := range f.Methods {
		c, err := compileMethod(methodDef, classInfo, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c
		if i < len(f.Methods)-1 {
			code += "\n"
		}
	}
	code += "}\n\n"
	return code, nil
}

func compileStruct(f StructDef, ns *Namespace) (string, error) {
	var code string
	switch f.AccessLevel {
	case PublicAccess:
		code = "public "
	case PrivateAccess:
		code = "private "
	case ProtectedAccess:
		code = "protected "
	}

	if f.Type.Namespace != "" {
		return "", msg(f.Line, f.Column, "Struct name in its definition should not be qualified by namespace.")
	}

	structInfo := ns.GetStruct(f.Type.Name, f.Type.Namespace)
	if structInfo == nil {
		panic("Internal error: cannot find StructInfo when compiling struct.")
	}

	code += string(f.Type.Name)
	if len(structInfo.Interfaces) > 0 {
		code += " : "
	}
	for i, inter := range structInfo.Interfaces {
		code += compileType(inter)
		if i < len(structInfo.Interfaces)-1 {
			code += ", "
		}
	}
	code += " {"
	for _, fieldDef := range f.Fields {
		c, err := compileField(fieldDef, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, propertyDef := range f.Properties {
		c, err := compileProperty(propertyDef, structInfo, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, constructorDef := range f.Constructors {
		c, err := compileConstructor(constructorDef, structInfo, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, methodDef := range f.Methods {
		c, err := compileMethod(methodDef, structInfo, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	code += "}"
	return code, nil
}

func compileInterface(def InterfaceDef, ns *Namespace, indent string) (string, error) {
	if def.Type.Namespace != "" {
		return "", msg(def.Line, def.Column, "Interface name in its definition should not be qualified by namespace.")
	}

	interfaceInfo := ns.GetInterface(def.Type.Name, def.Type.Namespace)
	if interfaceInfo == nil {
		panic("Internal error: cannot find ClassInfo when compiling class.")
	}

	var code string
	switch def.AccessLevel {
	case PublicAccess:
		code = "public interface "
	case PrivateAccess:
		code = "private interface "
	case ProtectedAccess:
		code = "protected interface "
	}
	code += string(interfaceInfo.Name)
	if len(interfaceInfo.Parents) > 0 {
		code += " : "
	}
	for i, parent := range interfaceInfo.Parents {
		code += compileType(parent)
		if i < len(interfaceInfo.Parents)-1 {
			code += ", "
		}
	}
	code += " {\n"
	for name, methods := range interfaceInfo.Methods {
		for _, method := range methods {
			if method.Return == nil {
				code += "\tvoid " + string(name) + "("
			} else {
				code += "\t" + compileType(method.Return) + " " + string(name) + "("
			}
			for i, paramType := range method.ParamTypes {
				code += compileType(paramType)
				if i < len(method.ParamTypes)-1 {
					code += ", "
				}
			}
			code += ");\n"
		}
	}
	for name, prop := range interfaceInfo.Properties {
		code += "\t" + compileType(prop.Type) + " " + string(name) + " {\n"
		if prop.HasGetter {
			code += "\t\t" + "get;\n"
		}
		if prop.HasSetter {
			code += "\t\t" + "set;\n"
		}
		code += "\t}\n"
	}

	code += "}\n\n"
	return code, nil
}
