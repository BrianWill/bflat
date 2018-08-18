package main

import (
	"errors"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

func parse(readerData []Atom) (TopDefs, error) {
	topDefs := TopDefs{
		Classes: []ClassDef{},
		Structs: []StructDef{},
		Funcs:   []FuncDef{},
		Globals: []GlobalDef{},
	}

	annotations := []AnnotationForm{}

	for _, atom := range readerData {
		switch atom := atom.(type) {
		case ParenList:
			elems := atom.Atoms
			if len(elems) == 0 {
				return TopDefs{}, errors.New("Invalid top-level atom: " + spew.Sdump(atom))
			}
			if sigil, ok := elems[0].(SigilAtom); ok {
				if sigil.Content != "@" {
					return TopDefs{}, errors.New("Invalid top-level atom: " + spew.Sdump(atom))
				}
				annotation, err := parseAnnotation(atom)
				if err != nil {
					return TopDefs{}, err
				}
				annotations = append(annotations, annotation)
				continue
			}
			first, ok := elems[0].(Symbol)
			if !ok {
				return TopDefs{}, errors.New("Invalid top-level atom: " + spew.Sdump(atom))
			}
			switch first.Content {
			case "class":
				class, err := parseClass(atom, annotations)
				if err != nil {
					return TopDefs{}, err
				}
				topDefs.Classes = append(topDefs.Classes, class)
				annotations = []AnnotationForm{} // reset to empty slice
			case "struct":
				structDef, err := parseStruct(atom, annotations, false)
				if err != nil {
					return TopDefs{}, err
				}
				topDefs.Structs = append(topDefs.Structs, structDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "func":
				funcDef, err := parseFunc(atom, annotations)
				if err != nil {
					return TopDefs{}, err
				}
				topDefs.Funcs = append(topDefs.Funcs, funcDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "interface":
				interfaceDef, err := parseInterface(atom, annotations)
				if err != nil {
					return TopDefs{}, err
				}
				topDefs.Interfaces = append(topDefs.Interfaces, interfaceDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "global":
				global, err := parseGlobal(atom, annotations)
				if err != nil {
					return TopDefs{}, err
				}
				topDefs.Globals = append(topDefs.Globals, global)
				annotations = []AnnotationForm{} // reset to empty slice
			case "ns":
				if topDefs.Namespace.Name != "" {
					return TopDefs{}, errors.New("Cannot have more than one namespace declaration in a file: " + spew.Sdump(atom))
				}
				ns, err := parseNamespaceDef(atom, annotations)
				if err != nil {
					return TopDefs{}, err
				}
				topDefs.Namespace = ns
				annotations = []AnnotationForm{} // reset to empty slice
			default:
				return TopDefs{}, errors.New("Invalid top-level atom: " + spew.Sdump(atom))
			}
		default:
			return TopDefs{}, errors.New("Invalid top-level atom: " + spew.Sdump(atom))
		}
	}
	return topDefs, nil
}

// assumes first atom is @ sigil
func parseAnnotation(parens ParenList) (AnnotationForm, error) {
	args := []Expression{}
	for _, a := range parens.Atoms[1:] {
		expr, err := parseExpression(a)
		if err != nil {
			return AnnotationForm{}, err
		}
		args = append(args, expr)
	}
	return AnnotationForm{}, nil
}

// parse (potentially) qualified name
func parseVarExpression(atom Atom) (VarExpression, error) {
	expr := VarExpression{}
	switch atom := atom.(type) {
	case Symbol:
		if atom.Content == strings.Title(atom.Content) {
			return VarExpression{}, errors.New("Invalid name (cannot begin with uppercase): " + spew.Sdump(atom))
		}
		expr.Name = atom.Content
	case AtomChain:
		atoms := atom.Atoms
		if len(atoms) == 0 {
			return VarExpression{}, errors.New("Invalid name: " + spew.Sdump(atom))
		}
		if symbol, ok := atoms[0].(Symbol); ok {
			if symbol.Content == strings.Title(symbol.Content) {
				return VarExpression{}, errors.New("Invalid name (cannot begin with uppercase): " + spew.Sdump(atom))
			}
			expr.Name = symbol.Content
			namespace, err := parseNamespace(atoms[1:], symbol.Line, symbol.Column)
			if err != nil {
				return VarExpression{}, errors.New("Invalid name: " + spew.Sdump(atom))
			}
			expr.Namespace = namespace
		} else {
			return VarExpression{}, errors.New("Invalid name (expecting symbol): " + spew.Sdump(atom))
		}
	default:
		return VarExpression{}, errors.New("Invalid name: " + spew.Sdump(atom))
	}
	return expr, nil
}

func parseClass(parens ParenList, annotations []AnnotationForm) (ClassDef, error) {
	structDef, err := parseStruct(parens, annotations, true)
	if err != nil {
		return ClassDef{}, err
	}
	return ClassDef{
		Line:         structDef.Line,
		Column:       structDef.Column,
		Type:         structDef.Type,
		AccessLevel:  structDef.AccessLevel,
		Supertypes:   structDef.Interfaces,
		Fields:       structDef.Fields,
		Methods:      structDef.Methods,
		Constructors: structDef.Constructors,
		Properties:   structDef.Properties,
		Annotations:  structDef.Annotations,
	}, nil
}

func parseStruct(parens ParenList, annotations []AnnotationForm, isClass bool) (StructDef, error) {
	structOrClass := "struct"
	if isClass {
		structOrClass = "class"
	}
	structDef := StructDef{
		Annotations: annotations,
		AccessLevel: PublicAccess,
	}
	elems := parens.Atoms
	if len(elems) < 2 {
		return StructDef{}, errors.New(strings.Title(structOrClass) + " must have a name: line " + itoa(parens.Line))
	}
	idx := 1
	// parse -priv or -prot flag (if found)
	if atomChain, ok := elems[idx].(AtomChain); ok {
		if sigil, ok := atomChain.Atoms[0].(SigilAtom); ok {
			if sigil.Content == "-" {
				if symbol, ok := atomChain.Atoms[1].(Symbol); ok {
					switch symbol.Content {
					case "priv":
						structDef.AccessLevel = PrivateAccess
					case "prot":
						structDef.AccessLevel = ProtectedAccess
					default:
						return StructDef{}, errors.New("Invalid atom in " + structOrClass + ": line " + itoa(parens.Line) + " column: " + itoa(atomChain.Column))
					}
				}
				idx++
			}
		}
	}
	dataType, err := parseDataType(elems[idx])
	if err != nil {
		return StructDef{}, errors.New(strings.Title(structOrClass) + " has invalid name: " + err.Error())
	}
	structDef.Type = dataType
	idx++
	if idx >= len(elems) {
		return structDef, nil
	}
	if sigil, ok := elems[idx].(SigilAtom); ok {
		if sigil.Content == ":" {
			idx++
			for {
				if idx >= len(elems) {
					break
				}
				dt, err := parseDataType(elems[idx])
				if err != nil {
					break
				}
				structDef.Interfaces = append(structDef.Interfaces, dt)
				idx++
			}
			if len(structDef.Interfaces) == 0 {
				return StructDef{}, errors.New(strings.Title(structOrClass) + " expects at least one interface after colon: " + spew.Sdump(parens))
			}
		}
	}
	annotations = []AnnotationForm{}
	for _, atom := range elems[idx:] {
		switch atom := atom.(type) {
		case ParenList:
			atoms := atom.Atoms
			if len(atoms) == 0 {
				return StructDef{}, errors.New("Invalid " + structOrClass + " member: " + spew.Sdump(atom))
			}
			if sigil, ok := atoms[0].(SigilAtom); ok {
				if sigil.Content != "@" {
					return StructDef{}, errors.New("Invalid " + structOrClass + " member: " + spew.Sdump(atom))
				}
				annotation, err := parseAnnotation(atom)
				if err != nil {
					return StructDef{}, err
				}
				annotations = append(annotations, annotation)
				continue
			}

			first, ok := atoms[0].(Symbol)
			if !ok {
				return StructDef{}, errors.New("Invalid " + structOrClass + " member: " + spew.Sdump(atom))
			}
			switch first.Content {
			case "f":
				field, err := parseField(atom, annotations)
				if err != nil {
					return StructDef{}, err
				}
				structDef.Fields = append(structDef.Fields, field)
				annotations = []AnnotationForm{} // reset to empty slice
			case "m":
				methodDef, err := parseMethod(atom, annotations)
				if err != nil {
					return StructDef{}, err
				}
				structDef.Methods = append(structDef.Methods, methodDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "p":
				property, err := parseProperty(atom, annotations)
				if err != nil {
					return StructDef{}, err
				}
				structDef.Properties = append(structDef.Properties, property)
				annotations = []AnnotationForm{} // reset to empty slice
			case "constructor":
				constructor, err := parseConstructor(atom, annotations)
				if err != nil {
					return StructDef{}, err
				}
				structDef.Constructors = append(structDef.Constructors, constructor)
				annotations = []AnnotationForm{} // reset to empty slice
			default:
				return StructDef{}, errors.New("Invalid " + structOrClass + " member: " + spew.Sdump(atom))
			}
		default:
			return StructDef{}, errors.New("Invalid " + structOrClass + " member: " + spew.Sdump(atom))
		}
	}
	return structDef, nil
}

func parseField(parens ParenList, annotations []AnnotationForm) (FieldDef, error) {
	field := FieldDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	atoms := parens.Atoms
	if len(atoms) < 3 {
		return FieldDef{}, errors.New("Invalid field: " + spew.Sdump(parens))
	}
	symbol, ok := atoms[1].(Symbol)
	if !ok {
		return FieldDef{}, errors.New("Expecting field name: " + spew.Sdump(parens))
	}
	field.Name = symbol.Content
	DataType, err := parseDataType(atoms[2])
	if err != nil {
		return FieldDef{}, err
	}
	field.Type = DataType
	if len(atoms) == 4 {
		expr, err := parseExpression(atoms[3])
		if err != nil {
			return FieldDef{}, err
		}
		field.Value = expr
	} else if len(atoms) > 4 {
		return FieldDef{}, errors.New("Too many atoms in field: " + spew.Sdump(parens))
	}
	return field, nil
}

func parseDataType(atom Atom) (DataType, error) {
	dataType := DataType{}
	switch atom := atom.(type) {
	case Symbol:
		if atom.Content != strings.Title(atom.Content) {
			return DataType{}, errors.New("Type name must begin with capital letter")
		}
		dataType.Name = atom.Content
		dataType.Line = atom.Line
		dataType.Column = atom.Column
	case AtomChain:
		atoms := atom.Atoms
		if len(atoms) < 1 {
			return DataType{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
		}
		if symbol, ok := atoms[0].(Symbol); ok {
			if symbol.Content != strings.Title(symbol.Content) {
				return DataType{}, errors.New("Type name must begin with capital letter")
			}
			dataType.Name = symbol.Content
		}
		if len(atoms) < 2 {
			return DataType{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
		}
		switch second := atoms[1].(type) {
		case AngleList:
			angleAtoms := second.Atoms
			if len(angleAtoms) == 0 {
				return DataType{}, errors.New("Invalid type spec (empty angle brackets): " + spew.Sdump(atom))
			}
			dataType.TypeParams = []DataType{}
			for _, typeAtom := range angleAtoms {
				ts, err := parseDataType(typeAtom)
				if err != nil {
					return DataType{}, err
				}
				dataType.TypeParams = append(dataType.TypeParams, ts)
			}
			if len(atoms) < 3 {
				break
			}
			namespace, err := parseNamespace(atoms[2:], atom.Line, atom.Column)
			if err != nil {
				return DataType{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
			}
			dataType.Namespace = namespace
		case SigilAtom:
			namespace, err := parseNamespace(atoms[1:], atom.Line, atom.Column)
			if err != nil {
				return DataType{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
			}
			dataType.Namespace = namespace
		default:
		}
	default:
		return DataType{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
	}
	return dataType, nil
}

func parseNamespaceDef(parens ParenList, annotations []AnnotationForm) (NamespaceDef, error) {
	atoms := parens.Atoms
	if len(atoms) != 2 {
		return NamespaceDef{}, errors.New("Invalid namespace form. Too many atoms. " + spew.Sdump(atoms))
	}
	symbol, ok := atoms[1].(Symbol)
	if !ok {
		return NamespaceDef{}, errors.New("Invalid namespace form. Expecting symbol. " + spew.Sdump(atoms))
	}
	if symbol.Content == strings.Title(symbol.Content) {
		return NamespaceDef{}, errors.New("Invalid namespace form: name cannot start with uppercase letter. " + spew.Sdump(atoms))
	}
	return NamespaceDef{
		Name:        symbol.Content,
		Line:        symbol.Line,
		Column:      symbol.Column,
		Annotations: annotations,
	}, nil
}

// expects / sigil followed by one or more symbols separated by dots
func parseNamespace(atoms []Atom, line int, column int) (string, error) {
	if len(atoms) < 2 {
		return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}
	if sigil, ok := atoms[0].(SigilAtom); ok {
		if sigil.Content != "/" {
			return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
		}
	} else {
		return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}
	strs := []string{}
	for i, atom := range atoms[1:] {
		if i%2 == 0 {
			if symbol, ok := atom.(Symbol); ok {
				if symbol.Content != strings.Title(symbol.Content) {
					return "", errors.New("Improperly formed namespace qualifier (namspace cannot begin with uppercase): line " + itoa(line) + " column " + itoa(column))
				}
				strs = append(strs, symbol.Content)
			} else {
				return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
			}
		} else {
			if sigil, ok := atom.(SigilAtom); ok {
				if sigil.Content != "." {
					return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
				}
			} else {
				return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
			}
		}
	}
	return strings.Join(strs, "."), nil
}

func parseExpression(atom Atom) (Expression, error) {
	varExpr, err := parseVarExpression(atom)
	if err == nil { // if no error, then VarExpression
		return varExpr, nil
	}
	var expr Expression
	switch atom := atom.(type) {
	case NumberAtom:
		expr = ParsedNumberAtom{
			IntegerPart: atom.Content,
			Line:        atom.Line,
			Column:      atom.Column,
		}
	case StringAtom:
		expr = atom
	case AtomChain:
		// test if atom's a valid number, e.g. -35.98
		elems := atom.Atoms
		if len(elems) < 1 {
			return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
		}
		// optional leading -
		idx := 0
		integerPart := ""
		if sigil, ok := elems[0].(SigilAtom); ok {
			if sigil.Content == "-" {
				integerPart += "-"
				idx++
				if len(elems) < 2 {
					return nil, errors.New("Invalid expression (unexpected sigil): " + spew.Sdump(atom))
				}
			} else {
				return nil, errors.New("Invalid expression (unexpected sigil): " + spew.Sdump(atom))
			}
		}
		// expecting number
		if num, ok := elems[idx].(NumberAtom); ok {
			integerPart += num.Content
			idx++
		} else {
			return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
		}
		fractionalPart := ""
		if idx < len(elems) {
			// optional dot followed by number
			if sigil, ok := elems[idx].(SigilAtom); ok {
				if sigil.Content != "." {
					return nil, errors.New("Invalid expression (unexpected sigil): " + spew.Sdump(atom))
				}
			} else {
				return nil, errors.New("Invalid expression (expected .): " + spew.Sdump(elems[idx]))
			}
			idx++
			if idx >= len(elems) {
				return nil, errors.New("Invalid number literal: expecting fractional part after decimal point: " + spew.Sdump(elems[idx]))
			}
			if num, ok := elems[idx].(NumberAtom); ok {
				fractionalPart = num.Content
			} else {
				return nil, errors.New("Invalid expression (expected number): " + spew.Sdump(elems[idx]))
			}
			idx++
		}
		if idx < len(elems) {
			return nil, errors.New("Invalid number literal (unexpected atoms): " + spew.Sdump(elems[idx]))
		}
		return ParsedNumberAtom{
			IntegerPart:    integerPart,
			FractionalPart: fractionalPart,
			Line:           atom.Line,
			Column:         atom.Column,
		}, nil
	case SquareList:
		expr, err = parseIndexing(atom, atom.Line, atom.Column)
		if err != nil {
			return nil, err
		}
	case ParenList:
		atoms := atom.Atoms
		if len(atoms) == 0 {
			return nil, errors.New("Invalid expression (empty parens): " + spew.Sdump(atom))
		}
		args := make([]Expression, len(atoms)-1)
		for i, a := range atoms[1:] {
			expr, err := parseExpression(a)
			if err != nil {
				return nil, err
			}
			args[i] = expr
		}
		varExpr, err := parseVarExpression(atoms[0])
		if err != nil {
			dt, err := parseDataType(atoms[0])
			if err != nil {
				return nil, errors.New("Invalid expression (expecting name or type): " + spew.Sdump(atom))
			}
			expr = TypeCallForm{
				Line:   atom.Line,
				Column: atom.Column,
				Type:   dt,
				Args:   args,
			}
		} else {
			expr = CallForm{
				Line:      atom.Line,
				Column:    atom.Column,
				Name:      varExpr.Name,
				Namespace: varExpr.Namespace,
				Args:      args,
			}
		}
	default:
		return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
	}
	return expr, nil
}

func parseMethod(parens ParenList, annotations []AnnotationForm) (MethodDef, error) {
	methodDef := MethodDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	atoms := parens.Atoms
	if len(atoms) < 2 {
		return MethodDef{}, errors.New("Invalid method definition: " + spew.Sdump(parens))
	}
	if symbol, ok := atoms[1].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return MethodDef{}, errors.New("Invalid method name (cannot begin with uppercase): " + spew.Sdump(symbol))
		}
		methodDef.Name = symbol.Content
	}
	if len(atoms) < 3 {
		return methodDef, nil
	}
	idx := 2
	dt, err := parseDataType(atoms[idx])
	if err == nil {
		methodDef.ReturnType = dt
		idx++
	}
	if len(atoms) <= idx {
		return MethodDef{}, errors.New("Incomplete method definition: " + spew.Sdump(parens))
	}
	// params
	if sigil, ok := atoms[idx].(SigilAtom); ok {
		if sigil.Content != ":" {
			return MethodDef{}, errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
		}
		idx++
		paramNames := []string{}
		paramTypes := []DataType{}
		for idx+1 < len(atoms) {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			dt, err := parseDataType(atoms[idx+1])
			if err != nil {
				return MethodDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			paramNames = append(paramNames, symbol.Content)
			paramTypes = append(paramTypes, dt)
			idx += 2
		}
		methodDef.ParamNames = paramNames
		methodDef.ParamTypes = paramTypes
	}
	stmts, err := parseBody(atoms[idx:])
	if err != nil {
		return MethodDef{}, err
	}
	methodDef.Body = stmts
	return methodDef, nil
}

func parseProperty(parens ParenList, annotations []AnnotationForm) (PropertyDef, error) {
	atoms := parens.Atoms
	splitIdx := len(atoms)
	if len(atoms) < 4 {
		return PropertyDef{}, msg(parens.Line, parens.Column, "Too few atoms in property form.")
	}
	propertyDef := PropertyDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	if symbol, ok := atoms[1].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return PropertyDef{}, errors.New("Invalid property name (cannot begin with uppercase): " + spew.Sdump(symbol))
		}
		propertyDef.Name = symbol.Content
	}
	for i, atom := range atoms[3:] {
		if chain, ok := atom.(AtomChain); ok {
			if len(chain.Atoms) == 2 {
				if sigil, ok := chain.Atoms[0].(SigilAtom); ok {
					if sigil.Content == "-" {
						if symbol, ok := chain.Atoms[1].(Symbol); ok {
							if symbol.Content == "set" {
								splitIdx = i + 1
							}
						}
					}
				}
			}
		}
	}
	getBody, err := parseBody(atoms[:splitIdx])
	if err != nil {
		return PropertyDef{}, err
	}
	setBody, err := parseBody(atoms[splitIdx:])
	if err != nil {
		return PropertyDef{}, err
	}
	propertyDef.GetBody = getBody
	propertyDef.SetBody = setBody
	return propertyDef, nil
}

func parseConstructor(parens ParenList, annotations []AnnotationForm) (ConstructorDef, error) {
	constructorDef := ConstructorDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	atoms := parens.Atoms
	if len(atoms) <= 2 {
		return ConstructorDef{}, errors.New("Incomplete constructor definition: " + spew.Sdump(parens))
	}
	idx := 1
	// params
	if sigil, ok := atoms[idx].(SigilAtom); ok {
		if sigil.Content != ":" {
			return ConstructorDef{}, errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
		}
		idx++
		paramNames := []string{}
		paramTypes := []DataType{}
		for idx+1 < len(atoms) {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			dt, err := parseDataType(atoms[idx+1])
			if err != nil {
				return ConstructorDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			paramNames = append(paramNames, symbol.Content)
			paramTypes = append(paramTypes, dt)
			idx += 2
		}
		constructorDef.ParamNames = paramNames
		constructorDef.ParamTypes = paramTypes
	}
	stmts, err := parseBody(atoms[idx:])
	if err != nil {
		return ConstructorDef{}, err
	}
	constructorDef.Body = stmts
	return constructorDef, nil
}

func parseFunc(parens ParenList, annotations []AnnotationForm) (FuncDef, error) {
	// assume first atom is 'func' symbol
	funcDef := FuncDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	atoms := parens.Atoms
	if len(atoms) < 2 {
		return FuncDef{}, errors.New("Invalid function definition: " + spew.Sdump(parens))
	}
	if symbol, ok := atoms[1].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return FuncDef{}, errors.New("Invalid func name (cannot begin with uppercase): " + spew.Sdump(parens))
		}
		funcDef.Name = symbol.Content
	}
	if len(atoms) < 3 {
		return funcDef, nil
	}
	idx := 2
	dt, err := parseDataType(atoms[idx])
	if err == nil {
		funcDef.ReturnType = dt
		idx++
	}
	if len(atoms) <= idx {
		return FuncDef{}, errors.New("Incomplete function definition: " + spew.Sdump(parens))
	}
	// params
	if sigil, ok := atoms[idx].(SigilAtom); ok {
		if sigil.Content != ":" {
			return FuncDef{}, errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
		}
		idx++
		paramNames := []string{}
		paramTypes := []DataType{}
		for idx+1 < len(atoms) {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			dt, err := parseDataType(atoms[idx+1])
			if err != nil {
				return FuncDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			paramNames = append(paramNames, symbol.Content)
			paramTypes = append(paramTypes, dt)
			idx += 2
		}
		funcDef.ParamNames = paramNames
		funcDef.ParamTypes = paramTypes
	}
	stmts, err := parseBody(atoms[idx:])
	if err != nil {
		return FuncDef{}, err
	}
	funcDef.Body = stmts
	return funcDef, nil
}

func parseBody(atoms []Atom) ([]Statement, error) {
	stmts := []Statement{}
	for i := 0; i < len(atoms); {
		atom := atoms[i]
		parens, ok := atom.(ParenList)
		if !ok {
			return nil, errors.New("Expecting parentheses in body: " + spew.Sdump(parens))
		}
		if len(parens.Atoms) == 0 {
			return nil, errors.New("Expecting non-empty parentheses in body: " + spew.Sdump(parens))
		}
		elems := parens.Atoms
		if symbol, ok := elems[0].(Symbol); ok {
			var stmt Statement
			var err error
			n := 1
			switch symbol.Content {
			case "if":
				stmt, n, err = parseIf(atoms[i:])
			case "for":
				stmt, err = parseFor(elems, symbol.Line, symbol.Column)
			case "return":
				stmt, err = parseReturn(elems, symbol.Line, symbol.Column)
			case "switch":
				stmt, n, err = parseSwitch(atoms[i:])
			case "throw":
				stmt, err = parseThrow(elems, symbol.Line, symbol.Column)
			case "try":
				stmt, n, err = parseTry(atoms[i:])
			case "break":
				stmt, err = parseBreak(elems, symbol.Line, symbol.Column)
			case "continue":
				stmt, err = parseContinue(elems, symbol.Line, symbol.Column)
			case "var":
				stmt, err = parseVar(elems, symbol.Line, symbol.Column)
			case "as":
				stmt, err = parseAssignment(elems, symbol.Line, symbol.Column)
			default:
				expr, err := parseExpression(atoms[i])
				if err != nil {
					return nil, err
				}
				call, ok := expr.(CallForm)
				if !ok {
					return nil, errors.New("Improper expression as statement: " + spew.Sdump(atoms[i]))
				}
				stmt = call
			}
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, stmt)
			i += n
			continue
		}
		// might be a call with a qualified name
		expr, err := parseExpression(atom)
		if err != nil {
			return nil, errors.New("Invalid statment: " + spew.Sdump(atoms))
		}
		stmts = append(stmts, expr.(CallForm))
		i++
	}
	return stmts, nil
}

func parseIf(atoms []Atom) (IfForm, int, error) {
	ifForm := IfForm{}
	// parse if clause
	ifAtoms := atoms[0].(ParenList).Atoms
	if len(ifAtoms) < 2 {
		return IfForm{}, 0, errors.New("Invalid if form (expecting condition): " + spew.Sdump(atoms))
	}
	var err error
	ifForm.Condition, err = parseExpression(ifAtoms[1])
	if err != nil {
		return IfForm{}, 0, err
	}
	ifForm.Body, err = parseBody(ifAtoms[2:])
	if err != nil {
		return IfForm{}, 0, err
	}
	// parse elif clauses and else clause
	n := 1
Loop:
	for _, atom := range atoms {
		parens, ok := atom.(ParenList)
		if !ok {
			break Loop
		}
		if len(parens.Atoms) == 0 {
			break Loop
		}
		elems := parens.Atoms
		symbol, ok := elems[0].(Symbol)
		if !ok {
			break Loop
		}
		switch symbol.Content {
		case "elif":
			if len(elems) < 2 {
				return IfForm{}, 0, errors.New("Invalid elif clause (expecting condition): " + spew.Sdump(elems))
			}
			condition, err := parseExpression(elems[1])
			if err != nil {
				return IfForm{}, 0, err
			}
			ifForm.ElifConds = append(ifForm.ElifConds, condition)
			body, err := parseBody(elems[2:])
			if err != nil {
				return IfForm{}, 0, err
			}
			ifForm.ElifBodies = append(ifForm.ElifBodies, body)
			n++
		case "else":
			ifForm.ElseBody, err = parseBody(elems[1:])
			if err != nil {
				return IfForm{}, 0, err
			}
			n++
			break Loop
		default:
			break Loop
		}
	}
	return ifForm, n, nil
}

func parseIndexing(square SquareList, line int, column int) (IndexingForm, error) {
	atoms := square.Atoms
	if len(atoms) < 1 {
		return IndexingForm{}, msg(line, column, "Indexing expression cannot be empty square brackets.")
	}
	args := make([]Expression, len(atoms))
	for i, a := range atoms {
		expr, err := parseExpression(a)
		if err != nil {
			return IndexingForm{}, err
		}
		args[i] = expr
	}
	if len(atoms) == 1 {
		args = append(args, VarExpression{
			Line:      square.Line,
			Column:    square.Column,
			Name:      thisWord,
			Namespace: "",
		})
	}
	return IndexingForm{
		Line:   square.Line,
		Column: square.Column,
		Args:   args,
	}, nil
}

func parseAssignment(atoms []Atom, line int, column int) (AssignmentForm, error) {
	if len(atoms) != 3 {
		return AssignmentForm{}, errors.New("Assignment statement has wrong number of elements: " + spew.Sdump(atoms))
	}
	var target Target
	var err error
	targetAtom := atoms[1]
	if square, ok := targetAtom.(SquareList); ok {
		target, err = parseIndexing(square, line, column)
	} else {
		target, err = parseVarExpression(targetAtom)
	}
	if err != nil {
		return AssignmentForm{}, err
	}
	value, err := parseExpression(atoms[2])
	if err != nil {
		return AssignmentForm{}, err
	}
	return AssignmentForm{
		Line:   line,
		Column: column,
		Target: target,
		Value:  value,
	}, nil
}

func parseReturn(atoms []Atom, line int, column int) (ReturnForm, error) {
	if len(atoms) != 2 {
		return ReturnForm{}, msg(line, column, "Return statement has wrong number of elements.")
	}
	expr, err := parseExpression(atoms[1])
	if err != nil {
		return ReturnForm{}, err
	}
	returnForm := ReturnForm{
		Line:   line,
		Column: column,
		Value:  expr,
	}
	return returnForm, nil
}

func parseThrow(atoms []Atom, line int, column int) (ThrowForm, error) {
	if len(atoms) != 2 {
		return ThrowForm{}, errors.New("Throw statement has wrong number of elements: " + spew.Sdump(atoms))
	}
	expr, err := parseExpression(atoms[1])
	if err != nil {
		return ThrowForm{}, err
	}
	throwForm := ThrowForm{
		Line:   line,
		Column: column,
		Value:  expr,
	}
	return throwForm, nil
}

func parseBreak(atoms []Atom, line int, column int) (BreakForm, error) {
	breakForm := BreakForm{
		Line:   line,
		Column: column,
	}
	if len(atoms) == 2 {
		symbol, ok := atoms[1].(Symbol)
		if !ok {
			return BreakForm{}, errors.New("Break statement has invalid label: " + spew.Sdump(atoms))
		}
		breakForm.Label = symbol.Content
	} else if len(atoms) == 1 {
	} else {
		return BreakForm{}, errors.New("Break statement has wrong number of elements: " + spew.Sdump(atoms))
	}
	return breakForm, nil
}

func parseContinue(atoms []Atom, line int, column int) (ContinueForm, error) {
	continueForm := ContinueForm{
		Line:   line,
		Column: column,
	}
	if len(atoms) == 2 {
		symbol, ok := atoms[1].(Symbol)
		if !ok {
			return ContinueForm{}, errors.New("Continue statement has invalid label: " + spew.Sdump(atoms))
		}
		continueForm.Label = symbol.Content
	} else if len(atoms) == 1 {
	} else {
		return ContinueForm{}, errors.New("Continue statement has wrong number of elements: " + spew.Sdump(atoms))
	}
	return continueForm, nil
}

func parseFor(atoms []Atom, line int, column int) (ForForm, error) {
	if len(atoms) < 3 {
		return ForForm{}, errors.New("For statement has too few elements: " + spew.Sdump(atoms))
	}
	condition, err := parseExpression(atoms[1])
	if err != nil {
		return ForForm{}, err
	}
	stmts, err := parseBody(atoms[2:])
	if err != nil {
		return ForForm{}, err
	}
	forForm := ForForm{
		Line:      line,
		Column:    column,
		Condition: condition,
		Body:      stmts,
	}
	return forForm, nil
}

func parseVar(atoms []Atom, line int, column int) (VarForm, error) {
	if len(atoms) != 3 && len(atoms) != 4 {
		return VarForm{}, errors.New("Var statement has wrong number of elements: " + spew.Sdump(atoms))
	}
	symbol, ok := atoms[1].(Symbol)
	if !ok {
		return VarForm{}, errors.New("Var statement expecting symbol for name: " + spew.Sdump(atoms))
	}
	if symbol.Content == strings.Title(symbol.Content) {
		return VarForm{}, errors.New("Local variable name must start lowercase: " + spew.Sdump(atoms))
	}
	varForm := VarForm{
		Line:   line,
		Column: column,
		Target: symbol.Content,
	}
	valIdx := 2
	if len(atoms) == 4 {
		valIdx = 3
	}
	var errType error
	var errVal error
	varForm.Type, errType = parseDataType(atoms[2])
	varForm.Value, errVal = parseExpression(atoms[valIdx])
	if len(atoms) == 3 {
		if errType != nil && errVal != nil {
			return VarForm{}, msg(atoms[2].GetLine(), atoms[2].GetColumn(), "Var form expecting expression or type.")
		}
	} else {
		if errType != nil {
			return VarForm{}, errType
		}
		if errVal != nil {
			return VarForm{}, errVal
		}
	}
	return varForm, nil
}

func parseSwitch(atoms []Atom) (SwitchForm, int, error) {
	switchForm := SwitchForm{}
	// parse if clause
	switchAtoms := atoms[0].(ParenList).Atoms
	if len(switchAtoms) < 2 {
		return SwitchForm{}, 0, errors.New("Invalid switch form (expecting value): " + spew.Sdump(atoms))
	}
	var err error
	switchForm.Value, err = parseExpression(switchAtoms[1])
	if err != nil {
		return SwitchForm{}, 0, err
	}
	// parse case clauses and default clause
	n := 1
Loop:
	for _, atom := range atoms {
		parens, ok := atom.(ParenList)
		if !ok {
			break Loop
		}
		if len(parens.Atoms) == 0 {
			break Loop
		}
		elems := parens.Atoms
		symbol, ok := elems[0].(Symbol)
		if !ok {
			break Loop
		}
		switch symbol.Content {
		case "case":
			if len(elems) < 2 {
				return SwitchForm{}, 0, errors.New("Invalid elif clause (expecting condition): " + spew.Sdump(elems))
			}
			val, err := parseExpression(elems[1])
			if err != nil {
				return SwitchForm{}, 0, err
			}
			switchForm.CaseValues = append(switchForm.CaseValues, val)
			body, err := parseBody(elems[2:])
			if err != nil {
				return SwitchForm{}, 0, err
			}
			switchForm.CaseBodies = append(switchForm.CaseBodies, body)
			n++
		case "default":
			switchForm.DefaultBody, err = parseBody(elems[1:])
			if err != nil {
				return SwitchForm{}, 0, err
			}
			n++
			break Loop
		default:
			break Loop
		}
	}
	return switchForm, n, nil
}

func parseTry(atoms []Atom) (TryForm, int, error) {
	tryForm := TryForm{}
	// parse if clause
	ifAtoms := atoms[0].(ParenList).Atoms
	if len(ifAtoms) < 2 {
		return TryForm{}, 0, errors.New("Invalid try form (expecting body): " + spew.Sdump(atoms))
	}
	var err error
	tryForm.Body, err = parseBody(ifAtoms[1:])
	if err != nil {
		return TryForm{}, 0, err
	}
	// parse catch clauses and finally clause
	n := 1
Loop:
	for _, atom := range atoms {
		parens, ok := atom.(ParenList)
		if !ok {
			break Loop
		}
		if len(parens.Atoms) == 0 {
			break Loop
		}
		elems := parens.Atoms
		symbol, ok := elems[0].(Symbol)
		if !ok {
			break Loop
		}
		switch symbol.Content {
		case "catch":
			if len(elems) < 2 {
				return TryForm{}, 0, errors.New("Invalid catch clause (expecting type): " + spew.Sdump(elems))
			}
			DataType, err := parseDataType(elems[1])
			if err != nil {
				return TryForm{}, 0, err
			}
			tryForm.CatchTypes = append(tryForm.CatchTypes, DataType)
			body, err := parseBody(elems[2:])
			if err != nil {
				return TryForm{}, 0, err
			}
			tryForm.CatchBodies = append(tryForm.CatchBodies, body)
			n++
		case "finally":
			tryForm.FinallyBody, err = parseBody(elems[1:])
			if err != nil {
				return TryForm{}, 0, err
			}
			n++
			break Loop
		default:
			break Loop
		}
	}
	return tryForm, n, nil
}

func parseGlobal(parens ParenList, annotations []AnnotationForm) (GlobalDef, error) {
	globalDef := GlobalDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	atoms := parens.Atoms
	if len(atoms) < 3 {
		return GlobalDef{}, errors.New("Invalid global: " + spew.Sdump(parens))
	}
	symbol, ok := atoms[1].(Symbol)
	if !ok {
		return GlobalDef{}, errors.New("Expecting global name: " + spew.Sdump(parens))
	}
	globalDef.Name = symbol.Content
	DataType, err := parseDataType(atoms[2])
	if err != nil {
		return GlobalDef{}, err
	}
	globalDef.Type = DataType
	if len(atoms) == 4 {
		expr, err := parseExpression(atoms[3])
		if err != nil {
			return GlobalDef{}, err
		}
		globalDef.Value = expr
	} else if len(atoms) > 4 {
		return GlobalDef{}, errors.New("Too many atoms in global: " + spew.Sdump(parens))
	}
	return globalDef, nil
}

func parseInterface(parens ParenList, annotations []AnnotationForm) (InterfaceDef, error) {
	interfaceDef := InterfaceDef{
		Annotations: annotations,
		AccessLevel: PublicAccess,
	}
	elems := parens.Atoms
	if len(elems) < 2 {
		return InterfaceDef{}, errors.New("Interface must have a name: line " + itoa(parens.Line))
	}
	idx := 1
	if atomChain, ok := elems[idx].(AtomChain); ok {
		if sigil, ok := atomChain.Atoms[0].(SigilAtom); ok {
			if sigil.Content == "-" {
				if symbol, ok := atomChain.Atoms[1].(Symbol); ok {
					switch symbol.Content {
					case "priv":
						interfaceDef.AccessLevel = PrivateAccess
					case "prot":
						interfaceDef.AccessLevel = ProtectedAccess
					default:
						return InterfaceDef{}, errors.New("Invalid atom in interface: line " + itoa(parens.Line) + " column: " + itoa(atomChain.Column))
					}
				}
				idx++
			}
		}
	}
	DataType, err := parseDataType(elems[idx])
	if err != nil {
		return InterfaceDef{}, errors.New("Interface has invalid name: " + err.Error())
	}
	interfaceDef.Type = DataType
	idx++
	for _, atom := range elems[idx:] {
		parens, ok := atom.(ParenList)
		if !ok {
			return InterfaceDef{}, errors.New("Invalid atom in interface method signature. " + spew.Sdump(parens))
		}
		if symbol, ok := parens.Atoms[0].(Symbol); ok {
			switch symbol.Content {
			case "m":
				paramTypes, returnType, name, err := parseMethodSignature(parens)
				if err != nil {
					return InterfaceDef{}, err
				}
				interfaceDef.MethodNames = append(interfaceDef.MethodNames, name)
				interfaceDef.MethodParams = append(interfaceDef.MethodParams, paramTypes)
				interfaceDef.MethodReturnTypes = append(interfaceDef.MethodReturnTypes, returnType)
			case "f":
				// todo: fields
			case "p":
				// todo: properties
			}
		} else {
			return InterfaceDef{}, errors.New("Expecting symbol at start of parenlist in interface. " + spew.Sdump(parens))
		}
	}
	return interfaceDef, nil
}

func parseMethodSignature(parens ParenList) ([]DataType, DataType, string, error) {
	var name string
	var returnType DataType
	atoms := parens.Atoms
	if len(atoms) < 2 {
		return nil, DataType{}, "", errors.New("Invalid method signature: " + spew.Sdump(parens))
	}
	if symbol, ok := atoms[1].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return nil, DataType{}, "", errors.New("Invalid method name (cannot begin with uppercase): " + spew.Sdump(symbol))
		}
		name = symbol.Content
	}
	if len(atoms) < 3 {
		return nil, DataType{}, "", nil
	}
	idx := 2
	dataType, err := parseDataType(atoms[idx])
	if err == nil {
		returnType = dataType
		idx++
	}
	if len(atoms) <= idx {
		return nil, DataType{}, "", errors.New("Incomplete method definition: " + spew.Sdump(parens))
	}
	// params
	paramTypes := []DataType{}
	if sigil, ok := atoms[idx].(SigilAtom); ok {
		if sigil.Content != ":" {
			return nil, DataType{}, "", errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
		}
		idx++
		for _, atom := range atoms[idx:] {
			dataType, err := parseDataType(atom)
			if err != nil {
				return nil, DataType{}, "", errors.New("Invalid parameter type: " + spew.Sdump(atom))
			}
			paramTypes = append(paramTypes, dataType)
		}
	}
	return paramTypes, returnType, name, nil
}
