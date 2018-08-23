package main

import (
	"errors"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
)

func codeGen(topDefs *TopDefs, ns *Namespace) (string, error) {
	code := "namespace " + ns.Name + " {\n\n"

	c, err := compileGlobals(topDefs.Globals, ns)
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

func compileNamespace(namespace string, basedir string, namespaces map[string]*Namespace) error {
	if _, ok := namespaces[namespace]; ok {
		return errors.New("Recursive import depedency: " + namespace)
	}

	topDefs := &TopDefs{
		Classes: []ClassDef{},
		Structs: []StructDef{},
		Funcs:   []FuncDef{},
		Globals: []GlobalDef{},
		Imports: []ImportDef{},
	}

	files, err := ioutil.ReadDir(basedir)
	if err != nil {
		log.Fatal(err)
	}

	namespaceShortName := namespace[strings.LastIndex(namespace, ".")+1:]
	hasMain := false
	prefix := namespaceShortName + "_"
	start := time.Now()
	for _, file := range files {
		name := file.Name()
		isMain := name == namespaceShortName+".bf"
		if isMain {
			hasMain = true
		}
		if isMain || (strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".bf")) {

			data, err := ioutil.ReadFile(basedir + "/" + name)
			if err != nil {
				return err
			}
			data = append(data, '\n')
			tokens, err := lex(string(data))
			if err != nil {
				return err
			}
			atoms, err := read(tokens)
			if err != nil {
				return err
			}
			err = parse(atoms, topDefs, isMain)
			if err != nil {
				return err
			}
		}
	}

	if !hasMain {
		return errors.New("Cannot compile namespace '" + namespace + "': expecting a file named '" + namespaceShortName + ".bf'")
	}

	ns, err := createNamespace(topDefs, namespace, basedir, namespaces)
	if err != nil {
		return err
	}
	namespaces[namespace] = ns

	code, err := codeGen(topDefs, ns)
	if err != nil {
		return err
	}

	debug("Time: ", time.Since(start))

	outputFilename := namespaceShortName + ".cs"
	err = ioutil.WriteFile(outputFilename, []byte(code), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func createNamespace(topDefs *TopDefs, namespace string, basedir string, namespaces map[string]*Namespace) (*Namespace, error) {
	if topDefs.Namespace.Name == "" {
		return nil, errors.New("Namespace '" + namespace + "' missing its namespace declaration.")
	}
	if topDefs.Namespace.Name != namespace {
		return nil, errors.New("Namespace '" + namespace + "' declaration does not match expected name.")
	}
	interfaces := map[string]*InterfaceInfo{}
	classes := map[string]*ClassInfo{}
	structs := map[string]*StructInfo{}
	globals := map[string]*GlobalInfo{}
	funcs := map[string][]*CallableInfo{}
	constructors := map[string][]*CallableInfo{}
	methods := map[string][]*CallableInfo{}

	fullNames := map[string]string{} // unqualified names or partly qualified names -> full qualified names
	// e.g. foo                 -> foo/mynamspace.subnamespace
	//      foo/subnamespace    -> foo/mynamspace.subnamespace

	shortnames := map[string]bool{}
	for _, importDef := range topDefs.Imports {
		if _, ok := shortnames[importDef.Shortname]; ok {
			return nil, errors.New("Name collision between imported namespace short names: " + importDef.Shortname)
		}
	}

	for _, importDef := range topDefs.Imports {

		foreign, ok := namespaces[importDef.Namespace]
		if !ok {
			err := compileNamespace(importDef.Namespace, basedir, namespaces)
			if err != nil {
				return nil, err
			}
			foreign = namespaces[importDef.Namespace]
		}

		for key, interfaceInfo := range foreign.Interfaces {
			if interfaceInfo.Namespace == foreign.Name {
				interfaces[key] = interfaceInfo
				if _, ok := fullNames[interfaceInfo.Name]; ok {
					return nil, errors.New("Name collision: " + interfaceInfo.Name + " imported from more than one namespaces.")
				}
				fullNames[interfaceInfo.Name] = key
				fullNames[interfaceInfo.Name+"/"+interfaceInfo.ShortNamespace] = key
			}
		}

		for key, classInfo := range foreign.Classes {
			if classInfo.Namespace == foreign.Name {
				classes[key] = classInfo
				if _, ok := fullNames[classInfo.Name]; ok {
					return nil, errors.New("Name collision: " + classInfo.Name + " imported from more than one namespaces.")
				}
				fullNames[classInfo.Name] = key
				fullNames[classInfo.Name+"/"+classInfo.ShortNamespace] = key
			}
		}

		for key, structInfo := range foreign.Structs {
			if structInfo.Namespace == foreign.Name {
				structs[key] = structInfo
				if _, ok := fullNames[structInfo.Name]; ok {
					return nil, errors.New("Name collision: " + structInfo.Name + " imported from more than one namespaces.")
				}
				fullNames[structInfo.Name] = key
				fullNames[structInfo.Name+"/"+structInfo.ShortNamespace] = key
			}
		}

		for key, globalInfo := range foreign.Globals {
			if globalInfo.Namespace == foreign.Name {
				globals[key] = globalInfo
				if _, ok := fullNames[globalInfo.Name]; ok {
					return nil, errors.New("Name collision: " + globalInfo.Name + " imported from more than one namespaces.")
				}
				fullNames[globalInfo.Name] = key
				fullNames[globalInfo.Name+"/"+globalInfo.ShortNamespace] = key
			}
		}

		// we don't look for function signature conflicts between imported namespaces because
		// we only care about conflicts at call sites

		for key, callables := range foreign.Funcs {
			for _, callable := range callables {
				if callable.Namespace == foreign.Name {
					funcs[key] = append(funcs[key], callable)
				}
			}
		}

		for key, callables := range foreign.Constructors {
			if len(callables) > 1 && callables[0].Namespace == foreign.Name {
				constructors[key] = callables
			}
		}

		// methods
		for key, callables := range foreign.Methods {
			for _, callable := range callables {
				if callable.Namespace == foreign.Name {
					methods[key] = append(methods[key], callable)
				}
			}
		}
	}

	for i, interfaceDef := range topDefs.Interfaces {
		if _, ok := fullNames[interfaceDef.Type.Name]; ok {
			return nil, msg(interfaceDef.Line, interfaceDef.Column, "Interface name already used.")
		}
		topDefs.Interfaces[i].Type.Namespace = namespace
		fullName := interfaceDef.Type.Name + "/" + namespace
		fullNames[interfaceDef.Type.Name] = fullName

		sigs := map[string]SignatureInfo{}
		for j, methodName := range interfaceDef.MethodNames {
			params := interfaceDef.MethodParams[j]
			returnType := interfaceDef.MethodReturnTypes[j]
			sigs[methodName] = SignatureInfo{
				ParamTypes: params,
				ReturnType: returnType,
			}
			methods[methodName] = append(methods[methodName],
				&CallableInfo{
					IsMethod:   true,
					ParamNames: make([]string, len(params)+1), // in case some loop uses len of ParamNames
					ParamTypes: append([]DataType{interfaceDef.Type}, params...),
					ReturnType: returnType,
				},
			)
		}

		interfaces[fullName] = &InterfaceInfo{
			Name:       interfaceDef.Type.Name,
			Namespace:  namespace,
			Signatures: sigs,
		}
	}

	for i, structDef := range topDefs.Structs {
		if _, ok := fullNames[structDef.Type.Name]; ok {
			return nil, msg(structDef.Line, structDef.Column, "Struct name already used.")
		}
		topDefs.Structs[i].Type.Namespace = namespace
		fullName := structDef.Type.Name + "/" + namespace
		fullNames[structDef.Type.Name] = fullName

		fields := map[string]FieldInfo{}
		for _, f := range structDef.Fields {
			fields[f.Name] = FieldInfo{
				Name:        f.Name,
				Type:        f.Type,
				AccessLevel: f.AccessLevel,
			}
		}

		structInterfaces := []*InterfaceInfo{}
		for _, dt := range structDef.Interfaces {
			interfaceFullName := dt.Name + "/" + dt.Namespace
			interfaceInfo, ok := interfaces[interfaceFullName]
			if !ok {
				return nil, msg(structDef.Line, structDef.Column, "Struct implements unknown interface.")
			}
			structInterfaces = append(structInterfaces, interfaceInfo)
		}

		structs[fullName] = &StructInfo{
			Name:       structDef.Type.Name,
			Namespace:  namespace,
			Fields:     fields,
			Interfaces: structInterfaces,
		}
	}

	for _, globalInfo := range topDefs.Globals {
		if _, ok := fullNames[globalInfo.Type.Name]; ok {
			return nil, msg(globalInfo.Line, globalInfo.Column, "Global name already used.")
		}
		fullName := globalInfo.Type.Name + "/" + namespace
		fullNames[globalInfo.Type.Name] = fullName
		globals[fullName] = &GlobalInfo{
			Name:      globalInfo.Type.Name,
			Namespace: namespace,
		}
	}

	parentExpectedIdxs := []int{}
	for i, class := range topDefs.Classes {
		if _, ok := fullNames[class.Type.Name]; ok {
			return nil, msg(class.Line, class.Column, "Class name already used.")
		}
		topDefs.Classes[i].Type.Namespace = namespace
		fullName := class.Type.Name + "/" + namespace
		fullNames[class.Type.Name] = fullName

		classInterfaces := []*InterfaceInfo{}

		for j, dt := range class.Supertypes {
			var interfaceFullName string
			if dt.Namespace == "" {
				interfaceFullName = dt.Name + "/" + namespace
			} else {
				interfaceFullName = dt.Name + "/" + dt.Namespace
			}
			interfaceInfo, ok := interfaces[interfaceFullName]
			if !ok {
				if j == 0 {
					parentExpectedIdxs = append(parentExpectedIdxs, i)
					continue
				}
				return nil, msg(class.Line, class.Column, "Class implements unknown interface.")
			}
			classInterfaces = append(classInterfaces, interfaceInfo)
		}

		fields := map[string]FieldInfo{}
		for _, f := range class.Fields {
			fields[f.Name] = FieldInfo{
				Name:        f.Name,
				Type:        f.Type,
				AccessLevel: f.AccessLevel,
			}
		}

		classes[fullName] = &ClassInfo{
			Name:       class.Type.Name,
			Namespace:  namespace,
			Fields:     fields,
			Interfaces: classInterfaces,
		}
	}
	// check that parents exist having already constructed list of all classes
	for _, idx := range parentExpectedIdxs {
		classDef := topDefs.Classes[idx]
		classInfo := classes[classDef.Type.Name+"/"+namespace]
		firstSuperType := classDef.Supertypes[0]
		parentFullName := firstSuperType.Name + "/" + firstSuperType.Namespace
		if firstSuperType.Namespace == "" {
			parentFullName += namespace
		}
		parentInfo := classes[parentFullName]
		if parentInfo == nil {
			return nil, msg(classDef.Line, classDef.Column, "Parent class does not exist.")
		}
		classInfo.Parent = parentInfo
	}

	for _, fn := range topDefs.Funcs {
		funcs[fn.Name] = append(funcs[fn.Name],
			&CallableInfo{
				IsMethod:   false,
				Namespace:  namespace,
				ParamNames: fn.ParamNames,
				ParamTypes: fn.ParamTypes,
				ReturnType: fn.ReturnType,
			},
		)
	}

	for _, class := range topDefs.Classes {
		fullName := class.Type.Name + "/" + namespace

		hasZeroArgConstructor := false
		for _, constructor := range class.Constructors {
			if len(constructor.ParamNames) == 0 {
				hasZeroArgConstructor = true
			}
			constructors[fullName] = append(constructors[fullName],
				&CallableInfo{
					IsMethod:   false,
					Namespace:  namespace,
					ParamNames: constructor.ParamNames,
					ParamTypes: constructor.ParamTypes,
					ReturnType: class.Type,
				},
			)
		}
		// add sig for default constructor if none explicitly defined
		if !hasZeroArgConstructor {
			constructors[fullName] = append(constructors[fullName],
				&CallableInfo{
					IsMethod:   false,
					Namespace:  namespace,
					ParamNames: nil,
					ParamTypes: nil,
					ReturnType: class.Type,
				},
			)
		}

		for _, method := range class.Methods {
			methods[method.Name] = append(methods[method.Name],
				&CallableInfo{
					IsMethod:   true,
					Namespace:  namespace,
					ParamNames: append([]string{thisWord}, method.ParamNames...),
					ParamTypes: append([]DataType{class.Type}, method.ParamTypes...),
					ReturnType: method.ReturnType,
				},
			)
		}
	}

	// check that classes actually implement their interfaces
	for _, classDef := range topDefs.Classes {
		fullName := fullNames[classDef.Type.Name]
		classInfo := classes[fullName]
		classDt := classInfo.DataType()
		for _, interfaceInfo := range classInfo.Interfaces {
			for methodName, sig := range interfaceInfo.Signatures {
				callables, ok := methods[methodName]
				if !ok {
					return nil, msg(classDef.Line, classDef.Column, "Class "+fullName+" does not implement method "+
						methodName+" of interface "+interfaceInfo.Name+"/"+interfaceInfo.Namespace+".")
				}
				match := false
			Loop:
				for _, callable := range callables {
					if !sig.ReturnType.EqualType(callable.ReturnType) {
						continue
					}
					if len(sig.ParamTypes)+1 != len(callable.ParamTypes) {
						continue
					}
					if !callable.ParamTypes[0].EqualType(classDt) {
						continue
					}
					for i := 1; i < len(sig.ParamTypes); i++ {
						if !sig.ParamTypes[i].EqualType(callable.ParamTypes[i]) {
							continue Loop
						}
					}
					match = true
					break
				}
				if !match {
					return nil, msg(classDef.Line, classDef.Column, "Class "+fullName+" does not implement method "+
						methodName+" of interface "+interfaceInfo.Name+"/"+interfaceInfo.Namespace+"."+spew.Sdump(interfaceInfo.Signatures))
				}
			}
		}
	}

	return &Namespace{
		Name:         namespace,
		Classes:      classes,
		Structs:      structs,
		Interfaces:   interfaces,
		Globals:      globals,
		Constructors: constructors,
		Funcs:        funcs,
		Methods:      methods,
		FullNames:    fullNames,
	}, nil
}

func fullName(name string, namespace string, ns *Namespace) string {
	if namespace == "" {
		if fullname, ok := ns.FullNames[name]; ok {
			return fullname
		}
		return name
	}
	return name + "/" + namespace
}

func (dt DataType) FullName(ns *Namespace) string {
	if dt.Namespace == "" {
		switch dt.Name {
		case "I":
			fallthrough
		case "II":
			fallthrough
		case "F":
			fallthrough
		case "FF":
			fallthrough
		case "Byte":
			fallthrough
		case "SByte":
			return dt.Name
		}
	}
	return fullName(dt.Name, dt.Namespace, ns)
}

func (dt DataType) EqualType(other DataType) bool {
	if dt.Name != other.Name {
		return false
	}
	if dt.Namespace != other.Namespace {
		return false
	}
	if len(dt.TypeParams) != len(other.TypeParams) {
		return false
	}
	for i, paramType := range dt.TypeParams {
		if !paramType.EqualType(other.TypeParams[i]) {
			return false
		}
	}
	return true
}

func (dt DataType) CSName(ns *Namespace) string {
	if dt.Namespace == "" {
		switch dt.Name {
		case "I":
			return "int"
		case "II":
			return "long"
		case "F":
			return "float"
		case "FF":
			return "double"
		case "Byte":
			return "byte"
		case "SByte":
			return "sbyte"
		}
	}
	if dt.Namespace == "" {
		return ns.Name + "." + dt.Name
	} else {
		return dt.Namespace + "." + dt.Name
	}
}

func (dt DataType) GetInfo(ns *Namespace) (TypeInfo, bool) {
	fullname := dt.FullName(ns)
	if ti, ok := ns.Interfaces[fullname]; ok {
		return ti, true
	}
	if ti, ok := ns.Classes[fullname]; ok {
		return ti, true
	}
	if ti, ok := ns.Structs[fullname]; ok {
		return ti, true
	}
	return nil, false
}

func isZeroType(dt DataType) bool {
	return dt.Name == "" && dt.TypeParams == nil && dt.Namespace == ""
}

func IsIndexableType(dt DataType, ns *Namespace) bool {
	// todo: for now, we're ignoring arrays
	return false
}

// returns true if field exists
// todo: account for access level
func GetFieldType(field string, dt DataType, ns *Namespace) (DataType, bool) {
	fullname := dt.FullName(ns)
	if classInfo, ok := ns.Classes[fullname]; ok {
		for {
			if fieldInfo, ok := classInfo.Fields[field]; ok {
				return fieldInfo.Type, true
			}
			if classInfo.Parent == nil {
				break
			}
			classInfo = classInfo.Parent
		}
		return DataType{}, false
	}
	if structInfo, ok := ns.Structs[fullname]; ok {
		if fieldInfo, ok := structInfo.Fields[field]; ok {
			return fieldInfo.Type, true
		} else {
			return DataType{}, false
		}
	}
	return DataType{}, false
}

func isType(child DataType, parent DataType, ns *Namespace, exact bool) bool {
	// child is subtype of builtin type
	if parent.Namespace == "" {
		if parent.Name == "Any" {
			return true
		}
		if child.Namespace == "" {
			switch parent.Name {
			case "I":
				switch child.Name {
				case "I":
					fallthrough
				case "Byte":
					fallthrough
				case "SByte":
					return true
				}
			case "II":
				switch child.Name {
				case "II":
					fallthrough
				case "I":
					fallthrough
				case "Byte":
					fallthrough
				case "SByte":
					return true
				}
			case "F":
				switch child.Name {
				case "F":
					fallthrough
				case "Byte":
					fallthrough
				case "SByte":
					return true
				}
			case "FF":
				switch child.Name {
				case "FF":
					fallthrough
				case "F":
					fallthrough
				case "I":
					fallthrough
				case "Byte":
					fallthrough
				case "SByte":
					return true
				}
			case "Byte":
				switch child.Name {
				case "Byte":
					return true
				}
			case "SByte":
				switch child.Name {
				case "SByte":
					return true
				}
			}
		}
	}

	childFull := child.FullName(ns)
	parentFull := parent.FullName(ns)

	if childFull == parentFull {
		return true
	}

	if parentInterface, ok := ns.Interfaces[parentFull]; ok {
		if childInterface, ok := ns.Interfaces[childFull]; ok {
			return childInterface.IsImplementor(parentInterface)
		}

		if childClass, ok := ns.Classes[childFull]; ok {
			return childClass.IsImplementor(parentInterface)
		}

		if childStruct, ok := ns.Structs[childFull]; ok {
			return childStruct.IsImplementor(parentInterface)
		}

		return false
	}

	if parentClass, ok := ns.Classes[parentFull]; ok {
		if childClass, ok := ns.Classes[childFull]; ok {
			return childClass.IsDescendent(parentClass)
		} else {
			return false // subtype of a class can only be another class
		}
	}

	//// todo: account for type params
	// for i, childParam := range child.TypeParams {
	// 	if !isType(childParam, parent.TypeParams[i], ns, exact) {
	// 		return false
	// 	}
	// }
	return false
}

func (si *StructInfo) IsImplementor(ii *InterfaceInfo) bool {
	for _, interfaceInfo := range si.Interfaces {
		if interfaceInfo == ii {
			return true
		}
	}
	return false
}

func (ci *ClassInfo) IsImplementor(ii *InterfaceInfo) bool {
	for ; ci != nil; ci = ci.Parent {
		for _, interfaceInfo := range ci.Interfaces {
			if interfaceInfo == ii {
				return true
			}
		}
	}
	return false
}

func (ci *ClassInfo) DataType() DataType {
	// todo: account for type params
	return DataType{Name: ci.Name, Namespace: ci.Namespace}
}

func (ci *ClassInfo) IsDescendent(ancestor *ClassInfo) bool {
	for ; ci != nil; ci = ci.Parent {
		if ci == ancestor {
			return true
		}
	}
	return false
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
		ti, ok := sig.ParamTypes[0].GetInfo(ns)
		if !ok {
			return nil, msg(line, column, "Internal error: GetInfo() should never return false in ClosestMatchingSignature")
		}
		switch ti := ti.(type) {
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
			if !winnerClass.IsDescendent(other) {
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

func (ci *InterfaceInfo) IsImplementor(ii *InterfaceInfo) bool {
	if ci == ii {
		return true
	}
	return false
}

func isValidType(dt DataType, ns *Namespace) bool {
	full := fullName(dt.Name, dt.Namespace, ns)

	if _, ok := ns.Interfaces[full]; ok {
		return true
	}

	if _, ok := ns.Classes[full]; ok {
		return true
	}

	if _, ok := ns.Structs[full]; ok {
		return true
	}

	if dt.Namespace == "" {
		switch dt.Name {
		case "I":
			fallthrough
		case "II":
			fallthrough
		case "F":
			fallthrough
		case "FF":
			fallthrough
		case "Byte":
			fallthrough
		case "SByte":
			return true
		}
	}

	return false
}

// returns data type and csharp name (zero values if no such globals)
func getGlobal(name string, nsStr string, ns *Namespace) (DataType, string) {
	// todo
	return DataType{}, ""
}

func compileExpression(expr Expression, ns *Namespace, expectedType DataType,
	locals map[string]DataType) (code string, dt DataType, err error) {
	switch expr := expr.(type) {
	case VarExpression:
		dt, code = getGlobal(expr.Name, expr.Namespace, ns)
		if code == "" { // no global found
			if expr.Namespace != "" {
				return "", DataType{}, msg(expr.Line, expr.Column, "No global variable found of name: "+expr.Name+"/"+expr.Namespace)
			}
			var ok bool
			dt, ok = locals[expr.Name]
			if !ok {
				return "", DataType{}, msg(expr.Line, expr.Column, "No local variable found of name: "+expr.Name)
			}
			if expr.Name == thisWord {
				code = "this"
			} else {
				code = "_" + expr.Name // use _ prefix to avoid name conflicts with namespaces
			}
		}
	case ParsedNumberAtom:
		if isZeroType(expectedType) {
			if expr.FractionalPart == "" {
				val, err := strconv.Atoi(expr.IntegerPart)
				if err != nil {
					return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Int number literal.")
				}
				if val > math.MaxInt32 || val < math.MinInt32 {
					return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Int number literal but magnitude is too great.")
				}
				code = expr.IntegerPart
				dt = IntType
			} else {
				code = "(double) " + expr.IntegerPart + "." + expr.FractionalPart
				dt = DoubleType
			}
		} else if expectedType.Name == IntType.Name {
			if expr.FractionalPart != "" {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Int literal, but got floating-point.")
			}
			val, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Int number literal.")
			}
			if val > math.MaxInt32 || val < math.MinInt32 {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Int number literal but magnitude is too great.")
			}
			code = expr.IntegerPart
			dt = IntType
		} else if expectedType.Name == LongIntType.Name {
			if expr.FractionalPart != "" {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting II literal, but got floating-point.")
			}
			_, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting II number literal, but value is not integer or out of range.")
			}
			code = "(long) " + expr.IntegerPart
			dt = LongIntType
		} else if expectedType.Name == FloatType.Name {
			if expr.FractionalPart == "" {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Float literal, but got integer.")
			}
			code = "(float) " + expr.IntegerPart + "." + expr.FractionalPart
			dt = FloatType
		} else if expectedType.Name == DoubleType.Name {
			// todo check within double range
			code = "(double) " + expr.IntegerPart + "." + expr.FractionalPart
			dt = DoubleType
		} else if expectedType.Name == ByteType.Name {
			if expr.FractionalPart != "" {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Byte literal, but got floating-point.")
			}
			val, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Byte number literal.")
			}
			if val > math.MaxUint8 || val < 0 {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting Byte number literal but value is out of range.")
			}
			code = "(byte) " + expr.IntegerPart
			dt = ByteType
		} else if expectedType.Name == SignedByteType.Name {
			if expr.FractionalPart != "" {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting SByte literal, but got floating-point.")
			}
			val, err := strconv.Atoi(expr.IntegerPart)
			if err != nil {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting SByte number literal.")
			}
			if val > math.MaxInt8 || val < math.MinInt8 {
				return "", DataType{}, msg(expr.Line, expr.Column, "Expecting SByte number literal but value is out of range.")
			}
			code = "(sbyte) " + expr.IntegerPart
			dt = SignedByteType
		} else {
			return "", DataType{}, msg(expr.Line, expr.Column, "Non-number type given as expected type for a number literal.")
		}
	case StringAtom:
		code = "\"" + expr.Content[1:len(expr.Content)-1] + "\""
		dt = StrType
	case CallForm:
		code, dt, err = compileCallForm(expr, ns, expectedType, locals)
		if err != nil {
			return "", DataType{}, err
		}
	case TypeCallForm:
		code, dt, err = compileTypeCallForm(expr, ns, expectedType, locals)
		if err != nil {
			return "", DataType{}, err
		}
	case IndexingForm:
		code, dt, err = compileIndexingForm(expr, ns, locals)
		if err != nil {
			return "", DataType{}, err
		}
	default:
		return "", DataType{}, errors.New("Unexpected non-expression: line " +
			itoa(expr.GetLine()) + " column " + itoa(expr.GetColumn()))
	}
	if expectedType.Name != "" && !isType(dt, expectedType, ns, false) {
		return "", DataType{}, msg(expr.GetLine(), expr.GetColumn(), "Expression has wrong type.")
	}
	return
}

func compileGlobals(globals []GlobalDef, ns *Namespace) (string, error) {
	code := "public class " + GlobalsClass + " {"
	for _, g := range globals {
		c, err := compileType(g.Type, ns)
		if err != nil {
			return "", err
		}
		code += c + " " + g.Name
		if g.Value != nil {
			c, returnedType, err := compileExpression(g.Value, ns, g.Type, map[string]DataType{})
			if err != nil {
				return "", err
			}
			if !isType(returnedType, g.Type, ns, false) {
				return "", msg(g.Line, g.Column, "Initial value of global does not match the declared type.")
			}
			code += " = " + c + ";\n"
		}
	}
	code += "}\n\n"
	return code, nil
}

// assumes a valid data type. Accepts Struct but not a StructDefinition
func compileType(dt DataType, ns *Namespace) (string, error) {
	if dt.Namespace == "" {
		switch dt.Name {
		case "I":
			return "int", nil
		case "F":
			return "float", nil
		case "Byte":
			return "byte", nil
		case "Bool":
			return "bool", nil
		case "Str":
			return "string", nil
		case "Any":
			return "object", nil
		default:
			//
			if !isValidType(dt, ns) {
				return "", msg(dt.Line, dt.Column, "Type is not valid.")
			}
			c := dt.Name
			for i, typeParam := range dt.TypeParams {
				if i == 0 {
					c += "<"
				}
				str, err := compileType(typeParam, ns)
				if err != nil {
					return "", err
				}
				c += str
				if i != len(dt.TypeParams)-1 {
					c += ", "
				} else {
					c += ">"
				}
			}
			return c, nil
		}
	}
	return "", nil
}

func compileIfForm(s IfForm, returnType DataType,
	ns *Namespace, locals map[string]DataType, insideLoop bool, indent string) (string, error) {
	c, conditionType, err := compileExpression(s.Condition, ns, BoolType, locals)
	if err != nil {
		return "", err
	}
	if !isType(conditionType, DataType{Name: "Bool"}, ns, true) {
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
		if !isType(conditionType, DataType{Name: "Bool"}, ns, true) {
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

func compileBody(statements []Statement, returnType DataType,
	ns *Namespace, locals map[string]DataType, insideLoop bool,
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
			newLocals := map[string]DataType{}
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
			c, _, err = compileCallForm(f, ns, DataType{}, locals)
			c = indent + c + ";\n"
		case VarForm:
			if locals[f.Target].Name != "" {
				return "", msg(f.Line, f.Column, "Local variable of same name already exists in this scope.")
			}
			typeStr := "var"
			if f.Type.Name != "" {
				typeStr, err = compileType(f.Type, ns)
				if err != nil {
					return "", err
				}
			}
			valStr := ""
			var exprType DataType
			if f.Value != nil {
				valStr, exprType, err = compileExpression(f.Value, ns, f.Type, locals)
				if err != nil {
					return "", err
				}
				if isZeroType(f.Type) {
					typeStr, err = compileType(exprType, ns)
					if err != nil {
						return "", err
					}
				} else if !isType(exprType, f.Type, ns, false) {
					return "", msg(f.Line, f.Column, "Initial value in var statement is wrong type.")
				}
			}
			if valStr == "" {
				c = indent + typeStr + " _" + f.Target + ";\n"
			} else {
				c = indent + typeStr + " _" + f.Target + " = " + valStr + ";\n"
			}
			if isZeroType(f.Type) {
				locals[f.Target] = exprType
			} else {
				locals[f.Target] = f.Type
			}
		}
		if err != nil {
			return "", err
		}
		code += c
	}
	return code, nil
}

func compileIndexingForm(f IndexingForm, ns *Namespace,
	locals map[string]DataType) (code string, dt DataType, err error) {

	code, dt, err = compileExpression(f.Args[len(f.Args)-1], ns, DataType{}, locals)
	if err != nil {
		return
	}
	for i := len(f.Args) - 2; i >= 0; i-- {
		expr := f.Args[i]
		if IsIndexableType(dt, ns) {
			// todo: implement arrays
		} else {
			if varExpr, ok := expr.(VarExpression); ok {
				if varExpr.Namespace != "" {
					err = msg(varExpr.Line, varExpr.Column, "Improper name in indexing form.")
					return
				}
				dt, ok = GetFieldType(varExpr.Name, dt, ns)
				if !ok {
					err = msg(varExpr.Line, varExpr.Column, "No field called '"+varExpr.Name+"' in indexing form.")
					return
				}
				code += "." + varExpr.Name
			} else {
				err = msg(varExpr.Line, varExpr.Column, "Improper name in indexing form.")
				return
			}
		}
	}
	return code, dt, nil
}

func compileAssignment(f AssignmentForm, ns *Namespace, locals map[string]DataType,
	indent string) (code string, err error) {
	var dt DataType
	switch target := f.Target.(type) {
	case VarExpression:

		isLocal := true
		if target.Namespace == "" {
			dt, isLocal = locals[target.Name]
		}
		if isLocal {
			code = "_" + target.Name
		} else {
			dt, code = getGlobal(target.Name, target.Namespace, ns)
			if code == "" {
				return "", msg(f.Line, f.Column, "Assignment to non-existent variable.")
			}
		}
	case IndexingForm:
		code, dt, err = compileIndexingForm(target, ns, locals)
		if err != nil {
			return "", err
		}
	}

	code += " = "
	exprStr, exprType, err := compileExpression(f.Value, ns, dt, locals)
	if err != nil {
		return "", err
	}
	if !isType(exprType, dt, ns, false) {
		return "", msg(f.Line, f.Column, "Assignment value is wrong type.")
	}
	return indent + code + exprStr + ";\n", nil
}

func compileReturn(f ReturnForm, returnType DataType, ns *Namespace, locals map[string]DataType, indent string) (string, error) {
	code := indent + "return "
	c, exprType, err := compileExpression(f.Value, ns, returnType, locals)
	if err != nil {
		return "", err
	}
	if !isType(exprType, returnType, ns, false) {
		return "", msg(f.Line, f.Column, "Return value is wrong type.")
	}
	code += c + ";\n"
	return code, nil
}

func compileFunc(f FuncDef, ns *Namespace, indent string) (string, error) {
	code := indent + "public static "
	if isZeroType(f.ReturnType) {
		code += "void "
	} else {
		c, err := compileType(f.ReturnType, ns)
		if err != nil {
			return "", err
		}
		code += c + " "
	}
	code += f.Name + "("
	locals := map[string]DataType{}
	for i, paramName := range f.ParamNames {
		paramType := f.ParamTypes[i]
		locals[paramName] = paramType
		c, err := compileType(paramType, ns)
		if err != nil {
			return "", err
		}
		code += "_" + paramName + " " + c
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, f.ReturnType, ns, locals, false, f.ReturnType.Name != "", indent+"\t")
	if err != nil {
		return "", err
	}
	code += body + indent + "}\n"
	return code, nil
}

func compileMethod(f MethodDef, class DataType, ns *Namespace, indent string) (string, error) {
	code := indent + "public "
	if isZeroType(f.ReturnType) {
		code += "void "
	} else {
		c, err := compileType(f.ReturnType, ns)
		if err != nil {
			return "", err
		}
		code += c + " "
	}
	code += f.Name + "("
	locals := map[string]DataType{thisWord: class}
	for i, paramName := range f.ParamNames {
		paramType := f.ParamTypes[i]
		locals[paramName] = paramType
		c, err := compileType(paramType, ns)
		if err != nil {
			return "", err
		}
		code += "_" + paramName + " " + c
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, f.ReturnType, ns, locals, false, f.ReturnType.Name != "", indent+"\t")
	if err != nil {
		return "", err
	}
	code += body + indent + "}\n"
	return code, nil
}

func compileConstructor(f ConstructorDef, class DataType, ns *Namespace, indent string) (string, error) {
	code := indent + "public " + class.Name + "("
	locals := map[string]DataType{thisWord: class}
	for i, paramName := range f.ParamNames {
		paramType := f.ParamTypes[i]
		locals[paramName] = paramType
		c, err := compileType(paramType, ns)
		if err != nil {
			return "", err
		}
		code += "_" + paramName + " " + c
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, class, ns, locals, false, false, indent+"\t")
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
	typeStr, err := compileType(f.Type, ns)
	if err != nil {
		return "", err
	}
	if f.Value != nil {
		exprStr, _, err := compileExpression(f.Value, ns, f.Type, nil)
		if err != nil {
			return "", err
		}
		code += typeStr + " " + f.Name + " = " + exprStr + ";"
	} else {
		code += typeStr + " " + f.Name + ";"
	}
	return code, nil
}

func compileProperty(p PropertyDef, ns *Namespace, indent string) (string, error) {
	// todo
	return "", nil
}

func compileClass(f ClassDef, ns *Namespace, indent string) (string, error) {
	var code string
	switch f.AccessLevel {
	case PublicAccess:
		code = "public "
	case PrivateAccess:
		code = "private "
	case ProtectedAccess:
		code = "protected "
	}
	code += f.Type.Name
	if len(f.Supertypes) > 0 {
		code += " : "
	}
	for i, super := range f.Supertypes {
		typeStr, err := compileType(super, ns)
		if err != nil {
			return "", err
		}
		code += typeStr
		if i != len(f.Supertypes)-1 {
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
		c, err := compileProperty(propertyDef, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, constructorDef := range f.Constructors {
		c, err := compileConstructor(constructorDef, f.Type, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for i, methodDef := range f.Methods {
		c, err := compileMethod(methodDef, f.Type, ns, "\t")
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
	code += f.Type.Name + " "
	for i, inter := range f.Interfaces {
		typeStr, err := compileType(inter, ns)
		if err != nil {
			return "", err
		}
		if i == 0 {
			code += ": "
		} else {
			code += ", "
		}
		code += typeStr
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
		c, err := compileProperty(propertyDef, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, constructorDef := range f.Constructors {
		c, err := compileConstructor(constructorDef, f.Type, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, methodDef := range f.Methods {
		c, err := compileMethod(methodDef, f.Type, ns, "\t")
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	code += "}"
	return code, nil
}

func compileInterface(def InterfaceDef, ns *Namespace, indent string) (string, error) {
	var code string
	switch def.AccessLevel {
	case PublicAccess:
		code = "public "
	case PrivateAccess:
		code = "private "
	case ProtectedAccess:
		code = "protected "
	}
	code += def.Type.Name
	if len(def.ParentInterfaces) > 0 {
		code += " : "
	}
	for i, parent := range def.ParentInterfaces {
		typeStr, err := compileType(parent, ns)
		if err != nil {
			return "", err
		}
		code += typeStr
		if i != len(def.ParentInterfaces)-1 {
			code += ", "
		}
	}
	code += " {\n"
	for i, methodName := range def.MethodNames {
		returnType := def.MethodReturnTypes[i]
		if isZeroType(returnType) {
			code += "\tpublic void " + methodName + "("
		} else {
			c, err := compileType(returnType, ns)
			if err != nil {
				return "", err
			}
			code += "\tpublic " + c + " " + methodName + "("
		}
		for j, param := range def.MethodParams[i] {
			c, err := compileType(param, ns)
			if err != nil {
				return "", err
			}
			if j == len(def.MethodParams)-1 {
				code += c
			} else {
				code += c + ", "
			}
		}
		code += ");\n"
	}
	code += "}\n\n"
	return code, nil
}
