package main

import (
	"errors"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

func parse(readerData []Atom) (TopDefs, error) {
	topDefs := TopDefs{
		classes: []ClassDef{},
		structs: []StructDef{},
		funcs:   []FuncDef{},
		globals: []GlobalDef{},
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
					return TopDefs{}, nil
				}
				topDefs.classes = append(topDefs.classes, class)
				annotations = []AnnotationForm{} // reset to empty slice
			case "struct":
				structDef, err := parseStruct(atom, annotations)
				if err != nil {
					return TopDefs{}, nil
				}
				topDefs.structs = append(topDefs.structs, structDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "func":
				funcDef, err := parseFunc(atom, annotations)
				if err != nil {
					return TopDefs{}, nil
				}
				topDefs.funcs = append(topDefs.funcs, funcDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "global":
				global, err := parseGlobal(atom, annotations)
				if err != nil {
					return TopDefs{}, nil
				}
				topDefs.globals = append(topDefs.globals, global)
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
	return ClassDef{}, nil
}

// get all symbols (as strings) that are at start of slice (stopping at first non-symbol)
func getLeadingSymbolStrings(atoms []Atom) []string {
	strs := []string{}
	for _, atom := range atoms {
		if symbol, ok := atom.(Symbol); ok {
			strs = append(strs, symbol.Content)
		} else {
			break
		}
	}
	return strs
}

func parseStruct(parens ParenList, annotations []AnnotationForm) (StructDef, error) {
	structDef := StructDef{
		Annotations: annotations,
		AccessLevel: PublicAccess,
	}
	elems := parens.Atoms
	idx := 1
	if len(elems) > idx {
		return StructDef{}, errors.New("Struct must have a name: line " + itoa(parens.Line))
	}
	symbol, ok := elems[idx].(Symbol)
	if !ok {
		return StructDef{}, errors.New("Struct must have a name: line " + itoa(parens.Line))
	}
	structDef.Name = symbol.Content
	idx++
	if len(elems) == idx {
		return structDef, nil
	}
	if atomChain, ok := elems[idx].(AtomChain); ok {
		if len(atomChain.Atoms) != 2 {
			return StructDef{}, errors.New("Invalid atom in struct: line " + itoa(parens.Line) + " column: " + itoa(atomChain.Column))
		}
		if sigil, ok := atomChain.Atoms[0].(SigilAtom); ok {
			if sigil.Content != "-" {
				return StructDef{}, errors.New("Invalid atom in struct: line " + itoa(parens.Line) + " column: " + itoa(atomChain.Column))
			}
		}
		if symbol, ok := atomChain.Atoms[1].(Symbol); ok {
			switch symbol.Content {
			case "priv":
				structDef.AccessLevel = PrivateAccess
			case "prot":
				structDef.AccessLevel = ProtectedAccess
			default:
				return StructDef{}, errors.New("Invalid atom in struct: line " + itoa(parens.Line) + " column: " + itoa(atomChain.Column))
			}
		}
		idx++
	}
	if sigil, ok := elems[idx].(SigilAtom); ok {
		if sigil.Content == ":" {
			idx++
			strs := getLeadingSymbolStrings(elems[idx:])
			if len(strs) == 0 {
				return StructDef{}, errors.New("Struct must have at least one interface listed after ':' sigil: line " + itoa(parens.Line))
			}
			idx += len(strs)
			structDef.Interfaces = strs
		}
	}
	annotations = []AnnotationForm{}
	for _, atom := range elems[idx:] {
		switch atom := atom.(type) {
		case ParenList:
			atoms := atom.Atoms
			if len(atoms) == 0 {
				return StructDef{}, errors.New("Invalid struct member: " + spew.Sdump(atom))
			}
			if sigil, ok := atoms[0].(SigilAtom); ok {
				if sigil.Content != "@" {
					return StructDef{}, errors.New("Invalid struct member: " + spew.Sdump(atom))
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
				return StructDef{}, errors.New("Invalid struct member: " + spew.Sdump(atom))
			}
			switch first.Content {
			case "f":
				field, err := parseField(atom, annotations)
				if err != nil {
					return StructDef{}, nil
				}
				structDef.Fields = append(structDef.Fields, field)
				annotations = []AnnotationForm{} // reset to empty slice
			case "m":
				methodDef, err := parseMethod(atom, annotations)
				if err != nil {
					return StructDef{}, nil
				}
				structDef.Methods = append(structDef.Methods, methodDef)
				annotations = []AnnotationForm{} // reset to empty slice
			case "p":
				property, err := parseProperty(atom, annotations)
				if err != nil {
					return StructDef{}, nil
				}
				structDef.Properties = append(structDef.Properties, property)
				annotations = []AnnotationForm{} // reset to empty slice
			case "constructor":
				constructor, err := parseConstructor(atom, annotations)
				if err != nil {
					return StructDef{}, nil
				}
				structDef.Constructors = append(structDef.Constructors, constructor)
				annotations = []AnnotationForm{} // reset to empty slice
			default:
				return StructDef{}, errors.New("Invalid struct member: " + spew.Sdump(atom))
			}
		default:
			return StructDef{}, errors.New("Invalid struct member: " + spew.Sdump(atom))
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
	typeSpec, err := parseTypeSpec(atoms[2])
	if err != nil {
		return FieldDef{}, err
	}
	field.Type = typeSpec
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

func parseTypeSpec(atom Atom) (TypeSpec, error) {
	typeSpec := TypeSpec{}
	switch atom := atom.(type) {
	case Symbol:
		if atom.Content != strings.Title(atom.Content) {
			return TypeSpec{}, errors.New("Type name must begin with capital letter")
		}
		typeSpec.Name = atom.Content
		typeSpec.Line = atom.Line
		typeSpec.Column = atom.Column
	case AtomChain:
		atoms := atom.Atoms
		if len(atoms) < 1 {
			return TypeSpec{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
		}
		if symbol, ok := atoms[0].(Symbol); ok {
			if symbol.Content != strings.Title(symbol.Content) {
				return TypeSpec{}, errors.New("Type name must begin with capital letter")
			}
			typeSpec.Name = symbol.Content
		}
		if len(atoms) < 2 {
			return TypeSpec{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
		}
		switch second := atoms[1].(type) {
		case AngleList:
			angleAtoms := second.Atoms
			if len(angleAtoms) == 0 {
				return TypeSpec{}, errors.New("Invalid type spec (empty angle brackets): " + spew.Sdump(atom))
			}
			typeSpec.TypeParams = []TypeSpec{}
			for _, typeAtom := range angleAtoms {
				ts, err := parseTypeSpec(typeAtom)
				if err != nil {
					return TypeSpec{}, err
				}
				typeSpec.TypeParams = append(typeSpec.TypeParams, ts)
			}
			if len(atoms) < 3 {
				break
			}
			namespace, err := parseNamespace(atoms[2:], atom.Line, atom.Column)
			if err != nil {
				return TypeSpec{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
			}
			typeSpec.Namespace = namespace
		case SigilAtom:
			namespace, err := parseNamespace(atoms[1:], atom.Line, atom.Column)
			if err != nil {
				return TypeSpec{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
			}
			typeSpec.Namespace = namespace
		default:
		}
	default:
		return TypeSpec{}, errors.New("Invalid type spec: " + spew.Sdump(atom))
	}
	return typeSpec, nil
}

// expects / sigil followed by one or more symbols separated by dots
func parseNamespace(atoms []Atom, line int, column int) ([]string, error) {
	if len(atoms) < 2 {
		return nil, errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}
	if sigil, ok := atoms[0].(SigilAtom); ok {
		if sigil.Content != "/" {
			return nil, errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
		}
	} else {
		return nil, errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
	}
	strs := []string{}
	for i, atom := range atoms[1:] {
		if i%2 == 0 {
			if sigil, ok := atom.(Symbol); ok {
				strs = append(strs, sigil.Content)
			} else {
				return nil, errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
			}
		} else {
			if sigil, ok := atom.(SigilAtom); ok {
				if sigil.Content != "." {
					return nil, errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
				}
			} else {
				return nil, errors.New("Improperly formed namespace qualifier: line " + itoa(line) + " column " + itoa(column))
			}
		}
	}
	return strs, nil
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
			Value:  atom.Content,
			Line:   atom.Line,
			Column: atom.Column,
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
		numStr := ""
		if sigil, ok := elems[0].(SigilAtom); ok {
			if sigil.Content == "-" {
				numStr += "-"
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
			numStr += num.Content
			idx++
		} else {
			return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
		}
		if len(elems) >= idx {
			// optional dot followed by number
			if len(elems) != idx+1 {
				return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
			}
			if sigil, ok := elems[idx].(SigilAtom); ok {
				if sigil.Content != "." {
					return nil, errors.New("Invalid expression (unexpected sigil): " + spew.Sdump(atom))
				}
			} else {
				return nil, errors.New("Invalid expression (expected .): " + spew.Sdump(elems[idx]))
			}
			if num, ok := elems[idx+1].(NumberAtom); ok {
				numStr += num.Content
			} else {
				return nil, errors.New("Invalid expression (expected number): " + spew.Sdump(elems[idx]))
			}
		}
		return ParsedNumberAtom{
			Value:  numStr,
			Line:   atom.Line,
			Column: atom.Column,
		}, nil
	case ParenList:
		atoms := atom.Atoms
		if len(atoms) == 0 {
			return nil, errors.New("Invalid expression (empty parens): " + spew.Sdump(atom))
		}
		varExpr, err := parseVarExpression(atoms[0])
		if err != nil {
			return nil, errors.New("Invalid expression (expecting name): " + spew.Sdump(atom))
		}
		args := make([]Expression, len(atoms)-1)
		for i, a := range atoms[1:] {
			expr, err := parseExpression(a)
			if err != nil {
				return nil, err
			}
			args[i] = expr
		}
		expr = CallForm{
			Line:      atom.Line,
			Column:    atom.Column,
			Name:      varExpr.Name,
			Class:     varExpr.Class,
			Namespace: varExpr.Namespace,
			Args:      args,
		}
	default:
		return nil, errors.New("Invalid expression: " + spew.Sdump(atom))
	}
	return expr, nil
}

func parseMethod(parens ParenList, annotations []AnnotationForm) (MethodDef, error) {
	return MethodDef{}, nil
}

func parseProperty(parens ParenList, annotations []AnnotationForm) (PropertyDef, error) {
	return PropertyDef{}, nil
}

func parseConstructor(parens ParenList, annotations []AnnotationForm) (ConstructorDef, error) {
	return ConstructorDef{}, nil
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
		funcDef.Name = symbol.Content
	}
	if len(atoms) < 3 {
		return funcDef, nil
	}
	idx := 2
	typeSpec, err := parseTypeSpec(atoms[idx])
	if err == nil {
		funcDef.ReturnType = typeSpec
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
		params := []ParamDef{}
		for len(atoms) < idx+2 {
			symbol, ok := atoms[idx].(Symbol)
			if !ok {
				break
			}
			typeSpec, err := parseTypeSpec(atoms[idx+1])
			if err != nil {
				return FuncDef{}, errors.New("Invalid parameter type: " + spew.Sdump(atoms[idx+1]))
			}
			params = append(params, ParamDef{symbol.Content, typeSpec})
			idx += 2
		}
		funcDef.Params = params
	}
	stmts, err := parseBody(atoms[idx:], funcDef.ReturnType)
	if err != nil {
		return FuncDef{}, err
	}
	funcDef.Body = stmts
	return funcDef, nil
}

func parseBody(atoms []Atom, returnType TypeSpec) ([]Statement, error) {
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
			var n int
			var err error
			switch symbol.Content {
			case "if":
				stmt, n, err = parseIf(atoms[i:], returnType)
			case "for":
			case "":
			}
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, stmt)
			if n > 0 {
				i += n
				continue
			}
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

func parseIf(atoms []Atom, returnType TypeSpec) (IfForm, int, error) {
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
	// TODO check that condition is boolean expression
	ifForm.Body, err = parseBody(ifAtoms[2:], returnType)
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
			// TODO check that condition is boolean expression
			ifForm.ElifConds = append(ifForm.ElifConds, condition)
			body, err := parseBody(elems[2:], returnType)
			if err != nil {
				return IfForm{}, 0, err
			}
			ifForm.ElifBodies = append(ifForm.ElifBodies, body)
			n++
		case "else":
			ifForm.ElseBody, err = parseBody(elems[1:], returnType)
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

func parseAssignment(atoms []Atom) (AssignmentForm, error) {
	return AssignmentForm{}, nil
}

func parseGlobal(parens ParenList, annotations []AnnotationForm) (GlobalDef, error) {
	return GlobalDef{}, nil
}
