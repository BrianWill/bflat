package main

func isNumber(dt DataType) bool {
	return dt.Name == "I" || dt.Name == "F" || dt.Name == "B" || dt.Name == "SB" || dt.Name == "II" || dt.Name == "FF"
}

func isInteger(dt DataType) bool {
	return dt.Name == "I" || dt.Name == "B" || dt.Name == "II" || dt.Name == "SB"
}

func compileOperation(op CallForm, ns *Namespace, expectedType DataType,
	locals map[string]DataType) (string, DataType, error) {
	if op.Namespace != "" {
		return "", DataType{}, msg(op.Line, op.Column, "Call to unknown method or function.")
	}
	returnType := expectedType
	expectedArgType := expectedType
	multiOperand := true
	switch op.Name {
	case "add", "sub", "mul", "div":
		if !isNumber(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
	case "lt", "lte", "gt", "gte":
		if !isNumber(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
		expectedArgType = DoubleType // todo: actually need type which is supertype of all numbers (Long is not subtype of Double)
	case "inc", "dec":
		multiOperand = false
		if !isInteger(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
	case "mod", "band", "bor", "bxor":
		if !isInteger(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
	case "bnot":
		if !isInteger(expectedType) {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation used where non-number expected")
		}
		multiOperand = false
	case "eq", "neq":
		expectedArgType = DataType{}
		returnType = BoolType
	case "not":
		multiOperand = false
		expectedArgType = BoolType
	case "or", "and":
		expectedArgType = BoolType
	case "cat":
		expectedArgType = StrType
	default:
		return "", DataType{}, msg(op.Line, op.Column, "Unknown operator, function, or method.")
	}
	if multiOperand {
		if len(op.Args) < 2 {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation requires at least two operands")
		}
	} else {
		if len(op.Args) != 1 {
			return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation requires one operand")
		}
	}
	operandCode := make([]string, len(op.Args))
	operandTypes := make([]DataType, len(op.Args))
	for i, expr := range op.Args {
		var err error
		operandCode[i], operandTypes[i], err = compileExpression(expr, ns, expectedArgType, locals)
		if err != nil {
			return "", DataType{}, err
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
			if !isType(operandTypes[i+1], operandTypes[0], ns, true) {
				return "", DataType{}, msg(op.Line, op.Column, "'"+op.Name+"' operation has mismatched operand types")
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

func compileCallForm(op CallForm, ns *Namespace, expectedType DataType,
	locals map[string]DataType) (string, DataType, error) {
	fullName := fullName(op.Name, op.Namespace, ns)
	sigs := append(ns.Funcs[fullName], ns.Methods[fullName]...)
	if len(sigs) == 0 {
		return compileOperation(op, ns, expectedType, locals)
	}

	argCode := make([]string, len(op.Args))
	argTypes := make([]DataType, len(op.Args))
	for i, expr := range op.Args {
		var err error
		argCode[i], argTypes[i], err = compileExpression(expr, ns, DataType{}, locals)
		if err != nil {
			return "", DataType{}, err
		}
	}
	code := ""

	// find sigs which match args
	var returnType DataType

	matching := []*CallableInfo{}
Loop:
	for _, sig := range sigs {
		if len(argTypes) == len(sig.ParamTypes) {
			for j, paramType := range sig.ParamTypes {
				if !isType(argTypes[j], paramType, ns, false) {
					continue Loop
				}
			}
			matching = append(matching, sig)
		}
	}

	if len(matching) == 0 {
		return compileOperation(op, ns, expectedType, locals)
	}

	sig := matching[0]
	if len(matching) > 1 {
		var err error
		sig, err = ClosestMatchingSignature(matching, ns, op.Line, op.Column)
		if err != nil {
			return "", DataType{}, err
		}
	}

	isMethod := sig.IsMethod
	if isMethod {
		code += argCode[0] + "."
	} else {
		code += sig.Namespace + "." + FuncsClass + "."
	}
	code += op.Name + "("
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
	returnType = sig.ReturnType

	return code, returnType, nil
}

func compileTypeCallForm(op TypeCallForm, ns *Namespace, expectedType DataType,
	locals map[string]DataType) (code string, returnType DataType, err error) {
	argCode := make([]string, len(op.Args))
	argTypes := make([]DataType, len(op.Args))
	for i, expr := range op.Args {
		var err error
		argCode[i], argTypes[i], err = compileExpression(expr, ns, DataType{}, locals)
		if err != nil {
			return "", DataType{}, err
		}
	}
	if isZeroType(op.Type) {
		// should be impossible
		return "", DataType{}, msg(op.Line, op.Column, "Compiling call form starting with zero type.")
	} else if isType(op.Type, IntType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to I.")
			return
		}

	} else if isType(op.Type, LongIntType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to II.")
			return
		}

	} else if isType(op.Type, FloatType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to F.")
			return
		}

	} else if isType(op.Type, DoubleType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to FF.")
			return
		}

	} else if isType(op.Type, ByteType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to B.")
			return
		}

	} else if isType(op.Type, SignedByteType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to SB.")
			return
		}

	} else if isType(op.Type, BoolType, ns, true) {
		if len(op.Args) != 1 || !isNumber(argTypes[0]) {
			err = msg(op.Line, op.Column, "Invalid cast to Bool.")
			return
		}

	} else if isType(op.Type, StrType, ns, true) {
		if len(op.Args) != 1 {
			err = msg(op.Line, op.Column, "Invalid cast to Str.")
			return
		}
		// if number, convert to string
		// if class or struct type, call ToString
	} else {
		fullName := ns.FullNames[op.Type.Name]
		constructorSigs := ns.Constructors[fullName]
		if len(constructorSigs) > 0 {
			matching := []int{}
			// find sigs which match args
		Loop:
			for i, sig := range constructorSigs {
				if len(argTypes) == len(sig.ParamTypes) {
					for j, paramType := range sig.ParamTypes {
						if !isType(argTypes[j], paramType, ns, false) {
							continue Loop
						}
					}
					matching = append(matching, i)
				}
			}
			if len(matching) > 1 {
				return "", DataType{}, msg(op.Line, op.Column, "Constructor call is ambiguous (multiple matching methods or functions).")
			} else if len(matching) == 1 {
				sig := constructorSigs[matching[0]]
				code += "new " + op.Type.CSName(ns) + "("
				for i, arg := range argCode {
					if i == len(argCode)-1 {
						code += arg
					} else {
						code += arg + ","
					}
				}
				code += ")"
				returnType = sig.ReturnType
			}
		}
	}
	return
}
