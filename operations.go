package main

func compileOperation(op CallForm, ns *Namespace, expectedType Type,
	locals map[ShortName]Type) (string, Type, error) {
	if op.Namespace != "" {
		return "", nil, msg(op.Line, op.Column, "Call to unknown method or function.")
	}
	returnType := expectedType
	expectedArgType := expectedType
	multiOperand := true
	switch op.Name {
	case "add", "sub", "mul", "div":
		if expectedType == nil {
			returnType = LongType
			expectedType = LongType
		}
		if !IsNumber(expectedType) {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation used where non-number expected")
		}
	case "lt", "lte", "gt", "gte":
		if expectedType == nil {
			returnType = BoolType
			expectedType = BoolType
		}
		if !IsNumber(expectedType) {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation used where non-number expected")
		}
		expectedArgType = DoubleType // todo: actually need type which is supertype of all numbers (Long is not subtype of Double)
	case "inc", "dec":
		if expectedType == nil {
			returnType = LongType
			expectedType = LongType
		}
		multiOperand = false
		if !IsInteger(expectedType) {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation used where non-number expected")
		}
	case "mod", "band", "bor", "bxor":
		if expectedType == nil {
			returnType = LongType
			expectedType = LongType
		}
		if !IsInteger(expectedType) {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation used where non-number expected")
		}
	case "bnot":
		if expectedType == nil {
			returnType = LongType
			expectedType = LongType
		}
		if !IsInteger(expectedType) {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation used where non-number expected")
		}
		multiOperand = false
	case "eq", "neq":
		expectedArgType = nil
		returnType = BoolType
	case "not":
		multiOperand = false
		expectedArgType = BoolType
	case "or", "and":
		expectedArgType = BoolType
	case "cat":
		expectedArgType = StrType
	default:
		return "", nil, msg(op.Line, op.Column, "Unknown operator, function, or method.")
	}
	if multiOperand {
		if len(op.Args) < 2 {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation requires at least two operands")
		}
	} else {
		if len(op.Args) != 1 {
			return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation requires one operand")
		}
	}
	operandCode := make([]string, len(op.Args))
	operandTypes := make([]Type, len(op.Args))
	for i, expr := range op.Args {
		var err error
		operandCode[i], operandTypes[i], err = compileExpression(expr, ns, expectedArgType, locals)
		if err != nil {
			return "", nil, err
		}
	}
	code := "("
	switch op.Name {
	case "add", "sub", "mul", "div", "mod", "and", "or", "band", "bor", "bxor", "cat":
		operatorSymbol := OperatorSymbols[op.Name]
		for i := range op.Args {
			code += operandCode[i]
			if i < len(op.Args)-1 {
				code += operatorSymbol
			}
		}
	case "inc":
		code += operandCode[0] + " + 1"
	case "dec":
		code += operandCode[0] + " - 1"
	case "eq", "neq":
		operatorSymbol := OperatorSymbols[op.Name]
		for i := 0; i < len(op.Args)-1; i++ {
			if operandTypes[i+1] != operandTypes[0] {
				return "", nil, msg(op.Line, op.Column, "'"+string(op.Name)+"' operation has mismatched operand types")
			}
			if i > 0 {
				code += " && "
			}
			code += operandCode[i] + operatorSymbol + operandCode[i+1]
		}
	case "not":
		code += "!" + operandCode[0]
	case "lt", "gt", "gte", "lte":
		operatorSymbol := OperatorSymbols[op.Name]
		for i := 0; i < len(op.Args)-1; i++ {
			if i > 0 {
				code += " && "
			}
			// todo: should use variables to store the compiled expressions because otherwise we're potentially repeating complex expressions
			code += operandCode[i] + operatorSymbol + operandCode[i+1]
		}
	case "bnot":
		code += "^" + operandCode[1]
	}
	code += ")"
	return code, returnType, nil

}

