package main

import (
	"errors"
	"math"
	"strconv"
)

func compile(topDefs TopDefs) (string, error) {
	ns, err := createNamespace(topDefs)
	if err != nil {
		return "", err
	}

	code := ""

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

	return code, nil
}

func createNamespace(topDefs TopDefs) (*Namespace, error) {
	namespace := topDefs.Namespace.Name
	interfaces := map[string]*InterfaceInfo{}
	classes := map[string]*ClassInfo{}
	structs := map[string]*StructInfo{}
	globals := map[string]*GlobalInfo{}
	funcs := map[string][]*CallableInfo{}

	fullNames := map[string]string{} // unqualified names -> fully qualified names
	// todo populate classes and fullNames with imports
	// an unqualified import will have two entries in fullNames:
	// e.g. importing foo from namespace bar to namespace ack will have two entries in ack's fullNames:
	// 		foo/bar
	//		foo/ack

	for i, interfaceDef := range topDefs.Interfaces {
		if _, ok := fullNames[interfaceDef.Type.Name]; ok {
			return nil, msg(interfaceDef.Line, interfaceDef.Column, "Interface name already used.")
		}
		topDefs.Interfaces[i].Type.Namespace = namespace
		fullName := interfaceDef.Type.Name + "/" + namespace
		fullNames[interfaceDef.Type.Name] = fullName
		interfaces[fullName] = &InterfaceInfo{
			Name:      interfaceDef.Type.Name,
			Namespace: namespace,
		}
	}

	for i, structDef := range topDefs.Structs {
		if _, ok := fullNames[structDef.Type.Name]; ok {
			return nil, msg(structDef.Line, structDef.Column, "Struct name already used.")
		}
		topDefs.Structs[i].Type.Namespace = namespace
		fullName := structDef.Type.Name + "/" + namespace
		fullNames[structDef.Type.Name] = fullName

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
			interfaceFullName := dt.Name + "/" + dt.Namespace
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

		classes[fullName] = &ClassInfo{
			Name:       class.Type.Name,
			Namespace:  namespace,
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
		fullName := fn.Name + "/" + namespace
		fullNames[fn.Name] = fullName
		funcs[fullName] = append(funcs[fullName],
			&CallableInfo{
				IsMethod:   false,
				ParamNames: fn.ParamNames,
				ParamTypes: fn.ParamTypes,
				ReturnType: fn.ReturnType,
			},
		)
	}

	return &Namespace{
		Name:       namespace,
		Classes:    classes,
		Structs:    structs,
		Interfaces: interfaces,
		Globals:    globals,
		Funcs:      funcs,
		FullNames:  fullNames,
	}, nil
}

func fullName(name string, namespace string, ns *Namespace) string {
	if namespace == "" {
		return ns.FullNames[name]
	}
	return name + "/" + namespace
}

func (dt DataType) fullName(ns *Namespace) string {
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

func isType(child DataType, parent DataType, ns *Namespace, exact bool) bool {
	// child is subtype of builtin type
	if parent.Namespace == "" {
		if parent.Name == "Any" {
			return true
		}
		if child.Namespace != "" {
			return false
		}
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
		return false
	}

	childFull := child.fullName(ns)
	parentFull := parent.fullName(ns)

	if childFull == parentFull {
		return true
	}

	// structs are only subtypes of themselves
	if _, ok := ns.Structs[childFull]; ok {
		return false
	}
	if _, ok := ns.Structs[parentFull]; ok {
		return false
	}

	if parentInterface, ok := ns.Interfaces[parentFull]; ok {
		if childInterface, ok := ns.Interfaces[childFull]; ok {
			return childInterface.isImplementor(parentInterface)
		}

		if childClass, ok := ns.Classes[childFull]; ok {
			return childClass.isImplementor(parentInterface)
		}

		if childStruct, ok := ns.Structs[childFull]; ok {
			return childStruct.isImplementor(parentInterface)
		}

		return false
	}

	if parentClass, ok := ns.Classes[parentFull]; ok {
		if childClass, ok := ns.Classes[childFull]; ok {
			return childClass.isDescendent(parentClass)
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

func (si *StructInfo) isImplementor(ii *InterfaceInfo) bool {
	for _, interfaceInfo := range si.Interfaces {
		if interfaceInfo == ii {
			return true
		}
	}
	return false
}

func (ci *ClassInfo) isImplementor(ii *InterfaceInfo) bool {
	for ; ci != nil; ci = ci.Parent {
		for _, interfaceInfo := range ci.Interfaces {
			if interfaceInfo == ii {
				return true
			}
		}
	}
	return false
}

func (ci *ClassInfo) isDescendent(ancestor *ClassInfo) bool {
	for ; ci != nil; ci = ci.Parent {
		if ci == ancestor {
			return true
		}
	}
	return false
}

func (ci *InterfaceInfo) isImplementor(ii *InterfaceInfo) bool {
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
			code = expr.Name
		}
	case ParsedNumberAtom:
		if expectedType.Name == "" {
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
	case DataType:
		return "", DataType{}, errors.New("Expected expression, got DataType: line " + itoa(expr.Line) + " column " + itoa(expr.Column))
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
				if f.Type.Name == "" {
					typeStr, err = compileType(exprType, ns)
					if err != nil {
						return "", err
					}
				} else if !isType(exprType, f.Type, ns, false) {
					return "", msg(f.Line, f.Column, "Initial value in var statement is wrong type.")
				}
			}
			if valStr == "" {
				c = indent + typeStr + " " + f.Target + ";\n"
			} else {
				c = indent + typeStr + " " + f.Target + " = " + valStr + ";\n"
			}
			if f.Type.Name == "" {
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

func compileAssignment(f AssignmentForm, ns *Namespace, locals map[string]DataType, indent string) (string, error) {
	var code string
	var dt DataType
	if f.Target.Namespace == "" {
		dt = locals[f.Target.Name]
	}
	if dt.Name == "" { // not a local
		dt, code = getGlobal(f.Target.Name, f.Target.Namespace, ns)
		if code == "" {
			return "", msg(f.Line, f.Column, "Assignment to non-existent variable.")
		}
	} else {
		code = f.Target.Name
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
	if f.ReturnType.Name == "" {
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
		code += paramName + " " + c
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
	if f.ReturnType.Name == "" {
		code += "void "
	} else {
		c, err := compileType(f.ReturnType, ns)
		if err != nil {
			return "", err
		}
		code += c + " "
	}
	code += f.Name + "("
	locals := map[string]DataType{"this": class}
	for i, paramName := range f.ParamNames {
		paramType := f.ParamTypes[i]
		locals[paramName] = paramType
		c, err := compileType(paramType, ns)
		if err != nil {
			return "", err
		}
		code += paramName + " " + c
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
	code := "public " + class.Name + "("
	locals := map[string]DataType{"this": class}
	for i, paramName := range f.ParamNames {
		paramType := f.ParamTypes[i]
		locals[paramName] = paramType
		c, err := compileType(paramType, ns)
		if err != nil {
			return "", err
		}
		code += paramName + " " + c
		if i != len(f.ParamNames)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, class, ns, locals, false, false, indent+"\t")
	if err != nil {
		return "", err
	}
	code += body + "}\n"
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
