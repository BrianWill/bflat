package main

import (
	"errors"

	"github.com/davecgh/go-spew/spew"
)

func compile(topDefs TopDefs) (string, error) {

	ns, err := createNamespace(topDefs)
	if err != nil {
		return "", err
	}
	spew.Dump(ns)
	return "", nil
}

func createNamespace(topDefs TopDefs) (*Namespace, error) {
	interfaces := map[string]*InterfaceInfo{}
	classes := map[string]*ClassInfo{}
	structs := map[string]*StructInfo{}
	globals := map[string]*GlobalInfo{}

	fullNames := map[string]string{} // unqualified names -> fully qualified names
	// todo populate classes and fullNames with imports
	// an unqualified import will have two entries in fullNames:
	// e.g. importing foo from namespace bar to namespace ack will have two entries in ack's fullNames:
	// 		foo/bar
	//		foo/ack

	for _, interfaceDef := range topDefs.Interfaces {
		if _, ok := fullNames[interfaceDef.Type.Name]; ok {
			return nil, msg(interfaceDef.Line, interfaceDef.Column, "Interface name already used.")
		}
		fullName := interfaceDef.Type.Name + "/" + topDefs.Namespace
		fullNames[interfaceDef.Type.Name] = fullName
		interfaces[fullName] = &InterfaceInfo{
			Name:      interfaceDef.Type.Name,
			Namespace: topDefs.Namespace,
		}
	}

	for _, structDef := range topDefs.Structs {
		if _, ok := fullNames[structDef.Type.Name]; ok {
			return nil, msg(structDef.Line, structDef.Column, "Struct name already used.")
		}
		fullName := structDef.Type.Name + "/" + topDefs.Namespace
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
			Namespace:  topDefs.Namespace,
			Interfaces: structInterfaces,
		}
	}

	for _, globalInfo := range topDefs.Globals {
		if _, ok := fullNames[globalInfo.Type.Name]; ok {
			return nil, msg(globalInfo.Line, globalInfo.Column, "Global name already used.")
		}
		fullName := globalInfo.Type.Name + "/" + topDefs.Namespace
		fullNames[globalInfo.Type.Name] = fullName
		globals[fullName] = &GlobalInfo{
			Name:      globalInfo.Type.Name,
			Namespace: topDefs.Namespace,
		}
	}

	for _, class := range topDefs.Classes {
		if _, ok := fullNames[class.Type.Name]; ok {
			return nil, msg(class.Line, class.Column, "Class name already used.")
		}
		fullName := class.Type.Name + "/" + topDefs.Namespace
		fullNames[class.Type.Name] = fullName

		classInterfaces := []*InterfaceInfo{}
		for _, dt := range class.Interfaces {
			interfaceFullName := dt.Name + "/" + dt.Namespace
			interfaceInfo, ok := interfaces[interfaceFullName]
			if !ok {
				return nil, msg(class.Line, class.Column, "Struct implements unknown interface.")
			}
			classInterfaces = append(classInterfaces, interfaceInfo)
		}

		classes[fullName] = &ClassInfo{
			Name:       class.Type.Name,
			Namespace:  topDefs.Namespace,
			Interfaces: classInterfaces,
		}
	}
	for _, class := range topDefs.Classes {
		if class.Parent.Name == "" {
			continue
		}
		fullName := class.Type.Name + "/" + topDefs.Namespace
		classInfo := classes[fullName]
		parentInfo := classes[class.Parent.Name+"/"+class.Parent.Namespace]
		if parentInfo == nil {
			return nil, msg(class.Line, class.Column, "Parent class does not exist.")
		}
		classInfo.Parent = parentInfo
	}

	return &Namespace{
		Classes:    classes,
		Structs:    structs,
		Interfaces: interfaces,
		Globals:    globals,
	}, nil

}

func isType(child DataType, parent DataType, ns *Namespace, exact bool) bool {
	if child.Name != parent.Name ||
		child.Namespace != parent.Namespace ||
		len(child.TypeParams) != len(parent.TypeParams) {
		if !isDescendent(child, parent, ns) &&
			!isImplementor(child, parent, ns) {
			return false
		}
	}
	for i, childParam := range child.TypeParams {
		if !isType(childParam, parent.TypeParams[i], ns, exact) {
			return false
		}
	}
	return true
}

func isDescendent(child DataType, parent DataType, ns *Namespace) bool {
	return false // todo
}

func isImplementor(child DataType, interfaceType DataType, ns *Namespace) bool {
	return false // todo
}

func isValidType(dt DataType, ns *Namespace) bool {
	return false
}