func compileCallForm(op CallForm, ns *Namespace, expectedType Type,
	locals map[ShortName]Type) (string, Type, error) {
	sigs := append(ns.GetFuncs(op.Name, op.Namespace), ns.GetMethods(op.Name, op.Namespace)...)
	if len(sigs) == 0 {
		return compileOperation(op, ns, expectedType, locals)
	}

	argCode := make([]string, len(op.Args))
	argTypes := make([]Type, len(op.Args))
	for i, expr := range op.Args {
		var err error
		argCode[i], argTypes[i], err = compileExpression(expr, ns, nil, locals)
		if err != nil {
			return "", nil, err
		}
	}
	code := ""

	// find sigs which match args
	var returnType Type

	matching := []*CallableInfo{}
Loop:
	for _, sig := range sigs {
		if len(argTypes) != len(sig.ParamTypes) {
			continue
		}
		t := ns.GetType(op.Static)
		if t != sig.Static {
			continue
		}
		for j, paramType := range sig.ParamTypes {
			if !IsSubType(argTypes[j], paramType) {
				continue Loop
			}
		}
		matching = append(matching, sig)
	}

	if len(matching) == 0 {
		return compileOperation(op, ns, expectedType, locals)
	}

	sig := matching[0]
	if len(matching) > 1 {
		var err error
		sig, err = ClosestMatchingSignature(matching, ns, op.Line, op.Column)
		if err != nil {
			return "", nil, err
		}
	}

	isMethod := sig.IsMethod
	if isMethod {
		code += argCode[0] + "."
	} else {
		if sig.Static == nil {
			code += string(sig.Namespace.Name) + "." + FuncsClass + "."
		} else {
			code += string(sig.Namespace.Name) + "." + compileType(sig.Static) + "."
		}
	}
	code += string(op.Name) + "("
	for i, arg := range argCode {
		if isMethod && i == 0 {
			continue
		}
		if i == len(argCode)-1 {
			code += arg
		} else {
			code += arg + ","
		}
	}
	code += ")"
	returnType = sig.Return

	return code, returnType, nil
}

func compileTypeCallForm(op TypeCallForm, ns *Namespace, expectedType Type,
	locals map[ShortName]Type) (code string, returnType Type, err error) {
	t := ns.GetType(op.Type)
	if t == nil {
		err = msg(op.Line, op.Column, "Invalid type call form: unknown type.")
		return
	}

	argCode := make([]string, len(op.Args))
	argTypes := make([]Type, len(op.Args))
	for i, expr := range op.Args {
		var err error
		argCode[i], argTypes[i], err = compileExpression(expr, ns, nil, locals)
		if err != nil {
			return "", nil, err
		}
	}
	if t == nil {
		// should be impossible
		return "", nil, msg(op.Line, op.Column, "Compiling call form starting with zero type.")
	} else if t == IntType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to I.")
			return
		}

	} else if t == LongType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to II.")
			return
		}

	} else if t == FloatType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to F.")
			return
		}

	} else if t == DoubleType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to FF.")
			return
		}

	} else if t == ByteType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to B.")
			return
		}

	} else if t == SignedByteType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to SB.")
			return
		}

	} else if t == BoolType {
		if len(op.Args) != 1 || !IsNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to Bool.")
			return
		}

	} else if t == StrType {
		if len(op.Args) != 1 {
			err = msg(op.Line, op.Column, "Invalid cast to Str.")
			return
		}
		// if number, convert to string
		// if class or struct type, call ToString
	} else if arrayType, ok := t.(ArrayType); ok {
		base, nDimensions := GetArrayType(arrayType)
		code = "new " + compileType(base)
		if op.SizeFlag && nDimensions != len(op.Args) {
			err = msg(op.Line, op.Column, "Wrong number of size arguments for array dimension.")
			return
		}
		for i := 0; i < nDimensions; i++ {
			if op.SizeFlag {
				if !IsInteger(argTypes[i]) {
					err = msg(op.Line, op.Column, "Non-integer size argument for array dimension.")
					return
				}
				code += "[" + argCode[i] + "]"
			} else {
				code += "[]"
			}
		}
		if !op.SizeFlag {
			code += "{"
			for i := 0; i < len(op.Args); i++ {
				if !IsSubType(argTypes[i], base) {
					err = msg(op.Line, op.Column, "Array value is wrong type.")
					return
				}
				code += argCode[i]
				if i < len(op.Args)-1 {
					code += ", "
				}
			}
			code += "}"
		}
		returnType = t
	} else {
		constructorSigs := ns.GetConstructors(op.Type.Name, op.Type.Namespace)
		if len(constructorSigs) > 0 {
			matching := []int{}
			// find sigs which match args
		Loop:
			for i, sig := range constructorSigs {
				if len(argTypes) == len(sig.ParamTypes) {
					for j, paramType := range sig.ParamTypes {
						if !IsSubType(argTypes[j], paramType) {
							continue Loop
						}
					}
					matching = append(matching, i)
				}
			}
			if len(matching) > 1 {
				return "", nil, msg(op.Line, op.Column, "Constructor call is ambiguous (multiple matching methods or functions).")
			} else if len(matching) == 1 {
				sig := constructorSigs[matching[0]]
				code += "new " + compileType(t) + "("
				for i, arg := range argCode {
					if i == len(argCode)-1 {
						code += arg
					} else {
						code += arg + ","
					}
				}
				code += ")"
				returnType = sig.Return
			}
		} else {
			return "", nil, msg(op.Line, op.Column, "Constructor call matches no known type.")
		}
	}
	return
}
