package main

import (
	"errors"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

func parse(readerData []Atom, topDefs *TopDefs, isMain bool) error {
	annotations := []AnnotationForm{}

	for _, atom := range readerData {
		switch atom := atom.(type) {
		case ParenList:
			elems := atom.Atoms
			if len(elems) == 0 {
				return errors.New("Invalid top-level atom: " + spew.Sdump(atom))
			}
			if sigil, ok := elems[0].(SigilAtom); ok {
				if sigil.Content != "@" {
					return errors.New("Invalid top-level atom: " + spew.Sdump(atom))
				}
				annotation, err := parseAnnotation(atom)
				if err != nil {
					return err
				}
				annotations = append(annotations, annotation)
				continue
			}
			first, ok := elems[0].(Symbol)
			if !ok {
				return errors.New("Invalid top-level atom: " + spew.Sdump(atom))
			}
			switch first.Content {
			case "class":
				class, err := parseClass(atom, annotations)
				if err != nil {
					return err
				}
				topDefs.Classes = append(topDefs.Classes, class)
				annotations = []AnnotationForm{} // reset to empty slice
			case "struct":
				structDef, err := parseStruct(atom, annotations, false)
				if err != nil {
					return err
				}
				topDefs.Structs = append(topDefs.Structs, structDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "func":
				funcDef, err := parseFunc(atom, annotations)
				if err != nil {
					return err
				}
				if !isMain && funcDef.Name == "main" {
					return msg(funcDef.Line, funcDef.Column, "A 'main' function can only be declared in the main file of the namespace.")
				}
				topDefs.Funcs = append(topDefs.Funcs, funcDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "interface":
				interfaceDef, err := parseInterface(atom, annotations)
				if err != nil {
					return err
				}
				topDefs.Interfaces = append(topDefs.Interfaces, interfaceDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "global":
				global, err := parseGlobal(atom, annotations)
				if err != nil {
					return err
				}
				topDefs.Globals = append(topDefs.Globals, global)
				annotations = []AnnotationForm{} // reset to empty slice
			case "import":
				if !isMain {
					return msg(first.Line, first.Column, "Imports should only go in the main source file of the namespace.")
				}
				importDef, err := parseImportDef(atom, annotations)
				if err != nil {
					return err
				}
				topDefs.Imports = append(topDefs.Imports, importDef)
				annotations = []AnnotationForm{} // reset to empty slice
			default:
				return errors.New("Invalid top-level atom: " + spew.Sdump(atom))
			}
		default:
			return errors.New("Invalid top-level atom: " + spew.Sdump(atom))
		}
	}
	return nil
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
		expr.Name = ShortName(atom.Content)
	case AtomChain:
		atoms := atom.Atoms
		if len(atoms) == 0 {
			return VarExpression{}, errors.New("Invalid name: " + spew.Sdump(atom))
		}
		if symbol, ok := atoms[0].(Symbol); ok {
			if symbol.Content == strings.Title(symbol.Content) {
				return VarExpression{}, errors.New("Invalid name (cannot begin with uppercase): " + spew.Sdump(atom))
			}
			expr.Name = ShortName(symbol.Content)
			namespace, err := parseNamespace(atoms[1:], symbol.Line, symbol.Column)
			if err != nil {
				return VarExpression{}, errors.New("Invalid name: " + spew.Sdump(atom))
			}
			expr.Namespace = NSNameShort(namespace)
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
	dataType, err := parseTypeAtom(elems[idx])
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
				dt, err := parseTypeAtom(elems[idx])
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
	idx := 1
	if idx >= len(atoms) {
		return FieldDef{}, errors.New("Expecting field name: " + spew.Sdump(parens))
	}
	if parseFlag(atoms[idx], "static") {
		field.IsStatic = true
		idx++
		if idx >= len(atoms) {
			return FieldDef{}, errors.New("Expecting field name: " + spew.Sdump(parens))
		}
	}
	symbol, ok := atoms[idx].(Symbol)
	if !ok {
		return FieldDef{}, errors.New("Expecting field name: " + spew.Sdump(parens))
	}
	field.Name = ShortName(symbol.Content)
	idx++
	if idx >= len(atoms) {
		return FieldDef{}, errors.New("Expecting field type: " + spew.Sdump(parens))
	}
	dataType, err := parseTypeAtom(atoms[idx])
	if err != nil {
		return FieldDef{}, err
	}
	field.Type = dataType
	idx++
	if idx < len(atoms) {
		expr, err := parseExpression(atoms[idx])
		if err != nil {
			return FieldDef{}, err
		}
		field.Value = expr
		idx++
		if idx < len(atoms) {
			return FieldDef{}, errors.New("Too many atoms in field: " + spew.Sdump(parens))
		}
	}
	return field, nil
}

func parseTypeAtom(atom Atom) (TypeAtom, error) {
	dataType := TypeAtom{}
	switch atom := atom.(type) {
	case Symbol:
		if atom.Content != strings.Title(atom.Content) {
			return TypeAtom{}, errors.New("Type name must begin with capital letter")
		}
		dataType.Name = ShortName(atom.Content)
		dataType.Line = atom.Line
		dataType.Column = atom.Column
	case AtomChain:
		atoms := atom.Atoms
		if len(atoms) < 1 {
			return TypeAtom{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
		}
		if symbol, ok := atoms[0].(Symbol); ok {
			if symbol.Content != strings.Title(symbol.Content) {
				return TypeAtom{}, errors.New("Type name must begin with capital letter")
			}
			dataType.Name = ShortName(symbol.Content)
		}
		if len(atoms) < 2 {
			return TypeAtom{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
		}
		switch second := atoms[1].(type) {
		case AngleList:
			angleAtoms := second.Atoms
			if len(angleAtoms) == 0 {
				return TypeAtom{}, errors.New("Invalid type spec (empty angle brackets): " + spew.Sdump(atom))
			}
			dataType.Params = []TypeAtom{}
			for _, typeAtom := range angleAtoms {
				ts, err := parseTypeAtom(typeAtom)
				if err != nil {
					return TypeAtom{}, err
				}
				dataType.Params = append(dataType.Params, ts)
			}
			if len(atoms) < 3 {
				break
			}
			namespace, err := parseNamespace(atoms[2:], atom.Line, atom.Column)
			if err != nil {
				return TypeAtom{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
			}
			dataType.Namespace = NSNameShort(namespace)
		case SigilAtom:
			namespace, err := parseNamespace(atoms[1:], atom.Line, atom.Column)
			if err != nil {
				return TypeAtom{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
			}
			dataType.Namespace = NSNameShort(namespace)
		default:
			return TypeAtom{}, errors.New("Improperly formed data type name: " + spew.Sdump(atom))
		}
	default:
		return TypeAtom{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
	}
	// translate AA and AAA into A<A<>> and A<A<A<>>>
	if dataType.Name == "AA" && dataType.Namespace == "" {
		dataType.Name = "A"
		dataType = TypeAtom{
			Name:   "A",
			Line:   dataType.Line,
			Column: dataType.Column,
			Params: []TypeAtom{dataType},
		}
	}
	if dataType.Name == "AAA" && dataType.Namespace == "" {
		dataType.Name = "A"
		dataType = TypeAtom{
			Name:   "A",
			Line:   dataType.Line,
			Column: dataType.Column,
			Params: []TypeAtom{dataType},
		}
		dataType = TypeAtom{
			Name:   "A",
			Line:   dataType.Line,
			Column: dataType.Column,
			Params: []TypeAtom{dataType},
		}
	}
	return dataType, nil
}

func parseImportDef(parens ParenList, annotations []AnnotationForm) (ImportDef, error) {
	atoms := parens.Atoms
	if len(atoms) < 2 {
		return ImportDef{}, errors.New("Invalid import form. Too few atoms. " + spew.Sdump(atoms))
	}
	symbol, ok := atoms[1].(Symbol)
	if !ok {
		return ImportDef{}, errors.New("Invalid import form. Expecting symbol. " + spew.Sdump(atoms))
	}
	if symbol.Content == strings.Title(symbol.Content) {
		return ImportDef{}, errors.New("Invalid import form: imported namespace cannot start with uppercase letter. " + spew.Sdump(atoms))
	}
	exclusions := []string{}
	aliases := map[string]string{}
	shortname := ""
	if len(atoms) > 2 {
		idx := 2
		if chain, ok := atoms[idx].(AtomChain); ok {
			if len(chain.Atoms) != 2 {
				return ImportDef{}, msg(parens.Line, parens.Column, "Unexpected atom chain in import form.")
			}
			if sigil, ok := chain.Atoms[0].(SigilAtom); ok {
				if sigil.Content != "-" {
					return ImportDef{}, msg(parens.Line, parens.Column, "Expecting - in atom chain of import form..")
				}
			} else {
				return ImportDef{}, msg(parens.Line, parens.Column, "Expecting - in atom chain of import form..")
			}
			if symbol, ok := chain.Atoms[1].(Symbol); ok {
				if symbol.Content != "shortname" {
					return ImportDef{}, msg(parens.Line, parens.Column, "Expecting 'shortname' symbol in atom chain of import form..")
				}
			} else {
				return ImportDef{}, msg(parens.Line, parens.Column, "Expecting 'shortname' symbol in atom chain of import form..")
			}
			idx++
			if symbol, ok := atoms[idx].(Symbol); ok {
				shortname = symbol.Content
				if shortname == strings.Title(shortname) {
					return ImportDef{}, msg(parens.Line, parens.Column, "Import shortname cannot start with uppercase letter.")
				}
			} else {
				return ImportDef{}, msg(parens.Line, parens.Column, "Expecting symbol for import shortname.")
			}
			idx++
		}

		for _, atom := range atoms[idx:] {
			if parens, ok := atom.(ParenList); ok {
				if len(parens.Atoms) < 2 {
					return ImportDef{}, msg(parens.Line, parens.Column, "Parens in import form contains too few atoms.")
				}
				if symbol, ok := parens.Atoms[0].(Symbol); ok {
					switch symbol.Content {
					case "exclude":
						if len(parens.Atoms) != 2 {
							return ImportDef{}, msg(parens.Line, parens.Column, "Exclude form in import expecting two atoms.")
						}
						if symbol, ok := parens.Atoms[1].(Symbol); ok {
							exclusions = append(exclusions, symbol.Content)
						} else {
							return ImportDef{}, msg(parens.Line, parens.Column, "Exclude form in import expecting symbol to exclude.")
						}
					case "alias":
						if len(parens.Atoms) != 3 {
							return ImportDef{}, msg(parens.Line, parens.Column, "Alias form in import expecting three atoms.")
						}
						var original string
						var substitute string
						if symbol, ok := parens.Atoms[1].(Symbol); ok {
							original = symbol.Content
						} else {
							return ImportDef{}, msg(parens.Line, parens.Column, "Alias form in import expecting symbol to alias.")
						}
						if symbol, ok := parens.Atoms[2].(Symbol); ok {
							substitute = symbol.Content
						} else {
							return ImportDef{}, msg(parens.Line, parens.Column, "Alias form in import expecting symbol for alias.")
						}
						// alias starting cases should match
						if (original == strings.Title(original)) !=
							(substitute == strings.Title(substitute)) {
							return ImportDef{}, msg(parens.Line, parens.Column, "Alias form in import expecting symbols with same starting letter case.")
						}
						aliases[original] = substitute
					default:
						return ImportDef{}, msg(parens.Line, parens.Column, "Parens in import form starts with unexpected symbol.")
					}
				} else {
					return ImportDef{}, msg(parens.Line, parens.Column, "Parens in import form should start with a symbol.")
				}
			} else {
				return ImportDef{}, errors.New("Invalid atom in import form: expecting parens. " + spew.Sdump(atoms))
			}
		}
	}
	return ImportDef{
		Line:        symbol.Line,
		Column:      symbol.Column,
		Namespace:   NSNameFull(symbol.Content),
		Shortname:   NSNameShort(shortname),
		Exclusions:  exclusions,
		Aliases:     aliases,
		Annotations: annotations,
	}, nil
}

// expects / sigil followed by one or more symbols separated by dots
func parseNamespace(atoms []Atom, line int, column int) (string, error) {
	if len(atoms) != 2 {
		return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}
	if sigil, ok := atoms[0].(SigilAtom); ok {
		if sigil.Content != "/" {
			return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
		}
	} else {
		return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}

	if symbol, ok := atoms[1].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return "", errors.New("Improperly formed namespace qualifier (namspace cannot begin with uppercase): line " + itoa(line) + " column " + itoa(column))
		}
		return symbol.Content, nil
	} else {
		return "", errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}
}

func parseExpression(atom Atom) (Expression, error) {
	varExpr, err := parseVarExpression(atom)
	if err == nil { // if no error, then VarExpression
		return varExpr, nil
	}
	typeExpr, err := parseTypeAtom(atom)
	if err == nil { // if no error, then TypeAtom
		return typeExpr, nil
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
		idx := 1
		var staticType TypeAtom
		if idx < len(atoms) {
			var err error
			staticType, err = parseTypeAtom(atoms[idx])
			if err == nil {
				idx++
			}
		}
		sizeFlag := false
		if idx < len(atoms) {
			if parseFlag(atoms[idx], "size") {
				sizeFlag = true
				idx++
			}
		}
		args := make([]Expression, len(atoms)-idx)
		for i, a := range atoms[idx:] {
			expr, err := parseExpression(a)
			if err != nil {
				return nil, err
			}
			args[i] = expr
		}
		varExpr, err := parseVarExpression(atoms[0])
		if err != nil {
			dt, err := parseTypeAtom(atoms[0])
			if err != nil {
				return nil, errors.New("Invalid expression (expecting name or type): " + spew.Sdump(atom))
			}
			expr = TypeCallForm{
				Line:     atom.Line,
				Column:   atom.Column,
				Type:     dt,
				SizeFlag: sizeFlag,
				Args:     args,
			}
		} else {
			expr = CallForm{
				Line:      atom.Line,
				Column:    atom.Column,
				Name:      varExpr.Name,
				Namespace: varExpr.Namespace,
				Static:    staticType,
				Args:      args,
			}
		}
	default:
		return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
	}
	return expr, nil
}

func parseFlag(atom Atom, expected string) bool {
	if chain, ok := atom.(AtomChain); ok {
		if len(chain.Atoms) == 2 {
			if sigil, ok := chain.Atoms[0].(SigilAtom); ok {
				if sigil.Content == "-" {
					if symbol, ok := chain.Atoms[1].(Symbol); ok {
						return symbol.Content == expected
					}
				}
			}
		}
	}
	return false
}

func parseMethod(parens ParenList, annotations []AnnotationForm) (MethodDef, error) {
	methodDef := MethodDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
	}
	atoms := parens.Atoms
	idx := 1
	if idx >= len(atoms) {
		return MethodDef{}, errors.New("Invalid method definition: " + spew.Sdump(parens))
	}
	if parseFlag(atoms[idx], "static") {
		idx++
		methodDef.IsStatic = true
	}
	if symbol, ok := atoms[idx].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return MethodDef{}, errors.New("Invalid method name (cannot begin with uppercase): " + spew.Sdump(symbol))
		}
		methodDef.Name = ShortName(symbol.Content)
	}
	idx++
	if idx >= len(atoms) {
		return MethodDef{}, errors.New("Incomplete method definition: " + spew.Sdump(parens))
	}
	dt, err := parseTypeAtom(atoms[idx])
	if err == nil {
		methodDef.Return = dt
		idx++
	}
	if idx >= len(atoms) {
		return MethodDef{}, errors.New("Incomplete method definition: " + spew.Sdump(parens))
	}
	// params
	if sigil, ok := atoms[idx].(SigilAtom); ok {
		if sigil.Content != ":" {
			return MethodDef{}, errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
		}
		idx++
		paramNames := []ShortName{}
		paramTypes := []TypeAtom{}
		for idx+1 < len(atoms) {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			dt, err := parseTypeAtom(atoms[idx+1])
			if err != nil {
				return MethodDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			paramNames = append(paramNames, ShortName(symbol.Content))
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
	if len(atoms) < 3 {
		return PropertyDef{}, msg(parens.Line, parens.Column, "Too few atoms in property form.")
	}
	propertyDef := PropertyDef{
		Line:        parens.Line,
		Column:      parens.Column,
		Annotations: annotations,
		IsManual:    false,
	}
	idx := 1
	if parseFlag(atoms[idx], "manual") {
		propertyDef.IsManual = true
		idx++
	}
	if symbol, ok := atoms[idx].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return PropertyDef{}, errors.New("Invalid property name (cannot begin with uppercase): " + spew.Sdump(symbol))
		}
		propertyDef.Name = ShortName(symbol.Content)
	} else {
		return PropertyDef{}, errors.New("Expecting symbol name for property: " + spew.Sdump(atoms[idx]))
	}
	idx++
	var err error
	propertyDef.Type, err = parseTypeAtom(atoms[idx])
	if err != nil {
		return PropertyDef{}, errors.New("Expecting type for property: " + spew.Sdump(atoms[idx]))
	}
	idx++
	setFlagIdx := len(atoms)
	for i, atom := range atoms[idx:] {
		if parseFlag(atom, "set") {
			setFlagIdx = i + idx
			break
		}
	}
	propertyDef.GetBody, err = parseBody(atoms[idx:setFlagIdx])
	if err != nil {
		return PropertyDef{}, err
	}
	if setFlagIdx != len(atoms) {
		propertyDef.SetBody, err = parseBody(atoms[setFlagIdx+1:])
		if err != nil {
			return PropertyDef{}, err
		}
	}
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
		paramNames := []ShortName{}
		paramTypes := []TypeAtom{}
		for idx+1 < len(atoms) {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			dt, err := parseTypeAtom(atoms[idx+1])
			if err != nil {
				return ConstructorDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			paramNames = append(paramNames, ShortName(symbol.Content))
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
	idx := 1
	if idx >= len(atoms) {
		return FuncDef{}, errors.New("Invalid function definition: " + spew.Sdump(parens))
	}
	if symbol, ok := atoms[idx].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			return FuncDef{}, errors.New("Invalid func name (cannot begin with uppercase): " + spew.Sdump(parens))
		}
		funcDef.Name = ShortName(symbol.Content)
	} else {
		return FuncDef{}, errors.New("Invalid func name (cannot begin with uppercase): " + spew.Sdump(parens))
	}
	idx++
	if idx >= len(atoms) {
		return funcDef, nil
	}
	dt, err := parseTypeAtom(atoms[idx])
	if err == nil {
		funcDef.Return = dt
		idx++
	}
	if idx >= len(atoms) {
		return FuncDef{}, errors.New("Incomplete function definition: " + spew.Sdump(parens))
	}
	// params
	if sigil, ok := atoms[idx].(SigilAtom); ok {
		if sigil.Content != ":" {
			return FuncDef{}, errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
		}
		idx++
		paramNames := []ShortName{}
		paramTypes := []TypeAtom{}
		for idx+1 < len(atoms) {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			dt, err := parseTypeAtom(atoms[idx+1])
			if err != nil {
				return FuncDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			paramNames = append(paramNames, ShortName(symbol.Content))
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
		Target: ShortName(symbol.Content),
	}
	valIdx := 2
	if len(atoms) == 4 {
		valIdx = 3
	}
	var errType error
	var errVal error
	varForm.Type, errType = parseTypeAtom(atoms[2])
	varForm.Value, errVal = parseExpression(atoms[valIdx])
	if len(atoms) == 3 {
		if errType != nil && errVal != nil {
			return VarForm{}, msg(atoms[2].GetLine(), atoms[2].GetColumn(), "Var form expecting expression or type.")
		}
		if errType == nil && errVal == nil {
			varForm.Value = nil
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
			TypeAtom, err := parseTypeAtom(elems[1])
			if err != nil {
				return TryForm{}, 0, err
			}
			tryForm.CatchTypes = append(tryForm.CatchTypes, TypeAtom)
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
	idx := 1
	if idx >= len(atoms) {
		return GlobalDef{}, errors.New("Invalid global: " + spew.Sdump(parens))
	}
	symbol, ok := atoms[idx].(Symbol)
	if !ok {
		return GlobalDef{}, errors.New("Expecting global name: " + spew.Sdump(parens))
	}
	globalDef.Name = ShortName(symbol.Content)
	idx++
	if idx >= len(atoms) {
		return GlobalDef{}, errors.New("Invalid global: " + spew.Sdump(parens))
	}
	dataType, err := parseTypeAtom(atoms[idx])
	if err != nil {
		return GlobalDef{}, err
	}
	globalDef.Type = dataType
	idx++
	if idx < len(atoms) {
		expr, err := parseExpression(atoms[idx])
		if err != nil {
			return GlobalDef{}, err
		}
		globalDef.Value = expr
		idx++
	}
	if idx < len(atoms) {
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
	dataType, err := parseTypeAtom(elems[idx])
	if err != nil {
		return InterfaceDef{}, errors.New("Interface has invalid name: " + err.Error())
	}
	interfaceDef.Type = dataType
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
				interfaceDef.MethodNames = append(interfaceDef.MethodNames, ShortName(name))
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

func parseMethodSignature(parens ParenList) (paramTypes []TypeAtom, returnType TypeAtom, name string, err error) {
	paramTypes = []TypeAtom{}
	atoms := parens.Atoms
	if len(atoms) < 2 {
		return nil, TypeAtom{}, "", errors.New("Invalid method signature: " + spew.Sdump(parens))
	}
	if symbol, ok := atoms[1].(Symbol); ok {
		if symbol.Content == strings.Title(symbol.Content) {
			err = errors.New("Invalid method name (cannot begin with uppercase): " + spew.Sdump(symbol))
			return
		}
		name = symbol.Content
	}
	if len(atoms) < 3 {
		return
	}
	idx := 2
	dataType, err := parseTypeAtom(atoms[idx])
	if err == nil {
		returnType = dataType
		idx++
	} else {
		err = nil
	}

	if idx < len(atoms) {
		if sigil, ok := atoms[idx].(SigilAtom); ok {
			if sigil.Content != ":" {
				err = errors.New("Invalid sigil (expecting colon): " + spew.Sdump(parens))
				return
			}
			idx++
			for _, atom := range atoms[idx:] {
				dataType, err := parseTypeAtom(atom)
				if err != nil {
					return nil, TypeAtom{}, "", errors.New("Invalid parameter type: " + spew.Sdump(atom))
				}
				paramTypes = append(paramTypes, dataType)
			}
		} else {
			err = errors.New("Invalid method signature (expecting colon): " + spew.Sdump(parens))
			return
		}
	}
	return
}