// returns data type and csharp name (zero values if no such globals)
func getGlobal(name string, nsStr string, ns *Namespace) (DataType, string) {
	// todo
	return DataType{}, ""
}

// returns signatures and csharp fully-qualified names
func getSignatures(name string, nsStr string, ns *Namespace) ([]Signature, []string) {

	return nil, nil
}

func compileExpression(expr Expression, namespace *Namespace, locals map[string]DataType) (string, DataType, error) {
	var dt DataType
	var code string
	switch expr := expr.(type) {
	case VarExpression:
		dt, code = getGlobal(expr.Name, expr.Namespace, namespace)
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
		// if floating-point, check if within range of float and append 'f'
	case StringAtom:
		return "\"" + expr.Content[1:len(expr.Content)-1] + "\"", StrType, nil
	case CallForm:
		return compileCallForm(expr, namespace, locals)
	case DataType:
		return "", DataType{}, errors.New("Expected expression, got DataType: line " + itoa(expr.Line) + " column " + itoa(expr.Column))
	}
	return code, dt, nil
}

func compileGlobals(globals []GlobalDef, ns *Namespace) (string, error) {
	code := "public class _Globals {"
	for _, g := range globals {
		c, err := compileType(g.Type, ns)
		if err != nil {
			return "", err
		}
		code += c + " " + g.Name
		if g.Value != nil {
			c, returnedType, err := compileExpression(g.Value, ns, map[string]DataType{})
			if err != nil {
				return "", err
			}
			if !isType(returnedType, g.Type, ns, false) {
				return "", msg(g.Line, g.Column, "Initial value of global does not match the declared type.")
			}
			code += " = " + c + ";\n"
		}
	}
	code += "}\n"
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
		}
	}
	return "", nil
}

func compileIfForm(s IfForm, returnType DataType,
	ns *Namespace, locals map[string]DataType, insideLoop bool) (string, error) {
	c, conditionType, err := compileExpression(s.Condition, ns, locals)
	if err != nil {
		return "", err
	}
	if !isType(conditionType, DataType{Name: "Bool"}, ns, true) {
		return "", msg(s.Line, s.Column, "The 'if' condition must return a boolean.")
	}
	code := "if (" + c + ") {\n"
	c, err = compileBody(s.Body, returnType, ns, locals, insideLoop, false)
	if err != nil {
		return "", nil
	}
	code += c + "\n}"
	for i, elif := range s.ElifConds {
		c, conditionType, err := compileExpression(elif, ns, locals)
		if err != nil {
			return "", err
		}
		if !isType(conditionType, DataType{Name: "Bool"}, ns, true) {
			return "", msg(s.Line, s.Column, "Elif condition expression does not return a boolean.")
		}
		code += " else if (" + c + ") {\n"
		c, err = compileBody(s.ElifBodies[i], returnType, ns, locals, insideLoop, false)
		if err != nil {
			return "", err
		}
		code += c + "}"
	}
	if len(s.ElseBody) > 0 {
		c, err := compileBody(s.ElseBody, returnType, ns, locals, insideLoop, false)
		if err != nil {
			return "", err
		}
		code += " else {\n" + c + "}"
	}
	return code + "\n", nil
}

func compileBody(statements []Statement, returnType DataType,
	ns *Namespace, locals map[string]DataType, insideLoop bool, requiresReturn bool) (string, error) {
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
			c, err = compileIfForm(f, returnType, ns, locals, insideLoop)
		case AssignmentForm:
			c, err = compileAssignment(f, ns, locals)
		case ReturnForm:
			c, err = compileReturn(f, returnType, ns, locals)
		case BreakForm:
			if insideLoop {
				c += "break \n"
			} else {
				err = msg(f.Line, f.Column, "cannot have break statement outside a loop.")
			}
		case ContinueForm:
			if insideLoop {
				c += "continue \n"
			} else {
				err = msg(f.Line, f.Column, "cannot have continue statement outside a loop.")
			}
		case CallForm:
			c, _, err = compileCallForm(f, ns, locals)
			c += ";\n"
		case VarForm:
			code, err := compileType(f.Type, ns)
			if err != nil {
				return "", err
			}
			if locals[f.Target].Name != "" {
				return "", msg(f.Line, f.Column, "Local variable of same name already exists in this scope.")
			}
			code += " " + f.Target
			if f.Value != nil {
				c, dt, err := compileExpression(f.Value, ns, locals)
				if err != nil {
					return "", err
				}
				if !isType(dt, f.Type, ns, false) {
					return "", msg(f.Line, f.Column, "Initial value in var statement is wrong type.")
				}
				code += " " + c
			}
			code += ";"
		}
		if err != nil {
			return "", err
		}
		code += c
	}
	return code, nil
}

func compileAssignment(f AssignmentForm, ns *Namespace, locals map[string]DataType) (string, error) {
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
	}
	code += " = "
	c, exprType, err := compileExpression(f.Value, ns, locals)
	if err != nil {
		return "", err
	}
	if !isType(exprType, dt, ns, false) {
		return "", msg(f.Line, f.Column, "Assignment value is wrong type.")
	}
	code += c + ";"
	return code, nil
}

func compileReturn(f ReturnForm, returnType DataType, ns *Namespace, locals map[string]DataType) (string, error) {
	code := "return "
	c, exprType, err := compileExpression(f.Value, ns, locals)
	if err != nil {
		return "", err
	}
	if !isType(exprType, returnType, ns, false) {
		return "", msg(f.Line, f.Column, "Return value is wrong type.")
	}
	code += c + ";"
	return code, nil
}

func compileFunc(f FuncDef, ns *Namespace) (string, error) {
	code := "public static "
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
	var locals map[string]DataType
	for i, param := range f.Params {
		locals[param.Name] = param.Type
		c, err := compileType(param.Type, ns)
		if err != nil {
			return "", err
		}
		code += param.Name + " " + c
		if i != len(f.Params)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, f.ReturnType, ns, locals, false, f.ReturnType.Name != "")
	if err != nil {
		return "", err
	}
	code += body + "}\n"
	return "", nil
}

func compileMethod(f MethodDef, class DataType, ns *Namespace) (string, error) {
	code := "public "
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
	for i, param := range f.Params {
		locals[param.Name] = param.Type
		c, err := compileType(param.Type, ns)
		if err != nil {
			return "", err
		}
		code += param.Name + " " + c
		if i != len(f.Params)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, f.ReturnType, ns, locals, false, f.ReturnType.Name != "")
	if err != nil {
		return "", err
	}
	code += body + "}\n"
	return "", nil
}

func compileConstructor(f ConstructorDef, class DataType, ns *Namespace) (string, error) {
	code := "public " + class.Name + "("
	locals := map[string]DataType{"this": class}
	for i, param := range f.Params {
		locals[param.Name] = param.Type
		c, err := compileType(param.Type, ns)
		if err != nil {
			return "", err
		}
		code += param.Name + " " + c
		if i != len(f.Params)-1 {
			code += ", "
		}
	}
	code += ") {\n"
	body, err := compileBody(f.Body, class, ns, locals, false, false)
	if err != nil {
		return "", err
	}
	code += body + "}\n"
	return "", nil
}

func compileField(f FieldDef, ns *Namespace) (string, error) {
	var code string
	switch f.AccessLevel {
	case PublicAccess:
		code = "public "
	case PrivateAccess:
		code = "private "
	case ProtectedAccess:
		code = "protected "
	}
	typeStr, err := compileType(f.Type, ns)
	if err != nil {
		return "", err
	}
	code += typeStr + " " + f.Name + ";"
	return code, nil
}

func compileProperty(p PropertyDef, ns *Namespace) (string, error) {
	// todo
	return "", nil
}

func compileClass(f ClassDef, ns *Namespace) (string, error) {
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
	if f.Parent.Name != "" {
		typeStr, err := compileType(f.Parent, ns)
		if err != nil {
			return "", err
		}
		code += ": " + typeStr + " "
	}
	for i, inter := range f.Interfaces {
		typeStr, err := compileType(inter, ns)
		if err != nil {
			return "", err
		}
		if i == 0 {
			if f.Parent.Name == "" {
				code += ": "
			}
		} else {
			code += ", "
		}
		code += typeStr
	}
	code += " {"
	for _, fieldDef := range f.Fields {
		c, err := compileField(fieldDef, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, propertyDef := range f.Properties {
		c, err := compileProperty(propertyDef, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, constructorDef := range f.Constructors {
		c, err := compileConstructor(constructorDef, f.Type, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, methodDef := range f.Methods {
		c, err := compileMethod(methodDef, f.Type, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	code += "}"
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
		c, err := compileField(fieldDef, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, propertyDef := range f.Properties {
		c, err := compileProperty(propertyDef, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, constructorDef := range f.Constructors {
		c, err := compileConstructor(constructorDef, f.Type, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	for _, methodDef := range f.Methods {
		c, err := compileMethod(methodDef, f.Type, ns)
		if err != nil {
			return "", err
		}
		code += c + "\n"
	}
	code += "}"
	return code, nil
}
